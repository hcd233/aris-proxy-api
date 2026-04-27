# LLM Proxy Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 OpenAI/Anthropic 代理编排从 `forward*` 分支内联改造为 service/usecase 入口直接执行的两阶段类型化 pipeline。

**Architecture:** 新增轻量泛型 pipeline 基础设施，usecase 层为每类代理接口维护强类型 state，并通过 `ResolveEndpoint -> SelectRoute -> route-specific steps` 组合链路。第一轮实现优先保持现有 converter、transport、store、audit 行为兼容，streaming step 保留完整 SSE 控制流。

**Tech Stack:** Go 1.25.1、Huma stream response、Fiber status、bytedance/sonic、现有 llmproxy converter/transport/pool。

---

## 文件结构

- Create: `internal/application/llmproxy/pipeline/pipeline.go`
  泛型 `Step[T]`、`StepFunc[T]`、`Pipeline[T]`、`NewPipeline[T]`、`Execute`、`StepNames`。
- Create: `internal/application/llmproxy/usecase/pipeline_route.go`
  usecase 层路由描述：`pipelineRouteMode`、`pipelineRoute`、`selectPipelineRoute`。
- Create: `internal/application/llmproxy/usecase/openai_pipeline_state.go`
  OpenAI Chat/Responses 的 state、前置 step、builder、route-specific step 装配。
- Create: `internal/application/llmproxy/usecase/anthropic_pipeline_state.go`
  Anthropic Messages/CountTokens 的 state、前置 step、builder、route-specific step 装配。
- Modify: `internal/application/llmproxy/usecase/openai.go`
  `CreateChatCompletion` 和 `CreateResponse` 改成直接构建并执行 pipeline。
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
  `CreateMessage` 改成直接构建并执行 pipeline；`CountTokens` 暂保持 query 侧实现，除非任务 6 迁移后能保持旧行为。
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
  把 `forwardChat*` 主编排方法拆成可由 step 调用的 helper，或让 step 直接复用并在最终清理阶段删除主编排入口。
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
  把 `forwardResponse*` 主编排方法拆成可由 step 调用的 helper，或让 step 直接复用并在最终清理阶段删除主编排入口。
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
  把 `forwardMessage*` 主编排方法拆成可由 step 调用的 helper，或让 step 直接复用并在最终清理阶段删除主编排入口。
- Create: `test/unit/llmproxy_pipeline/pipeline_test.go`
  pipeline 执行器单元测试。
- Modify: `test/unit/llmproxy_usecase/openai_forward_test.go`
  加强 mock 断言，确认 service 入口仍覆盖 native/cross、stream/unary。
- Modify: `test/unit/llmproxy_usecase/anthropic_forward_test.go`
  加强 mock 断言，确认 service 入口仍覆盖 native/cross、stream/unary。
- Modify: `test/unit/llmproxy_usecase/llmproxy_usecase_test.go`
  CountTokens 迁移后保留旧行为测试。

执行过程中不要新增 `internal/**/_test.go`。本仓库测试文件必须放在 `test/unit/<topic>/` 或 `test/e2e/<topic>/`。

---

### Task 1: Pipeline 基础设施

**Files:**
- Create: `internal/application/llmproxy/pipeline/pipeline.go`
- Create: `test/unit/llmproxy_pipeline/pipeline_test.go`

- [ ] **Step 1: 写失败测试**

Create `test/unit/llmproxy_pipeline/pipeline_test.go`:

```go
package llmproxy_pipeline

import (
    "context"
    "errors"
    "reflect"
    "testing"

    "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/pipeline"
)

type testState struct {
    calls []string
}

func TestPipelineExecute_RunsStepsInOrder(t *testing.T) {
    state := &testState{}
    p := pipeline.NewPipeline(
        pipeline.NewStep("first", func(_ context.Context, s *testState) error {
            s.calls = append(s.calls, "first")
            return nil
        }),
        pipeline.NewStep("second", func(_ context.Context, s *testState) error {
            s.calls = append(s.calls, "second")
            return nil
        }),
    )

    if err := p.Execute(context.Background(), state); err != nil {
        t.Fatalf("Execute() error: %v", err)
    }

    want := []string{"first", "second"}
    if !reflect.DeepEqual(state.calls, want) {
        t.Fatalf("calls = %v, want %v", state.calls, want)
    }
}

func TestPipelineExecute_StopsOnError(t *testing.T) {
    state := &testState{}
    expectedErr := errors.New("stop")
    p := pipeline.NewPipeline(
        pipeline.NewStep("first", func(_ context.Context, s *testState) error {
            s.calls = append(s.calls, "first")
            return expectedErr
        }),
        pipeline.NewStep("second", func(_ context.Context, s *testState) error {
            s.calls = append(s.calls, "second")
            return nil
        }),
    )

    err := p.Execute(context.Background(), state)
    if !errors.Is(err, expectedErr) {
        t.Fatalf("Execute() error = %v, want %v", err, expectedErr)
    }

    want := []string{"first"}
    if !reflect.DeepEqual(state.calls, want) {
        t.Fatalf("calls = %v, want %v", state.calls, want)
    }
}

func TestPipelineStepNames(t *testing.T) {
    p := pipeline.NewPipeline(
        pipeline.NewStep("resolve_endpoint", func(_ context.Context, _ *testState) error { return nil }),
        pipeline.NewStep("select_route", func(_ context.Context, _ *testState) error { return nil }),
    )

    want := []string{"resolve_endpoint", "select_route"}
    if !reflect.DeepEqual(p.StepNames(), want) {
        t.Fatalf("StepNames() = %v, want %v", p.StepNames(), want)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/llmproxy_pipeline`

Expected: FAIL，报错包含 `no required module provides package github.com/hcd233/aris-proxy-api/internal/application/llmproxy/pipeline`。

- [ ] **Step 3: 实现 pipeline 基础设施**

Create `internal/application/llmproxy/pipeline/pipeline.go`:

```go
package pipeline

import "context"

type Step[T any] interface {
    Name() string
    Run(ctx context.Context, state *T) error
}

type StepFunc[T any] struct {
    name string
    fn   func(ctx context.Context, state *T) error
}

func NewStep[T any](name string, fn func(ctx context.Context, state *T) error) Step[T] {
    return &StepFunc[T]{name: name, fn: fn}
}

func (s *StepFunc[T]) Name() string {
    return s.name
}

func (s *StepFunc[T]) Run(ctx context.Context, state *T) error {
    return s.fn(ctx, state)
}

type Pipeline[T any] struct {
    steps []Step[T]
}

func NewPipeline[T any](steps ...Step[T]) *Pipeline[T] {
    return &Pipeline[T]{steps: steps}
}

func (p *Pipeline[T]) Execute(ctx context.Context, state *T) error {
    for _, step := range p.steps {
        if err := step.Run(ctx, state); err != nil {
            return err
        }
    }
    return nil
}

func (p *Pipeline[T]) StepNames() []string {
    names := make([]string, 0, len(p.steps))
    for _, step := range p.steps {
        names = append(names, step.Name())
    }
    return names
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/llmproxy_pipeline`

Expected: PASS。

- [ ] **Step 5: 检查工作区**

Run: `git status --short`

Expected: 只出现本任务新增的 pipeline 文件和测试文件。不要提交，除非用户明确要求提交。

---

### Task 2: 路由与 OpenAI Chat state/builder 骨架

**Files:**
- Create: `internal/application/llmproxy/usecase/pipeline_route.go`
- Create: `internal/application/llmproxy/usecase/openai_pipeline_state.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `test/unit/llmproxy_usecase/openai_forward_test.go`

- [ ] **Step 1: 加强 OpenAI Chat 转发测试中的 proxy 调用断言**

Modify `test/unit/llmproxy_usecase/openai_forward_test.go`，把 `mockOpenAIProxy` 改成记录调用：

```go
type mockOpenAIProxy struct {
    chatUnaryCalled      bool
    chatStreamCalled     bool
    responseUnaryCalled  bool
    responseStreamCalled bool
}

func (p *mockOpenAIProxy) ForwardChatCompletion(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.OpenAIChatCompletion, error) {
    p.chatUnaryCalled = true
    return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxy) ForwardChatCompletionStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
    p.chatStreamCalled = true
    return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxy) ForwardCreateResponse(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) ([]byte, error) {
    p.responseUnaryCalled = true
    return []byte(`{"status":"completed"}`), nil
}

func (p *mockOpenAIProxy) ForwardCreateResponseStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(string, []byte) error) error {
    p.responseStreamCalled = true
    return nil
}
```

把 `mockOpenAIAnthropicProxy` 改成记录调用：

```go
type mockOpenAIAnthropicProxy struct {
    messageUnaryCalled  bool
    messageStreamCalled bool
    countTokensCalled   bool
}

func (p *mockOpenAIAnthropicProxy) ForwardCreateMessage(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
    p.messageUnaryCalled = true
    return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockOpenAIAnthropicProxy) ForwardCreateMessageStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
    p.messageStreamCalled = true
    return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockOpenAIAnthropicProxy) ForwardCountTokens(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
    p.countTokensCalled = true
    return &dto.AnthropicTokensCount{InputTokens: 10}, nil
}
```

在四个 Chat 测试末尾追加断言：

```go
if !proxy.chatStreamCalled {
    t.Fatal("expected OpenAI stream proxy to be called")
}
```

```go
if !proxy.chatUnaryCalled {
    t.Fatal("expected OpenAI unary proxy to be called")
}
```

```go
if !anthropicProxy.messageStreamCalled {
    t.Fatal("expected Anthropic stream proxy to be called")
}
```

```go
if !anthropicProxy.messageUnaryCalled {
    t.Fatal("expected Anthropic unary proxy to be called")
}
```

- [ ] **Step 2: 运行测试确认现有实现仍通过**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'TestOpenAICreateChatCompletion'`

Expected: PASS。这个步骤锁定重构前行为。

- [ ] **Step 3: 新增路由类型**

Create `internal/application/llmproxy/usecase/pipeline_route.go`:

```go
package usecase

import "github.com/hcd233/aris-proxy-api/internal/enum"

type pipelineRouteMode string

const (
    pipelineRouteModeUnary  pipelineRouteMode = "unary"
    pipelineRouteModeStream pipelineRouteMode = "stream"
)

type pipelineRoute struct {
    SourceProvider enum.ProviderType
    TargetProvider enum.ProviderType
    Mode           pipelineRouteMode
}

func selectPipelineRoute(sourceProvider, targetProvider enum.ProviderType, stream bool) pipelineRoute {
    mode := pipelineRouteModeUnary
    if stream {
        mode = pipelineRouteModeStream
    }
    return pipelineRoute{SourceProvider: sourceProvider, TargetProvider: targetProvider, Mode: mode}
}
```

- [ ] **Step 4: 新增 OpenAI Chat state 与 builder 骨架**

Create `internal/application/llmproxy/usecase/openai_pipeline_state.go`:

```go
package usecase

import (
    "context"

    "github.com/danielgtaylor/huma/v2"
    "go.uber.org/zap"

    llmpipeline "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/pipeline"
    "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
    "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
    "github.com/hcd233/aris-proxy-api/internal/dto"
    "github.com/hcd233/aris-proxy-api/internal/enum"
    "github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
    "github.com/hcd233/aris-proxy-api/internal/logger"
    "github.com/hcd233/aris-proxy-api/internal/util"
)

type openAIChatPipelineState struct {
    Req          *dto.OpenAIChatCompletionRequest
    Log          *zap.Logger
    Endpoint     *aggregate.Endpoint
    Upstream     transport.UpstreamEndpoint
    Route        pipelineRoute
    Stream       bool
    HTTPResponse *huma.StreamResponse
}

func (u *openAIUseCase) buildOpenAIChatPipeline() *llmpipeline.Pipeline[openAIChatPipelineState] {
    return llmpipeline.NewPipeline(
        llmpipeline.NewStep("resolve_endpoint", u.resolveOpenAIChatEndpoint),
        llmpipeline.NewStep("select_route", u.selectOpenAIChatRoute),
        llmpipeline.NewStep("execute_route", u.executeOpenAIChatRoute),
    )
}

func (u *openAIUseCase) resolveOpenAIChatEndpoint(ctx context.Context, state *openAIChatPipelineState) error {
    ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(state.Req.Body.Model), enum.ProviderOpenAI, enum.ProviderAnthropic)
    if err != nil {
        state.Log.Error("[OpenAIUseCase] Model not found", zap.String("model", state.Req.Body.Model), zap.Error(err))
        state.HTTPResponse = util.SendOpenAIModelNotFoundError(state.Req.Body.Model)
        return nil
    }
    state.Endpoint = ep
    state.Upstream = toTransportEndpoint(ep)
    return nil
}

func (u *openAIUseCase) selectOpenAIChatRoute(_ context.Context, state *openAIChatPipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    state.Route = selectPipelineRoute(enum.ProviderOpenAI, state.Endpoint.Provider(), state.Stream)
    return nil
}

func (u *openAIUseCase) executeOpenAIChatRoute(ctx context.Context, state *openAIChatPipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    if state.Route.TargetProvider == enum.ProviderAnthropic {
        state.HTTPResponse = u.forwardChatViaAnthropic(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Stream)
        return nil
    }
    state.HTTPResponse = u.forwardChatNative(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Stream)
    return nil
}
```

- [ ] **Step 5: 修改 `CreateChatCompletion` 入口直接执行 pipeline**

Modify `internal/application/llmproxy/usecase/openai.go`，把 `CreateChatCompletion` 函数替换为：

```go
func (u *openAIUseCase) CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error) {
    state := &openAIChatPipelineState{
        Req:    req,
        Log:    logger.WithCtx(ctx),
        Stream: req.Body.Stream != nil && *req.Body.Stream,
    }
    if err := u.buildOpenAIChatPipeline().Execute(ctx, state); err != nil {
        return nil, err
    }
    return state.HTTPResponse, nil
}
```

移除 `openai.go` 中不再使用的 imports：`go.uber.org/zap`、`vo`。保留 `logger`，因为新入口使用它。

- [ ] **Step 6: 运行 OpenAI Chat 聚焦测试**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'TestOpenAICreateChatCompletion'`

Expected: PASS。

- [ ] **Step 7: 检查工作区**

Run: `git status --short`

Expected: 显示 Task 1 和 Task 2 相关文件。不要提交，除非用户明确要求提交。

---

### Task 3: OpenAI Chat route-specific steps 替换 `execute_route` 临时分支

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai_pipeline_state.go`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`

- [ ] **Step 1: 在 builder 中追加两阶段 route-specific steps**

Modify `buildOpenAIChatPipeline` 为：

```go
func (u *openAIUseCase) buildOpenAIChatPipeline() *llmpipeline.Pipeline[openAIChatPipelineState] {
    return llmpipeline.NewPipeline(
        llmpipeline.NewStep("resolve_endpoint", u.resolveOpenAIChatEndpoint),
        llmpipeline.NewStep("select_route", u.selectOpenAIChatRoute),
        llmpipeline.NewStep("prepare_openai_chat_route", u.prepareOpenAIChatRoute),
        llmpipeline.NewStep("forward_openai_chat_route", u.forwardOpenAIChatRoute),
    )
}
```

Extend `openAIChatPipelineState`:

```go
type openAIChatPipelineState struct {
    Req          *dto.OpenAIChatCompletionRequest
    Log          *zap.Logger
    Endpoint     *aggregate.Endpoint
    Upstream     transport.UpstreamEndpoint
    Route        pipelineRoute
    Stream       bool
    HTTPResponse *huma.StreamResponse
    Body         []byte
}
```

- [ ] **Step 2: 实现 prepare step**

Add to `openai_pipeline_state.go`:

```go
func (u *openAIUseCase) prepareOpenAIChatRoute(_ context.Context, state *openAIChatPipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    if state.Route.TargetProvider == enum.ProviderAnthropic {
        rsp, body := u.prepareChatViaAnthropic(state.Log, state.Req, state.Upstream)
        state.HTTPResponse = rsp
        state.Body = body
        return nil
    }
    state.Body = prepareChatNativeBody(state.Req, state.Upstream)
    return nil
}
```

- [ ] **Step 3: 从 `openai_chat.go` 抽出 body helper**

Add to `internal/application/llmproxy/usecase/openai_chat.go` near existing chat functions:

```go
func prepareChatNativeBody(req *dto.OpenAIChatCompletionRequest, upstream transport.UpstreamEndpoint) []byte {
    body := transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)
    return util.EnsureAssistantMessageReasoningContent(body)
}

func (u *openAIUseCase) prepareChatViaAnthropic(log *zap.Logger, req *dto.OpenAIChatCompletionRequest, upstream transport.UpstreamEndpoint) (*huma.StreamResponse, []byte) {
    conv := converter.AnthropicProtocolConverter{}
    anthropicReq, err := conv.FromOpenAIRequest(req.Body)
    if err != nil {
        log.Error("[OpenAIUseCase] Failed to convert request to Anthropic format", zap.Error(err))
        return util.SendOpenAIInternalError(), nil
    }
    anthropicReq.Model = upstream.Model
    return nil, lo.Must1(sonic.Marshal(anthropicReq))
}
```

Modify `forwardChatNative` body preparation to call the helper:

```go
body := prepareChatNativeBody(req, upstream)
```

Modify `forwardChatViaAnthropic` body preparation to call helper:

```go
rsp, body := u.prepareChatViaAnthropic(log, req, upstream)
if rsp != nil {
    return rsp
}
```

- [ ] **Step 4: 实现 forward step**

Add to `openai_pipeline_state.go`:

```go
func (u *openAIUseCase) forwardOpenAIChatRoute(ctx context.Context, state *openAIChatPipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    if state.Route.TargetProvider == enum.ProviderAnthropic {
        conv := converter.AnthropicProtocolConverter{}
        if state.Stream {
            state.HTTPResponse = u.forwardChatViaAnthropicStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
            return nil
        }
        state.HTTPResponse = u.forwardChatViaAnthropicUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
        return nil
    }
    if state.Stream {
        state.HTTPResponse = u.forwardChatNativeStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
        return nil
    }
    state.HTTPResponse = u.forwardChatNativeUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
    return nil
}
```

Add missing import in `openai_pipeline_state.go`:

```go
"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
```

Remove `executeOpenAIChatRoute` from `openai_pipeline_state.go`.

- [ ] **Step 5: 运行 OpenAI Chat 聚焦测试**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'TestOpenAICreateChatCompletion'`

Expected: PASS。

- [ ] **Step 6: 检查编译**

Run: `go test -count=1 ./internal/application/llmproxy/...`

Expected: PASS 或 `[no test files]`，不能有编译错误。

---

### Task 4: 迁移 OpenAI Responses 到 service pipeline

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai_pipeline_state.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `test/unit/llmproxy_usecase/openai_forward_test.go`

- [ ] **Step 1: 为 Response 测试追加 proxy 调用断言**

在 `TestOpenAICreateResponse_NativeStream` 末尾追加：

```go
if !proxy.responseStreamCalled {
    t.Fatal("expected OpenAI response stream proxy to be called")
}
```

在 `TestOpenAICreateResponse_NativeUnary` 末尾追加：

```go
if !proxy.responseUnaryCalled {
    t.Fatal("expected OpenAI response unary proxy to be called")
}
```

在 `TestOpenAICreateResponse_ViaAnthropicStream` 末尾追加：

```go
if !anthropicProxy.messageStreamCalled {
    t.Fatal("expected Anthropic stream proxy to be called")
}
```

在 `TestOpenAICreateResponse_ViaAnthropicUnary` 末尾追加：

```go
if !anthropicProxy.messageUnaryCalled {
    t.Fatal("expected Anthropic unary proxy to be called")
}
```

- [ ] **Step 2: 运行 Response 测试确认现有行为**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'TestOpenAICreateResponse'`

Expected: PASS。

- [ ] **Step 3: 新增 Response state 与 builder**

Add to `openai_pipeline_state.go`:

```go
type openAIResponsePipelineState struct {
    Req          *dto.OpenAICreateResponseRequest
    Log          *zap.Logger
    Endpoint     *aggregate.Endpoint
    Upstream     transport.UpstreamEndpoint
    Route        pipelineRoute
    Stream       bool
    HTTPResponse *huma.StreamResponse
    Body         []byte
}

func (u *openAIUseCase) buildOpenAIResponsePipeline() *llmpipeline.Pipeline[openAIResponsePipelineState] {
    return llmpipeline.NewPipeline(
        llmpipeline.NewStep("resolve_endpoint", u.resolveOpenAIResponseEndpoint),
        llmpipeline.NewStep("select_route", u.selectOpenAIResponseRoute),
        llmpipeline.NewStep("prepare_openai_response_route", u.prepareOpenAIResponseRoute),
        llmpipeline.NewStep("forward_openai_response_route", u.forwardOpenAIResponseRoute),
    )
}

func (u *openAIUseCase) resolveOpenAIResponseEndpoint(ctx context.Context, state *openAIResponsePipelineState) error {
    ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(state.Req.Body.Model), enum.ProviderOpenAI, enum.ProviderAnthropic)
    if err != nil {
        state.Log.Error("[OpenAIUseCase] Response API model not found", zap.String("model", state.Req.Body.Model), zap.Error(err))
        state.HTTPResponse = util.SendOpenAIModelNotFoundError(state.Req.Body.Model)
        return nil
    }
    state.Endpoint = ep
    state.Upstream = toTransportEndpoint(ep)
    return nil
}

func (u *openAIUseCase) selectOpenAIResponseRoute(_ context.Context, state *openAIResponsePipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    state.Route = selectPipelineRoute(enum.ProviderOpenAI, state.Endpoint.Provider(), state.Stream)
    return nil
}
```

- [ ] **Step 4: 抽出 Response body helper**

Add to `openai_response.go`:

```go
func prepareResponseNativeBody(req *dto.OpenAICreateResponseRequest, upstream transport.UpstreamEndpoint) []byte {
    return transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)
}

func (u *openAIUseCase) prepareResponseViaAnthropic(log *zap.Logger, req *dto.OpenAICreateResponseRequest, upstream transport.UpstreamEndpoint) (*huma.StreamResponse, []byte) {
    conv := converter.AnthropicProtocolConverter{}
    anthropicReq, err := conv.FromResponseAPIRequest(req.Body)
    if err != nil {
        log.Error("[OpenAIUseCase] Failed to convert request to Anthropic format", zap.Error(err))
        return util.SendOpenAIInternalError(), nil
    }
    anthropicReq.Model = upstream.Model
    return nil, lo.Must1(sonic.Marshal(anthropicReq))
}
```

Modify existing `forwardResponseNative` to use `prepareResponseNativeBody`.

Modify existing `forwardResponseViaAnthropic` to use `prepareResponseViaAnthropic` and return early when rsp is not nil.

- [ ] **Step 5: 实现 Response prepare/forward steps**

Add to `openai_pipeline_state.go`:

```go
func (u *openAIUseCase) prepareOpenAIResponseRoute(_ context.Context, state *openAIResponsePipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    if state.Route.TargetProvider == enum.ProviderAnthropic {
        rsp, body := u.prepareResponseViaAnthropic(state.Log, state.Req, state.Upstream)
        state.HTTPResponse = rsp
        state.Body = body
        return nil
    }
    state.Body = prepareResponseNativeBody(state.Req, state.Upstream)
    return nil
}

func (u *openAIUseCase) forwardOpenAIResponseRoute(ctx context.Context, state *openAIResponsePipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    if state.Route.TargetProvider == enum.ProviderAnthropic {
        conv := converter.AnthropicProtocolConverter{}
        if state.Stream {
            state.HTTPResponse = u.forwardResponseViaAnthropicStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
            return nil
        }
        state.HTTPResponse = u.forwardResponseViaAnthropicUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
        return nil
    }
    if state.Stream {
        state.HTTPResponse = u.forwardResponseNativeStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
        return nil
    }
    state.HTTPResponse = u.forwardResponseNativeUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
    return nil
}
```

- [ ] **Step 6: 修改 `CreateResponse` 入口执行 pipeline**

Modify `openai.go`:

```go
func (u *openAIUseCase) CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error) {
    state := &openAIResponsePipelineState{
        Req:    req,
        Log:    logger.WithCtx(ctx),
        Stream: req.Body.Stream != nil && *req.Body.Stream,
    }
    if err := u.buildOpenAIResponsePipeline().Execute(ctx, state); err != nil {
        return nil, err
    }
    return state.HTTPResponse, nil
}
```

- [ ] **Step 7: 运行 OpenAI usecase 测试**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'TestOpenAICreate'`

Expected: PASS。

---

### Task 5: 迁移 Anthropic Messages 到 service pipeline

**Files:**
- Create: `internal/application/llmproxy/usecase/anthropic_pipeline_state.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Modify: `test/unit/llmproxy_usecase/anthropic_forward_test.go`

- [ ] **Step 1: 为 Anthropic proxy mock 加调用断言字段**

Modify `test/unit/llmproxy_usecase/anthropic_forward_test.go`:

```go
type mockAnthropicProxyForAnthropic struct {
    messageUnaryCalled  bool
    messageStreamCalled bool
    countTokensCalled   bool
}

func (p *mockAnthropicProxyForAnthropic) ForwardCreateMessage(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.AnthropicMessage, error) {
    p.messageUnaryCalled = true
    return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockAnthropicProxyForAnthropic) ForwardCreateMessageStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
    p.messageStreamCalled = true
    return &dto.AnthropicMessage{ID: "test"}, nil
}

func (p *mockAnthropicProxyForAnthropic) ForwardCountTokens(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.AnthropicTokensCount, error) {
    p.countTokensCalled = true
    return &dto.AnthropicTokensCount{InputTokens: 10}, nil
}
```

Modify OpenAI mock:

```go
type mockOpenAIProxyForAnthropic struct {
    chatUnaryCalled      bool
    chatStreamCalled     bool
    responseUnaryCalled  bool
    responseStreamCalled bool
}

func (p *mockOpenAIProxyForAnthropic) ForwardChatCompletion(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.OpenAIChatCompletion, error) {
    p.chatUnaryCalled = true
    return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxyForAnthropic) ForwardChatCompletionStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
    p.chatStreamCalled = true
    return &dto.OpenAIChatCompletion{ID: "test"}, nil
}

func (p *mockOpenAIProxyForAnthropic) ForwardCreateResponse(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) ([]byte, error) {
    p.responseUnaryCalled = true
    return nil, nil
}

func (p *mockOpenAIProxyForAnthropic) ForwardCreateResponseStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(string, []byte) error) error {
    p.responseStreamCalled = true
    return nil
}
```

Add assertions to native/cross stream/unary tests mirroring Task 2.

- [ ] **Step 2: 运行 Anthropic Messages 测试确认现有行为**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'TestAnthropicCreateMessage'`

Expected: PASS。

- [ ] **Step 3: 新增 Anthropic Message state 与 builder**

Create `internal/application/llmproxy/usecase/anthropic_pipeline_state.go`:

```go
package usecase

import (
    "context"

    "github.com/danielgtaylor/huma/v2"
    "go.uber.org/zap"

    "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
    llmpipeline "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/pipeline"
    "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
    "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
    "github.com/hcd233/aris-proxy-api/internal/dto"
    "github.com/hcd233/aris-proxy-api/internal/enum"
    "github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
    "github.com/hcd233/aris-proxy-api/internal/util"
)

type anthropicMessagePipelineState struct {
    Req          *dto.AnthropicCreateMessageRequest
    Log          *zap.Logger
    Endpoint     *aggregate.Endpoint
    Upstream     transport.UpstreamEndpoint
    Route        pipelineRoute
    Stream       bool
    ExposedModel string
    HTTPResponse *huma.StreamResponse
    Body         []byte
}

func (u *anthropicUseCase) buildAnthropicMessagePipeline() *llmpipeline.Pipeline[anthropicMessagePipelineState] {
    return llmpipeline.NewPipeline(
        llmpipeline.NewStep("resolve_endpoint", u.resolveAnthropicMessageEndpoint),
        llmpipeline.NewStep("select_route", u.selectAnthropicMessageRoute),
        llmpipeline.NewStep("prepare_anthropic_message_route", u.prepareAnthropicMessageRoute),
        llmpipeline.NewStep("forward_anthropic_message_route", u.forwardAnthropicMessageRoute),
    )
}

func (u *anthropicUseCase) resolveAnthropicMessageEndpoint(ctx context.Context, state *anthropicMessagePipelineState) error {
    ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(state.Req.Body.Model), enum.ProviderAnthropic, enum.ProviderOpenAI)
    if err != nil {
        state.Log.Error("[AnthropicUseCase] Model not found", zap.String("model", state.Req.Body.Model), zap.Error(err))
        state.HTTPResponse = util.SendAnthropicModelNotFoundError(state.Req.Body.Model)
        return nil
    }
    state.Endpoint = ep
    state.Upstream = toTransportEndpoint(ep)
    return nil
}

func (u *anthropicUseCase) selectAnthropicMessageRoute(_ context.Context, state *anthropicMessagePipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    state.Route = selectPipelineRoute(enum.ProviderAnthropic, state.Endpoint.Provider(), state.Stream)
    return nil
}
```

- [ ] **Step 4: 抽出 Anthropic body helper**

Add to `anthropic.go` near forwarding functions:

```go
func prepareMessageNativeBody(req *dto.AnthropicCreateMessageRequest, upstream transport.UpstreamEndpoint) []byte {
    return transport.ReplaceModelInBody(lo.Must1(sonic.Marshal(req.Body)), upstream.Model)
}

func (u *anthropicUseCase) prepareMessageViaOpenAI(log *zap.Logger, req *dto.AnthropicCreateMessageRequest, upstream transport.UpstreamEndpoint) (*huma.StreamResponse, []byte) {
    conv := converter.OpenAIProtocolConverter{}
    openAIReq, err := conv.FromAnthropicRequest(req.Body)
    if err != nil {
        log.Error("[AnthropicUseCase] Failed to convert request to OpenAI format", zap.Error(err))
        return util.SendAnthropicInternalError(), nil
    }
    openAIReq.Model = upstream.Model
    body := lo.Must1(sonic.Marshal(openAIReq))
    return nil, util.EnsureAssistantMessageReasoningContent(body)
}
```

Modify `forwardMessageNative` to use `prepareMessageNativeBody`.

Modify `forwardMessageViaOpenAI` to use `prepareMessageViaOpenAI` and return early when rsp is not nil.

- [ ] **Step 5: 实现 prepare/forward steps**

Add to `anthropic_pipeline_state.go`:

```go
func (u *anthropicUseCase) prepareAnthropicMessageRoute(_ context.Context, state *anthropicMessagePipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    if state.Route.TargetProvider == enum.ProviderOpenAI {
        rsp, body := u.prepareMessageViaOpenAI(state.Log, state.Req, state.Upstream)
        state.HTTPResponse = rsp
        state.Body = body
        return nil
    }
    state.Body = prepareMessageNativeBody(state.Req, state.Upstream)
    return nil
}

func (u *anthropicUseCase) forwardAnthropicMessageRoute(ctx context.Context, state *anthropicMessagePipelineState) error {
    if state.HTTPResponse != nil {
        return nil
    }
    if state.Route.TargetProvider == enum.ProviderOpenAI {
        conv := converter.OpenAIProtocolConverter{}
        if state.Stream {
            state.HTTPResponse = u.forwardMessageViaOpenAIStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body, &conv)
            return nil
        }
        state.HTTPResponse = u.forwardMessageViaOpenAIUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body, &conv)
        return nil
    }
    if state.Stream {
        state.HTTPResponse = u.forwardMessageNativeStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body)
        return nil
    }
    state.HTTPResponse = u.forwardMessageNativeUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body)
    return nil
}
```

- [ ] **Step 6: 修改 `CreateMessage` 入口执行 pipeline**

Modify `anthropic.go`:

```go
func (u *anthropicUseCase) CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error) {
    state := &anthropicMessagePipelineState{
        Req:          req,
        Log:          logger.WithCtx(ctx),
        Stream:       req.Body.Stream != nil && *req.Body.Stream,
        ExposedModel: req.Body.Model,
    }
    if err := u.buildAnthropicMessagePipeline().Execute(ctx, state); err != nil {
        return nil, err
    }
    return state.HTTPResponse, nil
}
```

- [ ] **Step 7: 运行 Anthropic Messages 测试**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'TestAnthropicCreateMessage'`

Expected: PASS。

---

### Task 6: CountTokens 保持旧语义并纳入 pipeline 入口

**Files:**
- Modify: `internal/application/llmproxy/usecase/anthropic_pipeline_state.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Modify: `test/unit/llmproxy_usecase/llmproxy_usecase_test.go`

- [ ] **Step 1: 增加 CountTokens usecase 入口测试**

Append to `test/unit/llmproxy_usecase/llmproxy_usecase_test.go`:

```go
func TestAnthropicUseCaseCountTokens_DelegatesToQuery(t *testing.T) {
    query := &mockAnthropicCountTokensUseCase{result: &dto.AnthropicTokensCount{InputTokens: 12}}
    uc := usecase.NewAnthropicUseCase(&mockResolver{}, &mockAnthropicListModelsForUseCase{}, query, &mockOpenAIProxyForCountTokens{}, &mockAnthropicProxy{})

    rsp, err := uc.CountTokens(context.Background(), &dto.AnthropicCountTokensRequest{Body: &dto.AnthropicCountTokensReq{Model: "claude-alias"}})
    if err != nil {
        t.Fatalf("CountTokens() error: %v", err)
    }
    if rsp.InputTokens != 12 {
        t.Fatalf("InputTokens = %d, want 12", rsp.InputTokens)
    }
    if !query.called {
        t.Fatal("expected CountTokens query to be called")
    }
}

type mockAnthropicCountTokensUseCase struct {
    called bool
    result *dto.AnthropicTokensCount
}

func (m *mockAnthropicCountTokensUseCase) Handle(_ context.Context, _ *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
    m.called = true
    return m.result, nil
}

type mockAnthropicListModelsForUseCase struct{}

func (m *mockAnthropicListModelsForUseCase) Handle(_ context.Context) (*dto.AnthropicListModelsRsp, error) {
    return &dto.AnthropicListModelsRsp{}, nil
}

type mockOpenAIProxyForCountTokens struct{}

func (p *mockOpenAIProxyForCountTokens) ForwardChatCompletion(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) (*dto.OpenAIChatCompletion, error) {
    return nil, nil
}

func (p *mockOpenAIProxyForCountTokens) ForwardChatCompletionStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
    return nil, nil
}

func (p *mockOpenAIProxyForCountTokens) ForwardCreateResponse(_ context.Context, _ transport.UpstreamEndpoint, _ []byte) ([]byte, error) {
    return nil, nil
}

func (p *mockOpenAIProxyForCountTokens) ForwardCreateResponseStream(_ context.Context, _ transport.UpstreamEndpoint, _ []byte, _ func(string, []byte) error) error {
    return nil
}
```

- [ ] **Step 2: 运行 CountTokens 测试**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'Test.*CountTokens'`

Expected: PASS。这个步骤确认 CountTokens 暂不改变行为。

- [ ] **Step 3: 记录 CountTokens 暂不迁移底层 query 的原因**

Modify `anthropic.go` CountTokens 注释：

```go
// CountTokens 调用上游 count_tokens（错误时返回空结果，与旧行为一致）。
// 该路径当前由 query 侧封装读取仓储和 Anthropic 上游调用；usecase 入口保留委托，
// 避免为不支持 OpenAI fallback 的 token count 引入不准确估算。
func (u *anthropicUseCase) CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error) {
    return u.countTokensQuery.Handle(ctx, req)
}
```

- [ ] **Step 4: 运行 CountTokens 测试确认通过**

Run: `go test -count=1 ./test/unit/llmproxy_usecase -run 'Test.*CountTokens'`

Expected: PASS。

---

### Task 7: 清理 `forward*` 主编排入口与 imports

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`

- [ ] **Step 1: 删除 OpenAI Chat 主编排函数**

Delete these functions from `openai_chat.go` only after Task 3 tests pass:

```go
func (u *openAIUseCase) forwardChatNative(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse
func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, log *zap.Logger, req *dto.OpenAIChatCompletionRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse
```

Keep these route-specific helpers because pipeline steps call them:

```go
func (u *openAIUseCase) forwardChatNativeStream(...)
func (u *openAIUseCase) forwardChatNativeUnary(...)
func (u *openAIUseCase) forwardChatViaAnthropicStream(...)
func (u *openAIUseCase) forwardChatViaAnthropicUnary(...)
```

- [ ] **Step 2: 删除 OpenAI Response 主编排函数**

Delete these functions from `openai_response.go` only after Task 4 tests pass:

```go
func (u *openAIUseCase) forwardResponseNative(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse
func (u *openAIUseCase) forwardResponseViaAnthropic(ctx context.Context, log *zap.Logger, req *dto.OpenAICreateResponseRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, stream bool) *huma.StreamResponse
```

Keep these route-specific helpers:

```go
func (u *openAIUseCase) forwardResponseNativeStream(...)
func (u *openAIUseCase) forwardResponseNativeUnary(...)
func (u *openAIUseCase) forwardResponseViaAnthropicStream(...)
func (u *openAIUseCase) forwardResponseViaAnthropicUnary(...)
```

- [ ] **Step 3: 删除 Anthropic Messages 主编排函数**

Delete these functions from `anthropic.go` only after Task 5 tests pass:

```go
func (u *anthropicUseCase) forwardMessageNative(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse
func (u *anthropicUseCase) forwardMessageViaOpenAI(ctx context.Context, log *zap.Logger, req *dto.AnthropicCreateMessageRequest, ep *aggregate.Endpoint, upstream transport.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse
```

Keep these route-specific helpers:

```go
func (u *anthropicUseCase) forwardMessageNativeStream(...)
func (u *anthropicUseCase) forwardMessageNativeUnary(...)
func (u *anthropicUseCase) forwardMessageViaOpenAIStream(...)
func (u *anthropicUseCase) forwardMessageViaOpenAIUnary(...)
```

- [ ] **Step 4: 清理 imports 并格式化**

Run: `gofmt -w internal/application/llmproxy/pipeline/pipeline.go internal/application/llmproxy/usecase/pipeline_route.go internal/application/llmproxy/usecase/openai_pipeline_state.go internal/application/llmproxy/usecase/anthropic_pipeline_state.go internal/application/llmproxy/usecase/openai.go internal/application/llmproxy/usecase/openai_chat.go internal/application/llmproxy/usecase/openai_response.go internal/application/llmproxy/usecase/anthropic.go test/unit/llmproxy_pipeline/pipeline_test.go test/unit/llmproxy_usecase/openai_forward_test.go test/unit/llmproxy_usecase/anthropic_forward_test.go test/unit/llmproxy_usecase/llmproxy_usecase_test.go`

Expected: command exits 0。

- [ ] **Step 5: 运行 usecase 测试**

Run: `go test -count=1 ./test/unit/llmproxy_usecase ./test/unit/llmproxy_pipeline`

Expected: PASS。

---

### Task 8: 验证与规范扫描

**Files:**
- No planned code edits.

- [ ] **Step 1: 运行 llmproxy 聚焦测试**

Run: `go test -count=1 ./test/unit/llmproxy_usecase ./test/unit/llmproxy_pipeline ./test/unit/converter ./test/unit/openai_response_dto ./test/unit/openai_stream_tool_call ./test/unit/anthropic_sse`

Expected: PASS。

- [ ] **Step 2: 运行应用包编译测试**

Run: `go test -count=1 ./internal/application/llmproxy/...`

Expected: PASS 或 `[no test files]`，不能有编译错误。

- [ ] **Step 3: 运行自定义规范扫描**

Run: `make lint-conv`

Expected: PASS。

- [ ] **Step 4: 运行全量测试**

Run: `go test -count=1 ./...`

Expected: PASS。E2E 默认因缺少 `BASE_URL` 和 `API_KEY` skip，不应访问生产。

- [ ] **Step 5: 检查最终 diff**

Run: `git status --short`

Expected: 只包含本计划相关文件变更。

Run: `git diff --stat`

Expected: 变更集中在 pipeline、新 usecase state/builder、usecase 入口、相关单元测试。

---

## 自检记录

- Spec 覆盖：计划覆盖 pipeline 基础设施、service/usecase 入口执行、两阶段 route、OpenAI Chat、OpenAI Responses、Anthropic Messages、CountTokens 旧语义说明、streaming 边界保留、测试与验证。
- 占位扫描：计划中没有 `TBD`、`TODO`、`implement later` 之类占位项。
- 类型一致性：`pipelineRoute`、`pipelineRouteMode`、`openAIChatPipelineState`、`openAIResponsePipelineState`、`anthropicMessagePipelineState` 名称在各任务中保持一致。
- 仓库规则：测试文件均位于 `test/unit/...`；未要求执行 git commit，符合当前会话没有明确提交请求的约束。
