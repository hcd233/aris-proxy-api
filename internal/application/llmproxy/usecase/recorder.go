package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// streamTimer 收敛流式转发路径中重复的「首 token 延迟 / 流持续时长」计时逻辑。
//
// 所有流式 forward 方法此前都各自手写 firstTokenTime/firstTokenLatencyMs/streamDurationMs
// 三件套；streamTimer 把这段编排收进一个可复用单元，调用方只需 markFirstToken + finish。
type streamTimer struct {
	start          time.Time
	firstToken     time.Time
	firstLatencyMs int64
	durationMs     int64
}

// newStreamTimer 创建并启动计时器（以当前时刻为流起点）。
func newStreamTimer() *streamTimer {
	return &streamTimer{start: time.Now()}
}

// markFirstToken 在首个有效增量到达时记录首 token 延迟；重复调用幂等。
func (t *streamTimer) markFirstToken() {
	if t.firstToken.IsZero() {
		t.firstToken = time.Now()
		t.firstLatencyMs = t.firstToken.Sub(t.start).Milliseconds()
	}
}

// finish 流结束时结算首 token 之后的流持续时长。
func (t *streamTimer) finish() {
	if !t.firstToken.IsZero() {
		t.durationMs = time.Since(t.firstToken).Milliseconds()
	}
}

// tokenUsage 抽象不同上游协议的 token 用量来源。
//
// 一次模型调用收尾时需要做两件事：把 token 计数写进审计任务、把 input+output 总量
// 上报给限流器。三种协议（OpenAI Chat / Anthropic Message / Response API）各有自己的
// Usage 结构与提取规则，tokenUsage 把这种差异收敛到一个小接口背后。
type tokenUsage interface {
	apply(task *dto.ModelCallAuditTask)
	reportable() int64
}

type openAITokenUsage struct{ usage *dto.OpenAICompletionUsage }

func (u openAITokenUsage) apply(task *dto.ModelCallAuditTask) { task.SetTokensFromOpenAIUsage(u.usage) }

func (u openAITokenUsage) reportable() int64 {
	if u.usage == nil {
		return 0
	}
	return u.usage.InputOutputTokens()
}

type anthropicTokenUsage struct{ msg *dto.AnthropicMessage }

func (u anthropicTokenUsage) apply(task *dto.ModelCallAuditTask) {
	task.SetTokensFromAnthropicUsage(u.msg)
}

func (u anthropicTokenUsage) reportable() int64 {
	if u.msg == nil || u.msg.Usage == nil {
		return 0
	}
	return u.msg.Usage.InputOutputTokens()
}

type responseTokenUsage struct{ rsp *dto.OpenAICreateResponseRsp }

func (u responseTokenUsage) apply(task *dto.ModelCallAuditTask) {
	task.SetTokensFromResponseUsage(u.rsp)
}

func (u responseTokenUsage) reportable() int64 {
	if u.rsp == nil || u.rsp.Usage == nil {
		return 0
	}
	return u.rsp.Usage.InputOutputTokens()
}

// callOutcome 描述一次模型调用收尾所需的全部信息——审计任务组装、token 上报、
// 上游状态/错误归一化的统一入参。
//
// successStatus=true 表示上游 HTTP 200 的非流式成功路径（状态码硬编码 200）；
// 否则由 ExtractUpstreamStatusAndError 从 err 推导状态码与错误信息。
// responseStatus 仅在 Response API native 路径下设置，用于注入 HTTP 200 但
// status=failed/incomplete 的 in-band 失败原因。
type callOutcome struct {
	model               *aggregate.Model
	exposedModel        string
	endpoint            string
	upstreamProtocol    enum.ProtocolType
	apiProtocol         enum.ProtocolType
	firstTokenLatencyMs int64
	streamDurationMs    int64
	usage               tokenUsage
	successStatus       bool
	err                 error
	responseStatus      *dto.OpenAICreateResponseRsp
}

// recordModelCall 收敛所有 forward 路径重复的审计尾部：组装 ModelCallAuditTask、
// 写入 token 计数、上报限流用量、归一化上游状态/错误，最后投递异步审计任务。
//
// 这是模型调用收尾的唯一 seam——审计与 token 上报的 bug 只可能出现在这一处，
// 也只需在这一处测试。
func recordModelCall(ctx context.Context, submitter TaskSubmitter, out callOutcome) {
	task := &dto.ModelCallAuditTask{
		Ctx:                 util.CopyContextValues(ctx),
		ModelID:             out.model.AggregateID(),
		Model:               out.exposedModel,
		Endpoint:            out.endpoint,
		UpstreamProtocol:    out.upstreamProtocol,
		APIProtocol:         out.apiProtocol,
		FirstTokenLatencyMs: out.firstTokenLatencyMs,
		StreamDurationMs:    out.streamDurationMs,
	}

	if out.usage != nil {
		out.usage.apply(task)
		reportTokenUsage(ctx, out.usage.reportable())
	}

	if out.successStatus {
		task.UpstreamStatusCode = fiber.StatusOK
	} else {
		task.UpstreamStatusCode, task.ErrorMessage = apiutil.ExtractUpstreamStatusAndError(out.err)
	}
	if out.responseStatus != nil {
		task.SetErrorFromResponseStatus(out.responseStatus)
	}

	_ = submitter.SubmitModelCallAuditTask(task) //nolint:errcheck // best-effort audit submission
}
