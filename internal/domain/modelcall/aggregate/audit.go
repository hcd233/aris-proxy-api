// Package aggregate ModelCall 域聚合根
package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// ModelCallAudit 模型调用审计聚合根
//
// 封装一次模型调用的完整审计信息。聚合构造后不可变，写入一次即固化；
// Complete/Fail 行为的差异只体现在 CallStatus 与事件类型。
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type ModelCallAudit struct {
	aggregate.Base

	apiKeyID         uint
	modelID          uint
	model            string            // exposed model（客户端请求别名）
	upstreamProvider enum.ProviderType // ep.Provider
	apiProvider      enum.ProviderType // 入站协议
	tokens           vo.TokenBreakdown
	latency          vo.CallLatency
	status           vo.CallStatus
	userAgent        string
	traceID          string
	createdAt        time.Time
}

// RecordCall 构造一条审计聚合（成功/失败由 CallInput.Status 区分，工厂不重复表达）
//
// RecordCompletedCall / RecordFailedCall 曾作为两条独立工厂存在，但二者行为完全等价
// （差异仅在于入参 RecordCallInput.Status 本身），因此在职能上合并为 RecordCall。
//
//	@param input RecordCallInput
//	@return *ModelCallAudit
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func RecordCall(input RecordCallInput) *ModelCallAudit {
	return newAudit(input, time.Now().UTC())
}

// RecordCallInput 构造审计聚合的输入参数
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type RecordCallInput struct {
	APIKeyID         uint
	ModelID          uint
	Model            string
	UpstreamProvider enum.ProviderType
	APIProvider      enum.ProviderType
	Tokens           vo.TokenBreakdown
	Latency          vo.CallLatency
	Status           vo.CallStatus
	UserAgent        string
	TraceID          string
}

// newAudit 构造聚合但不生成事件（由调用方选择 Complete/Fail 事件）
func newAudit(input RecordCallInput, now time.Time) *ModelCallAudit {
	return &ModelCallAudit{
		apiKeyID:         input.APIKeyID,
		modelID:          input.ModelID,
		model:            input.Model,
		upstreamProvider: input.UpstreamProvider,
		apiProvider:      input.APIProvider,
		tokens:           input.Tokens,
		latency:          input.Latency,
		status:           input.Status,
		userAgent:        input.UserAgent,
		traceID:          input.TraceID,
		createdAt:        now,
	}
}

// AggregateType 实现 aggregate.Root 接口
func (*ModelCallAudit) AggregateType() string { return constant.AggregateTypeModelCallAudit }

// APIKeyID 返回 API Key ID
func (a *ModelCallAudit) APIKeyID() uint { return a.apiKeyID }

// ModelID 返回模型端点 ID
func (a *ModelCallAudit) ModelID() uint { return a.modelID }

// Model 返回 exposed model 名
func (a *ModelCallAudit) Model() string { return a.model }

// UpstreamProvider 返回上游协议
func (a *ModelCallAudit) UpstreamProvider() enum.ProviderType { return a.upstreamProvider }

// APIProvider 返回入站协议
func (a *ModelCallAudit) APIProvider() enum.ProviderType { return a.apiProvider }

// Tokens 返回 token 统计
func (a *ModelCallAudit) Tokens() vo.TokenBreakdown { return a.tokens }

// Latency 返回延迟
func (a *ModelCallAudit) Latency() vo.CallLatency { return a.latency }

// Status 返回调用状态
func (a *ModelCallAudit) Status() vo.CallStatus { return a.status }

// UserAgent 返回 User-Agent
func (a *ModelCallAudit) UserAgent() string { return a.userAgent }

// TraceID 返回 Trace ID
func (a *ModelCallAudit) TraceID() string { return a.traceID }

// CreatedAt 返回创建时间
func (a *ModelCallAudit) CreatedAt() time.Time { return a.createdAt }
