# OpenAI Decision API 配置与代理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让网关支持 OpenAI Decision API（`POST /api/openai/v1/decisions`）的端点配置与转发，并产生与既有三个接口同口径的审计、计费、会话存储与触发词拦截行为。

**Architecture:** 沿用既有 "Endpoint 能力开关 + `SelectCompatRoute` + `transport` + `usecase` + `JSONResult` 透传" 三层结构：Endpoint 新增 `support_openai_decision` 开关；Decision 仅走 native 路线（不做跨协议转换）；请求体 typed DTO 序列化后替换 `model` 为上游模型名，响应体 raw 透传、仅解析 `model`/`answers`/`usage` 用于别名回写、会话存储与审计。

**Tech Stack:** Go 1.25.1 / fiber v3 + huma v2 / sonic / GORM + PostgreSQL / `samber/lo` + `samber/mo` / Redis（限流）/ Next.js + TypeScript（管理端）

**Spec:** `docs/superpowers/specs/2026-10-09-openai-decision-api-design.md`

## Global Constraints

- 工作目录：所有命令在 worktree `.worktrees/openai-decision-api`（分支 `feature/openai-decision-api-2026-10-09`）根目录执行。
- 序列化统一 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`。
- DTO 与生产代码禁止 `any` / `interface{}`：用 `sonic.NoCopyRawMessage` 或具体结构体。
- 业务错误统一 `internal/common/ierr`；禁止 `errors.New` / `fmt.Errorf`。
- 业务包禁止定义本地 `const` 块；常量放 `internal/common/constant/` 或 `internal/common/enum/`。
- HTTP 状态码用 `fiber.StatusXxx`；日志用 `logger.WithCtx(ctx)`，消息前缀 `[PascalCaseModule]`。
- 测试只放 `test/unit/<topic>/` 或 `test/e2e/<topic>/`；只用标准库 `testing`（禁止 testify / gomock / gomock）。
- 上下文：handler/usecase/transport 必须从调用方接收 `context.Context`，禁止自建 `context.Background()`。
- Go 文件编辑前先跑（禁止截断输出）：
  `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <file.go>`
- 前端：`cd web && npm run lint`、`npx tsc --noEmit -p tsconfig.json`、`npm run test`（vitest）；中文文案同步 zh/en/ja 三份 locale。
- 提交信息用既有约定（`feat(scope): ...` / `test(scope): ...` / `docs(scope): ...`），中文描述。

---

### Task 1: Endpoint `support_openai_decision` 能力开关（后端全链路）

**Files:**
- Modify: `internal/infrastructure/database/model/endpoint.go`
- Modify: `internal/common/constant/string.go`（`FieldEndpointSupportOpenAIDecision`）
- Modify: `internal/common/constant/sql.go`（`FieldSupportOpenAIDecision` + `EndpointRepoFieldsFull`）
- Modify: `internal/domain/llmproxy/aggregate/endpoint.go`
- Modify: `internal/infrastructure/repository/endpoint_repository.go`
- Modify: `internal/application/endpoint/port/handler.go`
- Modify: `internal/application/endpoint/command/create_endpoint.go`
- Modify: `internal/application/endpoint/command/update_endpoint.go`
- Modify: `internal/application/upstream/port/handler.go`
- Modify: `internal/application/upstream/query/list_upstream.go`
- Modify: `internal/dto/endpoint.go`
- Modify: `internal/dto/upstream.go`
- Modify: `internal/handler/endpoint.go`
- Modify: `internal/handler/upstream.go`
- Test: `test/unit/domain_llmproxy/endpoint_test.go`、`test/unit/endpoint_resolver/endpoint_resolver_test.go`、`test/unit/llmproxy_usecase/openai_forward_test.go`、`test/unit/llmproxy_usecase/anthropic_forward_test.go`、`test/unit/llmproxy_repo_scope/endpoint_repository_scope_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
  - `aggregate.CreateEndpoint(id uint, name, openaiBaseURL, anthropicBaseURL, apiKey string, supportChatCompletion, supportResponse, supportMessage, supportDecision bool) (*Endpoint, error)`
  - `(*aggregate.Endpoint).SupportOpenAIDecision() bool`
  - `(*aggregate.Endpoint).Update(name, openaiBaseURL, anthropicBaseURL, apiKey *string, supportChatCompletion, supportResponse, supportMessage, supportDecision *bool)`
  - `port.CreateEndpointCommand.SupportOpenAIDecision bool`、`port.UpdateEndpointCommand.SupportOpenAIDecision *bool`
  - `dto.CreateEndpointReqBody.SupportOpenAIDecision *bool`、`dto.UpdateEndpointReqBody.SupportOpenAIDecision *bool`、`dto.UpstreamEndpointItem.SupportOpenAIDecision bool`
  - `upstreamport.UpstreamEndpointView.SupportOpenAIDecision bool`

- [ ] **Step 1: 写失败测试（能力校验）**

编辑 `test/unit/domain_llmproxy/endpoint_test.go`：两个表驱动结构体各加一行字段 `supportOpenAIDecision bool`，`CreateEndpoint` 调用加第 9 个实参，并补 decision 用例。

```go
// TestCreateEndpoint_AllowsSingleProtocolEndpoint 的 cases 增加：
		{
			name:                  "openai_only_decision",
			openaiBaseURL:         "https://api.openai.com",
			supportOpenAIDecision: true,
		},
```

```go
// TestCreateEndpoint_RejectsMissingSupportedProtocolBaseURL 的 cases 增加：
		{
			name:                  "missing_openai_decision_base_url",
			supportOpenAIDecision: true,
		},
```

两处调用改为：

```go
			ep, err := aggregate.CreateEndpoint(
				1,
				tc.name,
				tc.openaiBaseURL,
				tc.anthropicBaseURL,
				"sk-test",
				tc.supportOpenAIChatCompletion,
				tc.supportOpenAIResponse,
				tc.supportAnthropicMessage,
				tc.supportOpenAIDecision,
			)
```

`TestCreateEndpoint_RejectsBothEmptyBaseURL` 与 `TestCreateEndpoint_RejectsNoCapability` 的实参改为：

```go
	_, err := aggregate.CreateEndpoint(
		1, "test", "", "", "sk-test",
		false, false, false, false,
	)
```

```go
	_, err := aggregate.CreateEndpoint(
		1, "test", "https://api.openai.com", "https://api.anthropic.com", "sk-test",
		false, false, false, false,
	)
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 -run TestCreateEndpoint ./test/unit/domain_llmproxy/`
Expected: 编译失败 —— `too many arguments in call to aggregate.CreateEndpoint`

- [ ] **Step 3: 领域聚合根改造**

`internal/domain/llmproxy/aggregate/endpoint.go`：结构体加字段（`supportAnthropicMessage` 之后）：

```go
	supportOpenAIDecision       bool
```

`CreateEndpoint` 签名与实现：

```go
// CreateEndpoint 构造 Endpoint 聚合根
func CreateEndpoint(
	id uint,
	name, openaiBaseURL, anthropicBaseURL, apiKey string,
	supportChatCompletion, supportResponse, supportMessage, supportDecision bool,
) (*Endpoint, error) {
	if name == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint name cannot be empty")
	}
	if apiKey == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint apiKey cannot be empty")
	}
	if openaiBaseURL == "" && anthropicBaseURL == "" {
		return nil, ierr.New(ierr.ErrValidation, "at least one base URL must be provided")
	}
	if !supportChatCompletion && !supportResponse && !supportMessage && !supportDecision {
		return nil, ierr.New(ierr.ErrValidation, "at least one capability must be enabled")
	}
	if (supportChatCompletion || supportResponse || supportDecision) && openaiBaseURL == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint openai baseURL cannot be empty when OpenAI APIs are supported")
	}
	if supportMessage && anthropicBaseURL == "" {
		return nil, ierr.New(ierr.ErrValidation, "endpoint anthropic baseURL cannot be empty when Anthropic messages API is supported")
	}
	ep := &Endpoint{
		name:                        name,
		openaiBaseURL:               openaiBaseURL,
		anthropicBaseURL:            anthropicBaseURL,
		apiKey:                      apiKey,
		supportOpenAIChatCompletion: supportChatCompletion,
		supportOpenAIResponse:       supportResponse,
		supportAnthropicMessage:     supportMessage,
		supportOpenAIDecision:       supportDecision,
	}
	ep.SetID(id)
	return ep, nil
}
```

getter（`SupportAnthropicMessage` 之后）：

```go
func (e *Endpoint) SupportOpenAIDecision() bool { return e.supportOpenAIDecision }
```

`Update` 签名与新增分支：

```go
func (e *Endpoint) Update(name, openaiBaseURL, anthropicBaseURL, apiKey *string, supportChatCompletion, supportResponse, supportMessage, supportDecision *bool) {
```

```go
	if supportMessage != nil {
		e.supportAnthropicMessage = *supportMessage
	}
	if supportDecision != nil {
		e.supportOpenAIDecision = *supportDecision
	}
```

- [ ] **Step 4: 常量与仓储**

`internal/common/constant/string.go`（`FieldEndpointSupportAnthropicMessage` 之后）：

```go
	FieldEndpointSupportOpenAIDecision      = "support_openai_decision"
```

`internal/common/constant/sql.go`：

```go
	FieldSupportOpenAIDecision       = "support_openai_decision"
```

`EndpointRepoFieldsFull` 改为：

```go
	EndpointRepoFieldsFull = []string{FieldID, FieldUserID, FieldName, FieldOpenaiBaseURL, FieldAnthropicBaseURL, FieldAPIKey,
		FieldSupportOpenAIChatCompletion, FieldSupportOpenAIResponse, FieldSupportAnthropicMessage, FieldSupportOpenAIDecision,
		FieldCreatedAt, FieldUpdatedAt}
```

`internal/infrastructure/database/model/endpoint.go`（`SupportAnthropicMessage` 之后）：

```go
	SupportOpenAIDecision       bool   `json:"support_openai_decision" gorm:"column:support_openai_decision;not null;default:false;comment:支持/decisions"`
```

`internal/infrastructure/repository/endpoint_repository.go`：

`toEndpointAggregate` 传参追加：

```go
		m.SupportAnthropicMessage,
		m.SupportOpenAIDecision,
	)
```

`toEndpointModel` 追加：

```go
		SupportAnthropicMessage:     ep.SupportAnthropicMessage(),
		SupportOpenAIDecision:       ep.SupportOpenAIDecision(),
```

`Update` 的 updates map 追加：

```go
		constant.FieldEndpointSupportAnthropicMessage:     ep.SupportAnthropicMessage(),
		constant.FieldEndpointSupportOpenAIDecision:       ep.SupportOpenAIDecision(),
```

- [ ] **Step 5: 应用层命令与 DTO 透传**

`internal/application/endpoint/port/handler.go`：`CreateEndpointCommand` 加 `SupportOpenAIDecision bool`；`UpdateEndpointCommand` 加 `SupportOpenAIDecision *bool`（均紧跟 `SupportAnthropicMessage` 字段）。

`internal/application/endpoint/command/create_endpoint.go`：

```go
	ep, err := aggregate.CreateEndpoint(0, cmd.Name, cmd.OpenaiBaseURL, cmd.AnthropicBaseURL, cmd.APIKey, cmd.SupportOpenAIChatCompletion, cmd.SupportOpenAIResponse, cmd.SupportAnthropicMessage, cmd.SupportOpenAIDecision)
```

`internal/application/endpoint/command/update_endpoint.go`：

```go
	ep.Update(cmd.Name, cmd.OpenaiBaseURL, cmd.AnthropicBaseURL, cmd.APIKey, cmd.SupportOpenAIChatCompletion, cmd.SupportOpenAIResponse, cmd.SupportAnthropicMessage, cmd.SupportOpenAIDecision)
```

`internal/dto/endpoint.go`：`CreateEndpointReqBody` 与 `UpdateEndpointReqBody` 各加（`SupportAnthropicMessage` 之后）：

```go
	SupportOpenAIDecision       *bool   `json:"supportOpenAIDecision,omitempty" doc:"是否支持 OpenAI Decision"`
```

`internal/dto/upstream.go`：`UpstreamEndpointItem` 加：

```go
	SupportOpenAIDecision       bool              `json:"supportOpenAIDecision" doc:"是否支持 OpenAI Decision"`
```

`internal/application/upstream/port/handler.go`：`UpstreamEndpointView` 加 `SupportOpenAIDecision bool`。

`internal/application/upstream/query/list_upstream.go`：`toEndpointView` 返回值加：

```go
		SupportAnthropicMessage:     ep.SupportAnthropicMessage(),
		SupportOpenAIDecision:       ep.SupportOpenAIDecision(),
```

`internal/handler/endpoint.go`：创建命令与更新命令两处映射各加：

```go
		SupportOpenAIDecision:       lo.FromPtr(req.Body.SupportOpenAIDecision),
```

```go
		SupportOpenAIDecision:       req.Body.SupportOpenAIDecision,
```

`internal/handler/upstream.go`：`toUpstreamEndpointItem` 加：

```go
		SupportOpenAIDecision:       v.SupportOpenAIDecision,
```

- [ ] **Step 6: 同步既有测试调用点**

各文件在 `aggregate.CreateEndpoint(...)` 实参末尾追加一个 `false`（Decision 无关用例）：

- `test/unit/endpoint_resolver/endpoint_resolver_test.go:125`、`:258`、`:285`、`:286`
- `test/unit/llmproxy_repo_scope/endpoint_repository_scope_test.go:63`
- `test/unit/llmproxy_usecase/anthropic_forward_test.go:83`
- `test/unit/llmproxy_usecase/openai_forward_test.go` 的 `buildCompatEndpoint`：`aggregate.CreateEndpoint(1, name, openaiBaseURL, anthropicBaseURL, "test-api-key", supportChat, supportResponse, supportMessage, false)`

- [ ] **Step 7: 运行聚焦测试**

Run: `go test -count=1 ./test/unit/domain_llmproxy/ ./test/unit/endpoint_resolver/ ./test/unit/endpoint_command/ ./test/unit/llmproxy_usecase/ ./test/unit/llmproxy_repo_scope/`
Expected: 全部 PASS

- [ ] **Step 8: 全量构建与 lint**

Run: `go build ./... && make lint-conv`
Expected: 无输出 / `ok`

- [ ] **Step 9: Commit**

```bash
git add internal/common internal/domain internal/infrastructure internal/application/endpoint internal/application/upstream internal/dto internal/handler test/unit
git commit -m "feat(endpoint): 新增 support_openai_decision 能力开关"
```

---

### Task 2: 前端 Endpoint 能力配置 UI

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/app/(dashboard)/upstream/shared.tsx`
- Modify: `web/src/app/(dashboard)/upstream/endpoint-dialog.tsx`
- Modify: `web/src/app/(dashboard)/upstream/grouped-view.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`
- Modify: `web/src/locales/zh.json`、`web/src/locales/en.json`、`web/src/locales/ja.json`
- Test: `web/src/app/(dashboard)/upstream/__tests__/endpoint-form-defaults.test.ts`（新建）

**Interfaces:**
- Consumes: Task 1 的 `supportOpenAIDecision` 字段（`CreateEndpointReqBody` / `UpdateEndpointReqBody` / `UpstreamEndpointItem`）
- Produces: 前端表单/列表对 `supportOpenAIDecision` 的完整读写

- [ ] **Step 1: 写失败测试**

新建 `web/src/app/(dashboard)/upstream/__tests__/endpoint-form-defaults.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import { emptyEndpointForm } from "../shared";

describe("emptyEndpointForm", () => {
  it("默认关闭 OpenAI Decision 能力", () => {
    expect(emptyEndpointForm.supportOpenAIDecision).toBe(false);
  });

  it("保留既有三项能力默认值", () => {
    expect(emptyEndpointForm.supportOpenAIChatCompletion).toBe(true);
    expect(emptyEndpointForm.supportOpenAIResponse).toBe(false);
    expect(emptyEndpointForm.supportAnthropicMessage).toBe(false);
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && npm run test -- endpoint-form-defaults`
Expected: FAIL —— `emptyEndpointForm.supportOpenAIDecision` 为 `undefined`

- [ ] **Step 3: types.ts**

`web/src/lib/types.ts`：`CreateEndpointReqBody` / `UpdateEndpointReqBody` 各加 `supportOpenAIDecision?: boolean;`；`UpstreamEndpointItem` 加 `supportOpenAIDecision: boolean;`（均紧跟 `supportAnthropicMessage`）。

- [ ] **Step 4: shared.tsx**

`web/src/app/(dashboard)/upstream/shared.tsx`：`EndpointForm` 加 `supportOpenAIDecision: boolean;`；`emptyEndpointForm` 加 `supportOpenAIDecision: false,`。

- [ ] **Step 5: endpoint-dialog.tsx 复选框**

在 `supportAnthropicMessage` 复选框之后追加：

```tsx
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.supportOpenAIDecision}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, supportOpenAIDecision: e.target.checked }))
                  }
                  className="rounded"
                />
                {t("endpoints.openai_decision_label")}
              </label>
```

- [ ] **Step 6: grouped-view.tsx 协议徽标**

在 `supportAnthropicMessage` 徽标之后追加：

```tsx
                        {ep.supportOpenAIDecision && (
                          <ProtocolBadge
                            protocol="openai-decision"
                            label={t("endpoints.openai_decision_label")}
                          />
                        )}
```

`ProviderIcon` 无需改动：`openai-decision` 命中 `openai` 前缀分支。

- [ ] **Step 7: 审计页协议标签**

`web/src/app/(dashboard)/audit/model/page.tsx` 的 `formatProtocol` 标签表补一行：

```ts
    "anthropic-message": "Messages",
    "openai-decision": "Decision",
```

- [ ] **Step 8: page.tsx 三处映射**

`openEditEndpoint` 的表单预填、`handleSaveEndpoint` 的 update 与 create 请求体，各在 `supportAnthropicMessage` 之后追加：

```ts
      supportOpenAIDecision: ep.supportOpenAIDecision,
```

```ts
          supportOpenAIDecision: endpointForm.supportOpenAIDecision,
```

（update / create 两处均加。）

- [ ] **Step 9: locales**

三个文件在 `endpoints.anthropic_messages_label` 之后追加：

```json
  "endpoints.openai_decision_label": "OpenAI Decision API",
```

（zh 用 `"OpenAI Decision API"`；en 同；ja 同。）

- [ ] **Step 10: 运行前端校验**

Run: `cd web && npm run test -- endpoint-form-defaults && npx tsc --noEmit -p tsconfig.json && npm run lint`
Expected: 测试 PASS、tsc 无错误、lint 无错误

- [ ] **Step 11: Commit**

```bash
git add web/src/lib/types.ts "web/src/app/(dashboard)/upstream" "web/src/app/(dashboard)/audit/model/page.tsx" web/src/locales
git commit -m "feat(web): 上游端点支持配置 OpenAI Decision 能力"
```

---

### Task 3: Decision 请求/响应 DTO

**Files:**
- Create: `internal/dto/openai/decision.go`
- Modify: `internal/dto/aliases.go`
- Test: `test/unit/openai_decision_dto/decision_dto_test.go`（新建）

**Interfaces:**
- Consumes: 既有 `openai.ResponseInputMessageContent` / `openai.ResponseInputContent`（`input` 的文本+图片 parts 复用）；`constant.NullJSONLiteral`；`enum.JSONSchemaTypeString` / `enum.JSONSchemaTypeBoolean`
- Produces:
  - `dto.OpenAICreateDecisionRequest{Body *OpenAICreateDecisionReq}`
  - `dto.DecisionInput{Text *string; Messages []*DecisionInputMessage}`
  - `dto.DecisionQuestion{Type string; Name *string; Instructions string; Choices []*DecisionChoice; Levels []*DecisionScoreLevel}`
  - `dto.DecisionChoiceValue{StringValue *string; BooleanValue *bool}`
  - `dto.OpenAIDecisionRsp{Model string; Answers sonic.NoCopyRawMessage; Usage *OpenAIDecisionUsage}`
  - `dto.OpenAIDecisionUsage.InputOutputTokens() int64`

- [ ] **Step 1: 写失败测试**

新建 `test/unit/openai_decision_dto/decision_dto_test.go`：

```go
package openai_decision_dto

import (
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestDecisionInput_StringOrMessages(t *testing.T) {
	t.Parallel()

	var textInput dto.DecisionInput
	if err := sonic.UnmarshalString(`"hello"`, &textInput); err != nil {
		t.Fatalf("unmarshal string input: %v", err)
	}
	if textInput.Text == nil || *textInput.Text != "hello" {
		t.Fatalf("text input = %+v, want hello", textInput)
	}

	var msgInput dto.DecisionInput
	raw := `[{"role":"user","content":[{"type":"input_text","text":"a"},{"type":"input_image","image_url":"https://x/y.png","detail":"high"}]}]`
	if err := sonic.UnmarshalString(raw, &msgInput); err != nil {
		t.Fatalf("unmarshal messages input: %v", err)
	}
	if len(msgInput.Messages) != 1 || len(msgInput.Messages[0].Content.Parts) != 2 {
		t.Fatalf("messages input = %+v, want 1 message with 2 parts", msgInput)
	}
	if got := msgInput.Messages[0].Content.Parts[1].ImageURL; got == nil || *got != "https://x/y.png" {
		t.Fatalf("image url = %v, want https://x/y.png", got)
	}
}

func TestDecisionInput_MarshalRoundTrip(t *testing.T) {
	t.Parallel()

	text := "hi"
	out, err := sonic.Marshal(dto.DecisionInput{Text: &text})
	if err != nil {
		t.Fatalf("marshal text input: %v", err)
	}
	if string(out) != `"hi"` {
		t.Fatalf("marshal text input = %s, want \"hi\"", out)
	}

	nilOut, err := sonic.Marshal(dto.DecisionInput{})
	if err != nil {
		t.Fatalf("marshal empty input: %v", err)
	}
	if string(nilOut) != "null" {
		t.Fatalf("marshal empty input = %s, want null", nilOut)
	}
}

func TestDecisionChoiceValue_BoolIsNotString(t *testing.T) {
	t.Parallel()

	var boolValue dto.DecisionChoiceValue
	if err := sonic.UnmarshalString("true", &boolValue); err != nil {
		t.Fatalf("unmarshal bool: %v", err)
	}
	if boolValue.BooleanValue == nil || !*boolValue.BooleanValue {
		t.Fatalf("bool value = %+v, want true", boolValue)
	}
	if boolValue.StringValue != nil {
		t.Fatal("bool value must not populate StringValue")
	}

	var strValue dto.DecisionChoiceValue
	if err := sonic.UnmarshalString(`"true"`, &strValue); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if strValue.StringValue == nil || *strValue.StringValue != "true" {
		t.Fatalf("string value = %+v, want \"true\"", strValue)
	}
	if strValue.BooleanValue != nil {
		t.Fatal("string value must not populate BooleanValue")
	}

	out, err := sonic.Marshal(dto.DecisionChoiceValue{BooleanValue: boolValue.BooleanValue})
	if err != nil {
		t.Fatalf("marshal bool: %v", err)
	}
	if string(out) != "true" {
		t.Fatalf("marshal bool = %s, want true", out)
	}
}

func TestDecisionQuestion_ThreeTypes(t *testing.T) {
	t.Parallel()

	raw := `[
		{"type":"predicate","instructions":"is it damaged?","name":"damaged"},
		{"type":"choice","instructions":"pick","choices":[{"value":"a","description":"A"},{"value":true}]},
		{"type":"score","instructions":"rate","levels":[{"label":"low"},{"label":"high","description":"very"}]}
	]`
	var questions []*dto.DecisionQuestion
	if err := sonic.UnmarshalString(raw, &questions); err != nil {
		t.Fatalf("unmarshal questions: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("questions = %d, want 3", len(questions))
	}
	if len(questions[1].Choices) != 2 || questions[1].Choices[1].Value.BooleanValue == nil {
		t.Fatalf("choice bool not parsed: %+v", questions[1].Choices)
	}
	if len(questions[2].Levels) != 2 || questions[2].Levels[1].Description == nil {
		t.Fatalf("score levels not parsed: %+v", questions[2].Levels)
	}
}

func TestOpenAIDecisionRsp_UsageDetails(t *testing.T) {
	t.Parallel()

	raw := `{"model":"gpt-x","answers":[{"type":"predicate","name":"d","probability":0.9}],
		"usage":{"input_tokens":42,"output_tokens":7,"total_tokens":49,
		"input_tokens_details":{"cached_tokens":12,"cache_write_tokens":5},
		"output_tokens_details":{"reasoning_tokens":3}}}`
	var rsp dto.OpenAIDecisionRsp
	if err := sonic.UnmarshalString(raw, &rsp); err != nil {
		t.Fatalf("unmarshal rsp: %v", err)
	}
	if rsp.Usage == nil || rsp.Usage.InputTokensDetails == nil {
		t.Fatalf("usage = %+v, want details", rsp.Usage)
	}
	if rsp.Usage.InputTokensDetails.CachedTokens != 12 || rsp.Usage.InputTokensDetails.CacheWriteTokens != 5 {
		t.Fatalf("input details = %+v", rsp.Usage.InputTokensDetails)
	}
	if rsp.Usage.OutputTokensDetails == nil || rsp.Usage.OutputTokensDetails.ReasoningTokens != 3 {
		t.Fatalf("output details = %+v", rsp.Usage.OutputTokensDetails)
	}
	if got := rsp.Usage.InputOutputTokens(); got != 49 {
		t.Fatalf("InputOutputTokens() = %d, want 49", got)
	}
	if string(rsp.Answers) == "" {
		t.Fatal("answers must be preserved raw")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/openai_decision_dto/`
Expected: 编译失败 —— `undefined: dto.DecisionInput`

- [ ] **Step 3: 实现 DTO**

新建 `internal/dto/openai/decision.go`：

```go
package openai

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// ==================== Decision API (/decisions) ====================
//
// 参考 docs/openai/create_decisions.md。
// 请求完整建模；响应只建模审计/存储所需子集（model/answers/usage），
// 响应体本身由 usecase 原样透传（与 /responses unary 同范式）。

// OpenAICreateDecisionRequest 创建 Decision 请求
type OpenAICreateDecisionRequest struct {
	Body *OpenAICreateDecisionReq `json:"body" doc:"请求体"`
}

// OpenAICreateDecisionReq 创建 Decision 请求体
type OpenAICreateDecisionReq struct {
	Model            string              `json:"model" required:"true" doc:"模型别名"`
	Input            DecisionInput       `json:"input" required:"true" doc:"待评估的文本或 user 消息数组"`
	Questions        []*DecisionQuestion `json:"questions" required:"true" minItems:"1" doc:"问题列表"`
	SafetyIdentifier *string             `json:"safety_identifier,omitempty" doc:"调用方提供的终端用户标识"`
}

// DecisionInput input 联合类型：string | []DecisionInputMessage
type DecisionInput struct {
	Text     *string                 `json:"-"`
	Messages []*DecisionInputMessage `json:"-"`
}

// UnmarshalJSON 先按字符串解析，失败则按消息数组解析
func (d *DecisionInput) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		d.Text = &s
		return nil
	}
	return sonic.Unmarshal(data, &d.Messages)
}

// MarshalJSON 按已填充的分支输出
func (d DecisionInput) MarshalJSON() ([]byte, error) {
	if d.Text != nil {
		return sonic.Marshal(*d.Text)
	}
	if d.Messages != nil {
		return sonic.Marshal(d.Messages)
	}
	return []byte(constant.NullJSONLiteral), nil
}

// Schema 声明为 string 或 DecisionInputMessage 数组
func (DecisionInput) Schema(r huma.Registry) *huma.Schema {
	msgSchema := r.Schema(reflect.TypeFor[DecisionInputMessage](), true, "DecisionInputMessage")
	return &huma.Schema{OneOf: []*huma.Schema{
		{Type: enum.JSONSchemaTypeString},
		{Type: enum.JSONSchemaTypeArray, Items: msgSchema},
	}}
}

// DecisionInputMessage 待评估的 user 消息（仅支持 role=user）
type DecisionInputMessage struct {
	Role    string                      `json:"role" doc:"固定 user"`
	Type    *string                     `json:"type,omitempty" doc:"固定 message"`
	Content ResponseInputMessageContent `json:"content" doc:"文本或文本+图片 parts"`
}

// DecisionQuestion 决策问题（predicate / choice / score）
type DecisionQuestion struct {
	Type         string                `json:"type" doc:"predicate/choice/score"`
	Name         *string               `json:"name,omitempty" doc:"问题名(可选)"`
	Instructions string                `json:"instructions" required:"true" doc:"问题指令"`
	Choices      []*DecisionChoice     `json:"choices,omitempty" doc:"type=choice 时的 2~255 个选项"`
	Levels       []*DecisionScoreLevel `json:"levels,omitempty" doc:"type=score 时的有序等级"`
}

// DecisionChoice choice 型问题的选项
type DecisionChoice struct {
	Value       DecisionChoiceValue `json:"value" doc:"选项值(string|bool)"`
	Description *string             `json:"description,omitempty" doc:"选项描述"`
}

// DecisionChoiceValue choice.value 联合类型：string | bool
// 同文本的 string 与 bool 是不同选项，故必须区分。
type DecisionChoiceValue struct {
	StringValue  *string `json:"-"`
	BooleanValue *bool   `json:"-"`
}

// UnmarshalJSON 先按 bool 解析（否则 "true" 会被当成字符串），失败再按 string
func (v *DecisionChoiceValue) UnmarshalJSON(data []byte) error {
	var b bool
	if err := sonic.Unmarshal(data, &b); err == nil {
		v.BooleanValue = &b
		return nil
	}
	var s string
	if err := sonic.Unmarshal(data, &s); err != nil {
		return err
	}
	v.StringValue = &s
	return nil
}

// MarshalJSON 按已填充的分支输出
func (v DecisionChoiceValue) MarshalJSON() ([]byte, error) {
	if v.BooleanValue != nil {
		return sonic.Marshal(*v.BooleanValue)
	}
	if v.StringValue != nil {
		return sonic.Marshal(*v.StringValue)
	}
	return []byte(constant.NullJSONLiteral), nil
}

// Schema 声明为 boolean 或 string
func (DecisionChoiceValue) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{OneOf: []*huma.Schema{
		{Type: enum.JSONSchemaTypeBoolean},
		{Type: enum.JSONSchemaTypeString},
	}}
}

// DecisionScoreLevel score 型问题的有序等级
type DecisionScoreLevel struct {
	Label       string  `json:"label" doc:"等级标签"`
	Description *string `json:"description,omitempty" doc:"等级描述"`
}

// ==================== Decision 响应（审计/存储子集） ====================

// OpenAIDecisionRsp Decision 响应子集
type OpenAIDecisionRsp struct {
	Model   string                 `json:"model" doc:"上游返回的模型名"`
	Answers sonic.NoCopyRawMessage `json:"answers" doc:"答案数组(原样保留)"`
	Usage   *OpenAIDecisionUsage   `json:"usage,omitempty" doc:"使用量"`
}

// OpenAIDecisionUsage Decision 使用量
type OpenAIDecisionUsage struct {
	InputTokens         int                          `json:"input_tokens" doc:"输入 token"`
	OutputTokens        int                          `json:"output_tokens" doc:"输出 token"`
	TotalTokens         int                          `json:"total_tokens" doc:"总 token"`
	InputTokensDetails  *DecisionInputTokensDetails  `json:"input_tokens_details,omitempty" doc:"输入明细"`
	OutputTokensDetails *DecisionOutputTokensDetails `json:"output_tokens_details,omitempty" doc:"输出明细"`
}

// DecisionInputTokensDetails 输入明细（两维均为 input_tokens 的子集）
type DecisionInputTokensDetails struct {
	CachedTokens     int `json:"cached_tokens" doc:"缓存命中 token"`
	CacheWriteTokens int `json:"cache_write_tokens" doc:"缓存写入 token"`
}

// DecisionOutputTokensDetails 输出明细
type DecisionOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens" doc:"推理 token"`
}

// InputOutputTokens 返回 input_tokens + output_tokens（原始口径，供限流上报）
func (u *OpenAIDecisionUsage) InputOutputTokens() int64 {
	if u == nil {
		return 0
	}
	return int64(u.InputTokens + u.OutputTokens)
}
```

- [ ] **Step 4: 导出别名**

`internal/dto/aliases.go`：在 `OpenAICreateResponseRequest` 别名附近加：

```go
type OpenAICreateDecisionRequest = openai.OpenAICreateDecisionRequest
type OpenAICreateDecisionReq = openai.OpenAICreateDecisionReq
type DecisionInput = openai.DecisionInput
type DecisionInputMessage = openai.DecisionInputMessage
type DecisionQuestion = openai.DecisionQuestion
type DecisionChoice = openai.DecisionChoice
type DecisionChoiceValue = openai.DecisionChoiceValue
type DecisionScoreLevel = openai.DecisionScoreLevel
```

在 `OpenAICreateResponseRsp` 别名附近加：

```go
type OpenAIDecisionRsp = openai.OpenAIDecisionRsp
type OpenAIDecisionUsage = openai.OpenAIDecisionUsage
type DecisionInputTokensDetails = openai.DecisionInputTokensDetails
type DecisionOutputTokensDetails = openai.DecisionOutputTokensDetails
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/openai_decision_dto/`
Expected: PASS（5 个测试）

- [ ] **Step 6: Commit**

```bash
git add internal/dto test/unit/openai_decision_dto
git commit -m "feat(dto): 新增 OpenAI Decision API 请求/响应 DTO"
```

---

### Task 4: 审计 token 归一化 `SetTokensFromDecisionUsage`

**Files:**
- Modify: `internal/dto/asynctask.go`
- Test: `test/unit/audit_token_usage/decision_usage_test.go`（新建）

**Interfaces:**
- Consumes: Task 3 的 `dto.OpenAIDecisionRsp` / `dto.OpenAIDecisionUsage`
- Produces: `(*dto.ModelCallAuditTask).SetTokensFromDecisionUsage(rsp *dto.OpenAIDecisionRsp)`

- [ ] **Step 1: 写失败测试**

新建 `test/unit/audit_token_usage/decision_usage_test.go`：

```go
package audit_token_usage

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestSetTokensFromDecisionUsage_NormalizesNetInput(t *testing.T) {
	t.Parallel()

	rsp := &dto.OpenAIDecisionRsp{
		Usage: &dto.OpenAIDecisionUsage{
			InputTokens:  42,
			OutputTokens: 7,
			TotalTokens:  49,
			InputTokensDetails: &dto.DecisionInputTokensDetails{
				CachedTokens:     12,
				CacheWriteTokens: 5,
			},
		},
	}

	var task dto.ModelCallAuditTask
	task.SetTokensFromDecisionUsage(rsp)

	if task.InputTokens != 25 {
		t.Fatalf("InputTokens = %d, want 25 (42-12-5)", task.InputTokens)
	}
	if task.OutputTokens != 7 {
		t.Fatalf("OutputTokens = %d, want 7", task.OutputTokens)
	}
	if task.CacheReadInputTokens != 12 {
		t.Fatalf("CacheReadInputTokens = %d, want 12", task.CacheReadInputTokens)
	}
	if task.CacheCreationInputTokens != 5 {
		t.Fatalf("CacheCreationInputTokens = %d, want 5", task.CacheCreationInputTokens)
	}
	sum := task.InputTokens + task.CacheReadInputTokens + task.CacheCreationInputTokens
	if sum != rsp.Usage.InputTokens {
		t.Fatalf("four-dim invariant broken: %d != %d", sum, rsp.Usage.InputTokens)
	}
}

func TestSetTokensFromDecisionUsage_ClampsAtZero(t *testing.T) {
	t.Parallel()

	rsp := &dto.OpenAIDecisionRsp{
		Usage: &dto.OpenAIDecisionUsage{
			InputTokens: 3,
			InputTokensDetails: &dto.DecisionInputTokensDetails{
				CachedTokens:     10,
				CacheWriteTokens: 4,
			},
		},
	}

	var task dto.ModelCallAuditTask
	task.SetTokensFromDecisionUsage(rsp)

	if task.InputTokens != 0 {
		t.Fatalf("InputTokens = %d, want 0", task.InputTokens)
	}
}

func TestSetTokensFromDecisionUsage_NilSafety(t *testing.T) {
	t.Parallel()

	var task dto.ModelCallAuditTask
	task.SetTokensFromDecisionUsage(nil)
	task.SetTokensFromDecisionUsage(&dto.OpenAIDecisionRsp{})

	if task.InputTokens != 0 || task.OutputTokens != 0 {
		t.Fatalf("task = %+v, want zero", task)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/audit_token_usage/`
Expected: 编译失败 —— `task.SetTokensFromDecisionUsage undefined`

- [ ] **Step 3: 实现归一化**

`internal/dto/asynctask.go`：在 `SetTokensFromResponseUsage` 之后追加：

```go
// SetTokensFromDecisionUsage 从 Decision API 响应设置 token 计数。
//
// input_tokens_details 下的 cached_tokens / cache_write_tokens 按「input_tokens 的子集」口径处理
// （与字段自身归属一致），故 InputTokens 落净输入（input − cached − cache_write），
// 保证「净输入 + 缓存创建 + 缓存读取 = 上游输入总量」的四维互斥不变量。
//
//	@receiver t *ModelCallAuditTask
//	@param rsp *OpenAIDecisionRsp
func (t *ModelCallAuditTask) SetTokensFromDecisionUsage(rsp *OpenAIDecisionRsp) {
	if rsp == nil || rsp.Usage == nil {
		return
	}
	t.InputTokens = rsp.Usage.InputTokens
	t.OutputTokens = rsp.Usage.OutputTokens
	if details := rsp.Usage.InputTokensDetails; details != nil {
		t.CacheReadInputTokens = details.CachedTokens
		t.CacheCreationInputTokens = details.CacheWriteTokens
	}
	t.InputTokens = max(t.InputTokens-t.CacheReadInputTokens-t.CacheCreationInputTokens, 0)
}
```

- [ ] **Step 4: 审计表列注释同步**

`internal/infrastructure/database/model/model_call_audit.go`：把 `UpstreamProtocol` 与 `APIProtocol` 两列的
`comment` 文案补上 `openai-decision`（仅注释文本变化，列类型不变）：

```go
	UpstreamProtocol         string    `json:"upstream_protocol" gorm:"column:upstream_protocol;not null;default:'';comment:上游协议(openai-chat-completion/openai-response/anthropic-message/openai-decision)"`
	APIProtocol              string    `json:"api_protocol" gorm:"column:api_protocol;not null;default:'';comment:接口层协议(openai-chat-completion/openai-response/anthropic-message/openai-decision)"`
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/audit_token_usage/`
Expected: PASS（3 个测试）

- [ ] **Step 6: Commit**

```bash
git add internal/dto/asynctask.go internal/infrastructure/database/model/model_call_audit.go test/unit/audit_token_usage
git commit -m "feat(dto): Decision usage 归一化为四维互斥 token 口径"
```

---

### Task 5: 协议枚举与兼容路线（native-only）

**Files:**
- Modify: `internal/common/enum/llmproxy_compat.go`
- Modify: `internal/common/enum/provider.go`
- Modify: `internal/application/llmproxy/usecase/compat_route.go`
- Test: `test/unit/llmproxy_usecase/openai_forward_test.go`

**Interfaces:**
- Consumes: Task 1 的 `(*aggregate.Endpoint).SupportOpenAIDecision()`
- Produces:
  - `enum.ProxyAPIOpenAIDecision`
  - `enum.ProtocolOpenAIDecision enum.ProtocolType = "openai-decision"`
  - `SelectCompatRoute(enum.ProxyAPIOpenAIDecision, ep)` → `CompatRouteNative`（端点支持）或 `CompatRouteUnsupported`

- [ ] **Step 1: 写失败测试**

在 `test/unit/llmproxy_usecase/openai_forward_test.go` 末尾追加：

```go
func TestSelectCompatRoute_DecisionNativeOnly(t *testing.T) {
	t.Parallel()

	decisionEp, _ := aggregate.CreateEndpoint(3, "decision-only", "https://api.openai.com", "", "sk-test", false, false, false, true)
	if route := usecase.SelectCompatRoute(enum.ProxyAPIOpenAIDecision, decisionEp); route != enum.CompatRouteNative {
		t.Fatalf("decision-only route = %v, want native", route)
	}

	chatEp, _ := aggregate.CreateEndpoint(4, "chat-only", "https://api.openai.com", "", "sk-test", true, false, false, false)
	if route := usecase.SelectCompatRoute(enum.ProxyAPIOpenAIDecision, chatEp); route != enum.CompatRouteUnsupported {
		t.Fatalf("chat-only route = %v, want unsupported", route)
	}

	if route := usecase.SelectCompatRoute(enum.ProxyAPIOpenAIDecision, nil); route != enum.CompatRouteUnsupported {
		t.Fatalf("nil endpoint route = %v, want unsupported", route)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 -run TestSelectCompatRoute_DecisionNativeOnly ./test/unit/llmproxy_usecase/`
Expected: 编译失败 —— `undefined: enum.ProxyAPIOpenAIDecision`

- [ ] **Step 3: 枚举与路线**

`internal/common/enum/llmproxy_compat.go`：

```go
const (
	ProxyAPIOpenAIChat ProxyAPI = iota
	ProxyAPIOpenAIResponse
	ProxyAPIAnthropicMessage
	ProxyAPIOpenAIDecision
)
```

`internal/common/enum/provider.go` 的 `ProtocolType` 常量块追加：

```go
	// ProtocolOpenAIDecision OpenAI Decision API 协议
	ProtocolOpenAIDecision ProtocolType = "openai-decision"
```

`internal/application/llmproxy/usecase/compat_route.go` 的 switch 追加：

```go
	case enum.ProxyAPIOpenAIDecision:
		if ep.SupportOpenAIDecision() {
			return enum.CompatRouteNative
		}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 -run TestSelectCompatRoute ./test/unit/llmproxy_usecase/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/common/enum internal/application/llmproxy/usecase/compat_route.go test/unit/llmproxy_usecase/openai_forward_test.go
git commit -m "feat(llmproxy): Decision API 协议枚举与 native-only 兼容路线"
```

---

### Task 6: 传输层 `ForwardCreateDecision` 与请求体序列化

**Files:**
- Modify: `internal/common/constant/upstream.go`
- Modify: `internal/application/llmproxy/usecase/port.go`（`OpenAIProxyPort`）
- Modify: `internal/infrastructure/transport/openai.go`
- Modify: `internal/application/llmproxy/util/model.go`
- Modify: `test/unit/llmproxy_usecase/openai_forward_test.go`（mock 补方法）
- Test: `test/unit/llmproxy_usecase/decision_forward_test.go`（新建，本任务只放序列化用例）

**Interfaces:**
- Consumes: Task 3 的 `dto.OpenAICreateDecisionReq`
- Produces:
  - `constant.UpstreamPathOpenAIDecisions = "/decisions"`
  - `OpenAIProxyPort.ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error)`
  - `proxyutil.MarshalOpenAIDecisionBodyForModel(req *dto.OpenAICreateDecisionReq, modelName string) []byte`

- [ ] **Step 1: 写失败测试**

新建 `test/unit/llmproxy_usecase/decision_forward_test.go`：

```go
package llmproxy_usecase

import (
	"testing"

	"github.com/bytedance/sonic"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestMarshalOpenAIDecisionBodyForModel_RewritesModelOnly(t *testing.T) {
	t.Parallel()

	name := "damaged"
	req := &dto.OpenAICreateDecisionReq{
		Model: "exposed-alias",
		Input: dto.DecisionInput{Text: lo.ToPtr("The package arrived with a broken screen.")},
		Questions: []*dto.DecisionQuestion{{
			Type:         "predicate",
			Name:         &name,
			Instructions: "Does the customer report a damaged item?",
		}},
	}

	body := proxyutil.MarshalOpenAIDecisionBodyForModel(req, "gpt-upstream-real")

	var got map[string]any
	if err := sonic.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["model"] != "gpt-upstream-real" {
		t.Fatalf("model = %v, want gpt-upstream-real", got["model"])
	}
	if got["input"] != "The package arrived with a broken screen." {
		t.Fatalf("input = %v, want the original text", got["input"])
	}
	questions, ok := got["questions"].([]any)
	if !ok || len(questions) != 1 {
		t.Fatalf("questions = %v, want 1 item", got["questions"])
	}
	if req.Model != "exposed-alias" {
		t.Fatalf("original request mutated: model = %s", req.Model)
	}
}
```

（本文件顶部 import 需包含 `"github.com/samber/lo"`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 -run TestMarshalOpenAIDecisionBodyForModel ./test/unit/llmproxy_usecase/`
Expected: 编译失败 —— `undefined: proxyutil.MarshalOpenAIDecisionBodyForModel`

- [ ] **Step 3: 序列化 helper**

`internal/application/llmproxy/util/model.go`：在 `MarshalOpenAIResponseBodyForModel` 之后追加：

```go
// MarshalOpenAIDecisionBodyForModel 使用上游模型名序列化 Decision API 请求体，且不修改原请求。
func MarshalOpenAIDecisionBodyForModel(req *dto.OpenAICreateDecisionReq, modelName string) []byte {
	body := *req
	body.Model = modelName
	return lo.Must1(MarshalUpstreamBody(&body))
}
```

- [ ] **Step 4: 上游路径常量与端口方法**

`internal/common/constant/upstream.go`：

```go
	UpstreamPathOpenAIDecisions       = "/decisions"
```

`internal/application/llmproxy/usecase/port.go` 的 `OpenAIProxyPort` 追加：

```go
	ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error)
```

- [ ] **Step 5: 传输层实现**

`internal/infrastructure/transport/openai.go`：在 `ForwardCreateResponse` 之后追加：

```go
// ForwardCreateDecision 转发 OpenAI Decision API 请求（unary，仅 /decisions）。
func (p *openAIProxy) ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	log := logger.WithCtx(ctx)

	resp, err := p.doUpstreamRequest(ctx, ep, body, constant.UpstreamPathOpenAIDecisions)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // ensure body closed on return

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[OpenAIProxy] Read decision upstream response error", zap.Error(err))
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	return respBody, nil
}
```

- [ ] **Step 6: 补 mock 方法**

`test/unit/llmproxy_usecase/openai_forward_test.go` 的 `mockOpenAIProxy` 结构体加字段：

```go
	decisionUnaryCalled bool
	lastDecisionCtx     context.Context
	lastDecisionBody    []byte
	decisionResp        []byte
	decisionErr         error
```

并加方法：

```go
func (p *mockOpenAIProxy) ForwardCreateDecision(ctx context.Context, _ vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	p.decisionUnaryCalled = true
	p.lastDecisionCtx = ctx
	p.lastDecisionBody = append([]byte(nil), body...)
	if p.decisionErr != nil {
		return nil, p.decisionErr
	}
	return p.decisionResp, nil
}
```

- [ ] **Step 7: 运行测试与构建**

Run: `go test -count=1 -run TestMarshalOpenAIDecisionBodyForModel ./test/unit/llmproxy_usecase/ && go build ./...`
Expected: PASS + 构建成功

- [ ] **Step 8: Commit**

```bash
git add internal/common/constant/upstream.go internal/application/llmproxy test/unit/llmproxy_usecase
git commit -m "feat(transport): 新增 Decision API 上游转发与请求体序列化"
```

---

### Task 7: 用例核心转发 + 会话存储 + 审计

**Files:**
- Modify: `internal/application/llmproxy/port/handler.go`（`OpenAIUseCase`）
- Modify: `internal/application/llmproxy/usecase/openai.go`（`CreateDecision`）
- Create: `internal/application/llmproxy/usecase/openai_decision.go`
- Modify: `internal/application/llmproxy/usecase/recorder.go`（`decisionTokenUsage`）
- Test: `test/unit/llmproxy_usecase/decision_forward_test.go`（追加）

**Interfaces:**
- Consumes: Task 3 DTO、Task 4 `SetTokensFromDecisionUsage`、Task 5 路线、Task 6 端口方法
- Produces:
  - `port.OpenAIUseCase.CreateDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest) (Result, error)`
  - `decisionTokenUsage{ rsp *dto.OpenAIDecisionRsp }`（`tokenUsage` 实现）

- [ ] **Step 1: 写失败测试**

在 `test/unit/llmproxy_usecase/decision_forward_test.go` 追加（该文件 import 需补齐为：`context`、`errors`、`net/http`、`testing`、`github.com/bytedance/sonic`、`github.com/samber/lo`、`application/llmproxy/port`、`application/llmproxy/usecase`、`common/enum`、`common/model`、`domain/llmproxy/aggregate`、`internal/dto`）：

```go
// decisionTaskSubmitter 捕获 Decision 路径提交的审计/存储任务。
type decisionTaskSubmitter struct {
	auditTasks   []*dto.ModelCallAuditTask
	storeTasks   []*dto.MessageStoreTask
}

func (s *decisionTaskSubmitter) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	s.auditTasks = append(s.auditTasks, task)
	return nil
}

func (s *decisionTaskSubmitter) SubmitMessageStoreTask(task *dto.MessageStoreTask) error {
	s.storeTasks = append(s.storeTasks, task)
	return nil
}

var _ usecase.TaskSubmitter = (*decisionTaskSubmitter)(nil)

func buildDecisionEndpoint() *aggregate.Endpoint {
	ep, _ := aggregate.CreateEndpoint(5, "decision-endpoint", "https://api.openai.com", "", "sk-test", false, false, false, true)
	return ep
}

func decisionResponse(answers string) []byte {
	return []byte(`{"model":"gpt-upstream-real","answers":` + answers + `,"usage":{"input_tokens":42,"output_tokens":7,"total_tokens":49,"input_tokens_details":{"cached_tokens":12,"cache_write_tokens":5},"output_tokens_details":{"reasoning_tokens":3}}}`)
}

func newDecisionRequest() *dto.OpenAICreateDecisionRequest {
	name := "damaged"
	return &dto.OpenAICreateDecisionRequest{Body: &dto.OpenAICreateDecisionReq{
		Model: "test-alias",
		Input: dto.DecisionInput{Text: lo.ToPtr("The package arrived with a broken screen.")},
		Questions: []*dto.DecisionQuestion{{
			Type:         "predicate",
			Name:         &name,
			Instructions: "Does the customer report a damaged item?",
		}},
	}}
}

func newDecisionUseCase(proxy *mockOpenAIProxy, submitter *decisionTaskSubmitter) port.OpenAIUseCase {
	resolver := &mockResolver{resolveEndpoint: buildDecisionEndpoint(), resolveModel: buildTestModel()}
	return usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, nil, nil)
}

func TestCreateDecision_NativeForwardRewritesModelAndStoresSession(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: decisionResponse(`[{"type":"predicate","name":"damaged","probability":0.95}]`)}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	result, err := uc.CreateDecision(context.Background(), newDecisionRequest())
	if err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if jsonResult.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", jsonResult.StatusCode)
	}

	var upstreamBody map[string]any
	if err := sonic.Unmarshal(proxy.lastDecisionBody, &upstreamBody); err != nil {
		t.Fatalf("unmarshal upstream body: %v", err)
	}
	if upstreamBody["model"] != "test-model" {
		t.Fatalf("upstream model = %v, want test-model (upstream_model)", upstreamBody["model"])
	}

	var exposed map[string]any
	if err := sonic.Unmarshal(jsonResult.Body, &exposed); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}
	if exposed["model"] != "test-alias" {
		t.Fatalf("exposed model = %v, want test-alias", exposed["model"])
	}
	answers, ok := exposed["answers"].([]any)
	if !ok || len(answers) != 1 {
		t.Fatalf("answers = %v, want 1 item", exposed["answers"])
	}

	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
	audit := submitter.auditTasks[0]
	if audit.APIProtocol != enum.ProtocolOpenAIDecision || audit.UpstreamProtocol != enum.ProtocolOpenAIDecision {
		t.Fatalf("audit protocols = %s/%s, want openai-decision", audit.APIProtocol, audit.UpstreamProtocol)
	}
	if audit.UpstreamStatusCode != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", audit.UpstreamStatusCode)
	}
	if audit.InputTokens != 25 || audit.OutputTokens != 7 || audit.CacheReadInputTokens != 12 || audit.CacheCreationInputTokens != 5 {
		t.Fatalf("audit tokens = %+v, want net 25/7/12/5", audit)
	}

	if len(submitter.storeTasks) != 1 {
		t.Fatalf("store tasks = %d, want 1", len(submitter.storeTasks))
	}
	store := submitter.storeTasks[0]
	if len(store.Messages) != 2 {
		t.Fatalf("stored messages = %d, want 2 (user + assistant)", len(store.Messages))
	}
	if store.Messages[0].Content.Text != "The package arrived with a broken screen." {
		t.Fatalf("user message = %q", store.Messages[0].Content.Text)
	}
	if store.Messages[1].Content.Text != `[{"type":"predicate","name":"damaged","probability":0.95}]` {
		t.Fatalf("assistant message = %q", store.Messages[1].Content.Text)
	}
	if store.InputTokens != 42 || store.OutputTokens != 7 {
		t.Fatalf("store tokens = %d/%d, want 42/7", store.InputTokens, store.OutputTokens)
	}
}

func TestCreateDecision_ImageInputStoredAsParts(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: decisionResponse(`[{"type":"predicate","name":"damaged","probability":0.9}]`)}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	detail := "high"
	imageURL := "https://example.com/x.png"
	req := &dto.OpenAICreateDecisionRequest{Body: &dto.OpenAICreateDecisionReq{
		Model: "test-alias",
		Input: dto.DecisionInput{Messages: []*dto.DecisionInputMessage{{
			Role: "user",
			Content: dto.ResponseInputMessageContent{Parts: []*dto.ResponseInputContent{
				{Type: "input_text", Text: lo.ToPtr("damaged?")},
				{Type: "input_image", ImageURL: &imageURL, Detail: &detail},
			}},
		}}},
		Questions: []*dto.DecisionQuestion{{Type: "predicate", Instructions: "damaged?"}},
	}}

	if _, err := uc.CreateDecision(context.Background(), req); err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	if len(submitter.storeTasks) != 1 {
		t.Fatalf("store tasks = %d, want 1", len(submitter.storeTasks))
	}
	parts := submitter.storeTasks[0].Messages[0].Content.Parts
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if parts[1].Type != enum.ContentPartTypeImageURL || parts[1].ImageURL != imageURL || parts[1].ImageDetail != detail {
		t.Fatalf("image part = %+v", parts[1])
	}
}

func TestCreateDecision_ModelNotSupported(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{}
	submitter := &decisionTaskSubmitter{}
	resolver := &mockResolver{resolveEndpoint: buildCompatEndpoint("chat-only", true, false, false), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, nil, nil)

	_, err := uc.CreateDecision(context.Background(), newDecisionRequest())
	if err == nil {
		t.Fatal("CreateDecision() must fail when endpoint does not support decisions")
	}
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("error = %T, want *port.ProxyError", err)
	}
	if proxy.decisionUnaryCalled {
		t.Fatal("upstream must not be called when model is unsupported")
	}
}

func TestCreateDecision_UpstreamErrorPassthrough(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionErr: &model.UpstreamError{
		StatusCode: http.StatusTooManyRequests,
		Headers:    map[string]string{"content-type": "application/json"},
		Body:       `{"error":{"message":"rate limited"}}`,
	}}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	_, err := uc.CreateDecision(context.Background(), newDecisionRequest())
	var proxyErr *port.ProxyError
	if !errors.As(err, &proxyErr) {
		t.Fatalf("error = %T, want *port.ProxyError", err)
	}
	if proxyErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", proxyErr.StatusCode)
	}
	if string(proxyErr.Body) != `{"error":{"message":"rate limited"}}` {
		t.Fatalf("body = %s, want upstream body", proxyErr.Body)
	}
	if len(submitter.auditTasks) != 1 || submitter.auditTasks[0].UpstreamStatusCode != http.StatusTooManyRequests {
		t.Fatalf("audit = %+v, want 429 failure audit", submitter.auditTasks)
	}
}

func TestCreateDecision_UnparsableBodyStillPassesThrough(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: []byte(`not-json`)}
	submitter := &decisionTaskSubmitter{}
	uc := newDecisionUseCase(proxy, submitter)

	result, err := uc.CreateDecision(context.Background(), newDecisionRequest())
	if err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if string(jsonResult.Body) != "not-json" {
		t.Fatalf("body = %s, want passthrough", jsonResult.Body)
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("store tasks = %d, want 0", len(submitter.storeTasks))
	}
	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 -run TestCreateDecision ./test/unit/llmproxy_usecase/`
Expected: 编译失败 —— `uc.CreateDecision undefined`

- [ ] **Step 3: 端口与用例编排**

`internal/application/llmproxy/port/handler.go` 的 `OpenAIUseCase` 追加：

```go
	CreateDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest) (Result, error)
```

`internal/application/llmproxy/usecase/openai.go`：`ListModels` 之后追加：

```go
// CreateDecision 处理 OpenAI Decision API 请求（native-only，无流式形态）。
func (u *openAIUseCase) CreateDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest) (port.Result, error) {
	log := logger.WithCtx(ctx)

	model := req.Body.Model
	var compatRoute enum.CompatRoute
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	ep, m, err := u.resolver.Resolve(ctx, userID, vo.EndpointAlias(model), func(ep *aggregate.Endpoint) bool {
		compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIDecision, ep)
		return compatRoute != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Decision API model not found or unsupported", zap.String("model", model), zap.Error(err))
		return nil, proxyutil.SendOpenAIModelNotFoundError(model)
	}

	upstream := toTransportEndpoint(m, ep, false)
	body := proxyutil.MarshalOpenAIDecisionBodyForModel(req.Body, upstream.Model)

	startTime := time.Now()
	respBody, err := u.openAIProxy.ForwardCreateDecision(ctx, upstream, body)
	totalMs := time.Since(startTime).Milliseconds()
	if err != nil {
		auditFailure(ctx, m, u.taskSubmitter, u.tokenMetrics, model, ep.Name(), enum.ProtocolOpenAIDecision, totalMs, err)
		return nil, ProxyErrorFromUpstream(err, enum.ProtocolKindOpenAI, openAIInternalErrorBody)
	}

	replaced := proxyutil.ReplaceModelInBody(respBody, model)
	headers := buildPassthroughHeaders(ctx)
	headers[constant.HTTPHeaderContentType] = constant.HTTPContentTypeJSON

	out := callOutcome{
		model:               m,
		endpoint:            ep.Name(),
		upstreamProtocol:    enum.ProtocolOpenAIDecision,
		apiProtocol:         enum.ProtocolOpenAIDecision,
		firstTokenLatencyMs: totalMs,
		successStatus:       true,
	}
	var rsp dto.OpenAIDecisionRsp
	if parseErr := sonic.Unmarshal(replaced, &rsp); parseErr != nil {
		log.Debug("[OpenAIUseCase] Failed to parse Decision API response body", zap.Error(parseErr))
	} else {
		u.storeDecisionSession(ctx, req, &rsp, m.ModelID())
		out.usage = decisionTokenUsage{&rsp}
	}
	recordModelCall(ctx, u.taskSubmitter, u.tokenMetrics, out)

	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       replaced,
		Protocol:   enum.ProtocolKindOpenAI,
	}, nil
}
```

`internal/application/llmproxy/usecase/openai.go` 需补 import：`net/http`、`time`、`github.com/bytedance/sonic`（`zap`/`constant`/`enum`/`aggregate`/`vo`/`dto`/`proxyutil`/`port`/`util`/`logger` 已存在）。

- [ ] **Step 4: 会话存储与 usage 适配器**

新建 `internal/application/llmproxy/usecase/openai_decision.go`：

```go
package usecase

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	commonvo "github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// storeDecisionSession 把一次 Decision 调用落成一条会话：user=input、assistant=answers 原始 JSON 文本。
//
// Decision 的 input 是单一逻辑载荷（非多轮对话），故多个 input 消息合并为一条 user 消息；
// answers 以原始 JSON 文本存储，信息无损且不发明渲染格式。
func (u *openAIUseCase) storeDecisionSession(ctx context.Context, req *dto.OpenAICreateDecisionRequest, rsp *dto.OpenAIDecisionRsp, modelID string) {
	if util.CtxValueBool(ctx, constant.CtxKeySkipStore) {
		return
	}
	log := logger.WithCtx(ctx)

	var inputTokens, outputTokens int
	if rsp.Usage != nil {
		inputTokens = rsp.Usage.InputTokens
		outputTokens = rsp.Usage.OutputTokens
	}

	if err := u.taskSubmitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:        util.CopyContextValues(ctx),
		APIKeyName: util.CtxValueString(ctx, constant.CtxKeyAPIKeyName),
		APIKeyID:   util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
		ModelID:    modelID,
		Messages: []*commonvo.UnifiedMessage{
			buildDecisionInputMessage(req.Body.Input),
			{
				Role:    enum.RoleAssistant,
				Content: &commonvo.UnifiedContent{Text: string(rsp.Answers)},
			},
		},
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}); err != nil {
		log.Error("[OpenAIUseCase] Failed to submit decision message store task", zap.Error(err))
	}
}

// buildDecisionInputMessage 把 Decision input 合并为一条 user 统一消息。
func buildDecisionInputMessage(input dto.DecisionInput) *commonvo.UnifiedMessage {
	if input.Text != nil {
		return &commonvo.UnifiedMessage{Role: enum.RoleUser, Content: &commonvo.UnifiedContent{Text: *input.Text}}
	}

	parts := make([]*commonvo.UnifiedContentPart, 0, len(input.Messages))
	for _, msg := range input.Messages {
		if msg == nil {
			continue
		}
		if msg.Content.Text != "" {
			parts = append(parts, &commonvo.UnifiedContentPart{Type: enum.ContentPartTypeText, Text: msg.Content.Text})
		}
		for _, part := range msg.Content.Parts {
			if part == nil {
				continue
			}
			parts = append(parts, &commonvo.UnifiedContentPart{
				Type:        enum.ContentPartType(part.Type),
				Text:        lo.FromPtr(part.Text),
				ImageURL:    lo.FromPtr(part.ImageURL),
				ImageDetail: lo.FromPtr(part.Detail),
			})
		}
	}
	return &commonvo.UnifiedMessage{Role: enum.RoleUser, Content: &commonvo.UnifiedContent{Parts: parts}}
}
```

`internal/application/llmproxy/usecase/recorder.go`：在 `responseTokenUsage` 之后追加：

```go
type decisionTokenUsage struct{ rsp *dto.OpenAIDecisionRsp }

func (u decisionTokenUsage) apply(task *dto.ModelCallAuditTask) {
	task.SetTokensFromDecisionUsage(u.rsp)
}

func (u decisionTokenUsage) reportable() int64 {
	if u.rsp == nil || u.rsp.Usage == nil {
		return 0
	}
	return u.rsp.Usage.InputOutputTokens()
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 -run TestCreateDecision ./test/unit/llmproxy_usecase/ && go test -count=1 ./test/unit/llmproxy_usecase/`
Expected: PASS（5 个新增用例 + 既有用例全绿）

- [ ] **Step 6: Commit**

```bash
git add internal/application/llmproxy
git commit -m "feat(llmproxy): Decision API 核心转发、会话存储与审计"
```

---

### Task 8: 触发词拦截（deny → refusal，omit → 跳过存储）

**Files:**
- Modify: `internal/application/llmproxy/usecase/trigger_check.go`（`extractDecisionText`、`checkDecisionContent`）
- Modify: `internal/application/llmproxy/usecase/openai.go`（`CreateDecision` 前置拦截分支）
- Modify: `internal/application/llmproxy/util/trigger_content_filter.go`（`BuildDecisionRefusalBody`）
- Test: `test/unit/llmproxy_usecase/decision_forward_test.go`（追加）

**Interfaces:**
- Consumes: Task 7 的 `CreateDecision`
- Produces:
  - `(*openAIUseCase).checkDecisionContent(req *dto.OpenAICreateDecisionRequest) []uint`
  - `proxyutil.BuildDecisionRefusalBody(model string, questions []*dto.DecisionQuestion) port.Result`

- [ ] **Step 1: 写失败测试**

在 `test/unit/llmproxy_usecase/decision_forward_test.go` 追加：

```go
// stubTriggerChecker 以固定命中结果驱动 Decision 拦截分支。
type stubTriggerChecker struct {
	matched []uint
	words   []string
	denyIDs []uint
	omitIDs []uint
}

func (s *stubTriggerChecker) Check(string) []uint                  { return s.matched }
func (s *stubTriggerChecker) MatchedWords([]uint) []string         { return s.words }
func (s *stubTriggerChecker) DenyIDs([]uint) []uint                { return s.denyIDs }
func (s *stubTriggerChecker) OmitIDs([]uint) []uint                { return s.omitIDs }
func (s *stubTriggerChecker) CaptureIDs([]uint) []uint             { return nil }
func (s *stubTriggerChecker) IncrementHits(context.Context, []uint) error { return nil }

var _ usecase.TriggerChecker = (*stubTriggerChecker)(nil)

func newInterceptDecisionUseCase(proxy *mockOpenAIProxy, submitter *decisionTaskSubmitter, checker *stubTriggerChecker) port.OpenAIUseCase {
	resolver := &mockResolver{resolveEndpoint: buildDecisionEndpoint(), resolveModel: buildTestModel()}
	return usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, submitter, checker, nil)
}

func TestCreateDecision_DenyReturnsRefusalPerQuestion(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{}
	submitter := &decisionTaskSubmitter{}
	checker := &stubTriggerChecker{matched: []uint{1}, words: []string{"blocked"}, denyIDs: []uint{1}}
	uc := newInterceptDecisionUseCase(proxy, submitter, checker)

	result, err := uc.CreateDecision(context.Background(), newDecisionRequest())
	if err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	jsonResult, ok := result.(*port.JSONResult)
	if !ok {
		t.Fatalf("result = %T, want *port.JSONResult", result)
	}
	if proxy.decisionUnaryCalled {
		t.Fatal("upstream must not be called on deny")
	}

	var body struct {
		Model   string `json:"model"`
		Answers []struct {
			Type string  `json:"type"`
			Name *string `json:"name"`
		} `json:"answers"`
		Usage map[string]any `json:"usage"`
	}
	if err := sonic.Unmarshal(jsonResult.Body, &body); err != nil {
		t.Fatalf("unmarshal refusal body: %v", err)
	}
	if body.Model != "test-alias" {
		t.Fatalf("model = %s, want test-alias", body.Model)
	}
	if len(body.Answers) != 1 || body.Answers[0].Type != "refusal" {
		t.Fatalf("answers = %+v, want 1 refusal", body.Answers)
	}
	if body.Answers[0].Name == nil || *body.Answers[0].Name != "damaged" {
		t.Fatalf("refusal name = %v, want damaged", body.Answers[0].Name)
	}
	if body.Usage == nil {
		t.Fatal("usage must be present")
	}
	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
	if submitter.auditTasks[0].ErrorMessage == "" {
		t.Fatal("deny audit must carry trigger word remark")
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("store tasks = %d, want 0", len(submitter.storeTasks))
	}
}

func TestCreateDecision_OmitSkipsSessionStore(t *testing.T) {
	t.Parallel()

	proxy := &mockOpenAIProxy{decisionResp: decisionResponse(`[{"type":"predicate","name":"damaged","probability":0.9}]`)}
	submitter := &decisionTaskSubmitter{}
	checker := &stubTriggerChecker{matched: []uint{2}, words: []string{"private"}, omitIDs: []uint{2}}
	uc := newInterceptDecisionUseCase(proxy, submitter, checker)

	if _, err := uc.CreateDecision(context.Background(), newDecisionRequest()); err != nil {
		t.Fatalf("CreateDecision() error: %v", err)
	}
	if !proxy.decisionUnaryCalled {
		t.Fatal("upstream must be called when only omit matches")
	}
	if len(submitter.storeTasks) != 0 {
		t.Fatalf("store tasks = %d, want 0 (omit)", len(submitter.storeTasks))
	}
	if len(submitter.auditTasks) != 1 {
		t.Fatalf("audit tasks = %d, want 1", len(submitter.auditTasks))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 -run 'TestCreateDecision_Deny|TestCreateDecision_Omit' ./test/unit/llmproxy_usecase/`
Expected: FAIL —— deny 用例实际调用了上游 / omit 用例提交了存储任务

- [ ] **Step 3: 文本提取**

`internal/application/llmproxy/usecase/trigger_check.go`：在 `extractOpenAIResponseText` 之后追加：

```go
// extractDecisionText 提取 Decision API 请求中的全部用户可控文本。
//
// 覆盖：input（字符串或消息 content 的文本/parts）、questions[].instructions、
// choices[].description、levels[].label/description。
func extractDecisionText(req *dto.OpenAICreateDecisionRequest) string {
	var buf strings.Builder

	if req.Body.Input.Text != nil {
		buf.WriteString(*req.Body.Input.Text)
	}
	for _, msg := range req.Body.Input.Messages {
		if msg == nil {
			continue
		}
		buf.WriteString(msg.Content.Text)
		for _, part := range msg.Content.Parts {
			if part != nil && part.Text != nil {
				buf.WriteString(*part.Text)
			}
		}
	}

	for _, q := range req.Body.Questions {
		if q == nil {
			continue
		}
		buf.WriteString(q.Instructions)
		for _, choice := range q.Choices {
			if choice != nil {
				buf.WriteString(lo.FromPtr(choice.Description))
				if choice.Value.StringValue != nil {
					buf.WriteString(*choice.Value.StringValue)
				}
			}
		}
		for _, level := range q.Levels {
			if level != nil {
				buf.WriteString(level.Label)
				buf.WriteString(lo.FromPtr(level.Description))
			}
		}
	}

	return buf.String()
}

func (u *openAIUseCase) checkDecisionContent(req *dto.OpenAICreateDecisionRequest) []uint {
	if u.triggerChecker == nil {
		return nil
	}
	return u.triggerChecker.Check(extractDecisionText(req))
}
```

- [ ] **Step 4: 拒绝响应构造**

`internal/application/llmproxy/util/trigger_content_filter.go`：追加：

```go
// decisionRefusalAnswer 触发词拦截时逐问代替模型给出的 refusal 答案。
type decisionRefusalAnswer struct {
	Type string  `json:"type"`
	Name *string `json:"name"`
}

// decisionRefusalUsage 零值 usage（响应契约要求该对象存在）。
type decisionRefusalUsage struct {
	InputTokens         int                                `json:"input_tokens"`
	OutputTokens        int                                `json:"output_tokens"`
	TotalTokens         int                                `json:"total_tokens"`
	InputTokensDetails  decisionRefusalInputTokensDetails  `json:"input_tokens_details"`
	OutputTokensDetails decisionRefusalOutputTokensDetails `json:"output_tokens_details"`
}

type decisionRefusalInputTokensDetails struct {
	CachedTokens     int `json:"cached_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

type decisionRefusalOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type decisionRefusalBody struct {
	Model   string                  `json:"model"`
	Answers []decisionRefusalAnswer `json:"answers"`
	Usage   decisionRefusalUsage    `json:"usage"`
}

// BuildDecisionRefusalBody 构造触发词 deny 命中的 Decision 响应：HTTP 200，
// 每个 question 回一条协议原生的 refusal 答案，不调用上游。
//
//	@param model string 响应中暴露的模型名（请求模型别名）
//	@param questions []*dto.DecisionQuestion 请求中的问题列表
//	@return port.Result
func BuildDecisionRefusalBody(model string, questions []*dto.DecisionQuestion) port.Result {
	answers := make([]decisionRefusalAnswer, 0, len(questions))
	for _, q := range questions {
		answer := decisionRefusalAnswer{Type: "refusal"}
		if q != nil {
			answer.Name = q.Name
		}
		answers = append(answers, answer)
	}
	body := lo.Must1(sonic.Marshal(&decisionRefusalBody{
		Model:   model,
		Answers: answers,
		Usage:   decisionRefusalUsage{},
	}))
	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       body,
		Protocol:   enum.ProtocolKindOpenAI,
	}
}
```

注意：`util` 包不能引用 `enum.ProtocolOpenAIDecision` 作为答案 type 字面量？可以，但此处用字面量 `"refusal"` 即可（协议字段值，非项目枚举）。

- [ ] **Step 5: 接入拦截分支**

`internal/application/llmproxy/usecase/openai.go` 的 `CreateDecision`：在 resolve 成功之后、`upstream := ...` 之前插入：

```go
	if matched := u.checkDecisionContent(req); len(matched) > 0 {
		_ = u.triggerChecker.IncrementHits(ctx, matched) //nolint:errcheck // best-effort hit counting

		if denyIDs := u.triggerChecker.DenyIDs(matched); len(denyIDs) > 0 {
			words := u.triggerChecker.MatchedWords(denyIDs)
			auditTask := &dto.ModelCallAuditTask{
				Ctx:              util.CopyContextValues(ctx),
				ModelID:          m.ModelID(),
				Endpoint:         ep.Name(),
				UpstreamProtocol: enum.ProtocolOpenAIDecision,
				APIProtocol:      enum.ProtocolOpenAIDecision,
				ErrorMessage:     fmt.Sprintf(constant.TriggerAuditRemarkTemplate, formatTriggerWords(words)),
			}
			_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
			return proxyutil.BuildDecisionRefusalBody(model, req.Body.Questions), nil
		}

		// capture 短路未实现：Decision 的 input 是待评估文本而非多轮对话，
		// 不存在「最后一条用户提问」概念（见设计文档 §2.2）。
		if len(u.triggerChecker.OmitIDs(matched)) > 0 {
			ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
		}
	}
```

`fmt` 已在 `openai.go` 的 import 中。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/llmproxy_usecase/`
Expected: PASS（全部）

- [ ] **Step 7: Commit**

```bash
git add internal/application/llmproxy test/unit/llmproxy_usecase
git commit -m "feat(llmproxy): Decision API 触发词拦截（deny 回 refusal、omit 跳过存储）"
```

---

### Task 9: Handler 与 Router 注册

**Files:**
- Modify: `internal/handler/openai.go`
- Modify: `internal/router/openai.go`

**Interfaces:**
- Consumes: Task 7 的 `port.OpenAIUseCase.CreateDecision`
- Produces: `POST /api/openai/v1/decisions` 路由（API Key 鉴权 + 双限流 + body 上限）

- [ ] **Step 1: Handler 方法**

`internal/handler/openai.go`：`OpenAIHandler` 接口追加：

```go
	HandleCreateDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest) (*huma.StreamResponse, error)
```

实现（`HandleCreateResponse` 之后）：

```go
// HandleCreateDecision 处理 Decision API 请求
//
//	@receiver h *openAIHandler
//	@param ctx context.Context
//	@param req *dto.OpenAICreateDecisionRequest
//	@return *huma.StreamResponse
//	@return error
func (h *openAIHandler) HandleCreateDecision(ctx context.Context, req *dto.OpenAICreateDecisionRequest) (*huma.StreamResponse, error) {
	result, err := h.uc.CreateDecision(ctx, req)
	return apiutil.AdaptProxyResult(ctx, result, err, openAIInternalFallbackBody)
}
```

说明：Decision 无流式形态，**不挂** `WithStreamLifecycle`（该回调只在 adapter 真实写 SSE 时触发，unary 路径挂载即死代码）。

- [ ] **Step 2: 路由注册**

`internal/router/openai.go`：`/responses` 之后追加：

```go
	huma.Register(openaiGroup, huma.Operation{
		OperationID:  "createDecision",
		Method:       http.MethodPost,
		Path:         "/decisions",
		Summary:      "Create decision",
		Description:  "Answers classification or scoring questions about the same input.",
		Tags:         []string{constant.TagOpenAI},
		MaxBodyBytes: constant.MaxLLMProxyBodyBytes,
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(cache, "callProxyLLM", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLM, constant.LimitCallProxyLLM),
			middleware.TokenBucketTokenRateLimiterMiddleware(cache, "callProxyLLMToken", constant.CtxKeyAPIKeyID, constant.PeriodCallProxyLLMToken, constant.LimitCallProxyLLMToken),
		},
		Security: []map[string][]string{
			{constant.SecuritySchemeAPIKey: {}},
		},
	}, openaiHandler.HandleCreateDecision)
```

- [ ] **Step 3: 构建与 lint**

Run: `go build ./... && make lint`
Expected: 构建成功；conv + static 全绿

- [ ] **Step 4: 确认 OpenAPI 路由已注册**

Run: `grep -rn '"/decisions"' internal/router/openai.go`
Expected: 命中 `Path: "/decisions",`

- [ ] **Step 5: Commit**

```bash
git add internal/handler/openai.go internal/router/openai.go
git commit -m "feat(router): 注册 POST /api/openai/v1/decisions 代理路由"
```

---

### Task 10: E2E 用例

**Files:**
- Create: `test/e2e/openai_decision/openai_decision_test.go`
- Create: `test/e2e/openai_decision/fixtures/requests/predicate.json`
- Create: `test/e2e/openai_decision/fixtures/requests/choice.json`
- Create: `test/e2e/openai_decision/fixtures/requests/score.json`

**Interfaces:**
- Consumes: Task 9 的路由；环境变量 `BASE_URL`、`API_KEY`、`DECISION_MODEL`（可选，缺省用 fixture 内模型名）
- Produces: CLI 可跑的 E2E 断言

- [ ] **Step 1: 写 fixture**

`test/e2e/openai_decision/fixtures/requests/predicate.json`：

```json
{
  "model": "REPLACE_WITH_DECISION_MODEL",
  "input": "The package arrived with a broken screen.",
  "questions": [
    {
      "type": "predicate",
      "name": "damaged",
      "instructions": "Does the customer report a damaged item?"
    }
  ]
}
```

`test/e2e/openai_decision/fixtures/requests/choice.json`：

```json
{
  "model": "REPLACE_WITH_DECISION_MODEL",
  "input": "The package arrived with a broken screen.",
  "questions": [
    {
      "type": "choice",
      "name": "sentiment",
      "instructions": "What is the customer sentiment?",
      "choices": [
        { "value": "negative", "description": "unhappy" },
        { "value": "positive", "description": "happy" },
        { "value": true, "description": "boolean marker" }
      ]
    }
  ]
}
```

`test/e2e/openai_decision/fixtures/requests/score.json`：

```json
{
  "model": "REPLACE_WITH_DECISION_MODEL",
  "input": "The package arrived with a broken screen.",
  "questions": [
    {
      "type": "score",
      "name": "severity",
      "instructions": "Rate the complaint severity.",
      "levels": [
        { "label": "low" },
        { "label": "medium" },
        { "label": "high", "description": "requires refund" }
      ]
    }
  ]
}
```

- [ ] **Step 2: 写 E2E 测试**

`test/e2e/openai_decision/openai_decision_test.go`：

```go
// Package openai_decision 覆盖 POST /api/openai/v1/decisions 的端到端行为。
package openai_decision

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const e2eHTTPTimeout = 90 * time.Second

// mustE2EEnv 返回 (baseURL, apiKey, model)；E2E 默认离线 skip。
func mustE2EEnv(t *testing.T) (baseURL, apiKey, model string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	model = os.Getenv("DECISION_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		t.Skip("BASE_URL, API_KEY and DECISION_MODEL are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey, model
}

func loadFixture(t *testing.T, name, model string) []byte {
	t.Helper()
	data, err := os.ReadFile("./fixtures/requests/" + name + ".json")
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return []byte(strings.ReplaceAll(string(data), "REPLACE_WITH_DECISION_MODEL", model))
}

func postDecisions(t *testing.T, baseURL, apiKey string, body []byte) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: e2eHTTPTimeout}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/openai/v1/decisions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

type decisionRsp struct {
	Model   string `json:"model"`
	Answers []struct {
		Type        string   `json:"type"`
		Name        *string  `json:"name"`
		Probability *float64 `json:"probability"`
		Choice      sonic.NoCopyRawMessage `json:"choice"`
		Confidence  *float64 `json:"confidence"`
		Score       *float64 `json:"score"`
	} `json:"answers"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func assertDecisionResponse(t *testing.T, rsp *http.Response, wantType, model string) decisionRsp {
	t.Helper()
	defer func() { _ = rsp.Body.Close() }() //nolint:errcheck // test cleanup

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if rsp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rsp.StatusCode, body)
	}

	var parsed decisionRsp
	if err := sonic.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, body)
	}
	if parsed.Model != model {
		t.Fatalf("model = %s, want %s (exposed alias)", parsed.Model, model)
	}
	if len(parsed.Answers) != 1 {
		t.Fatalf("answers = %d, want 1", len(parsed.Answers))
	}
	if parsed.Answers[0].Type != wantType && parsed.Answers[0].Type != "refusal" {
		t.Fatalf("answer type = %s, want %s or refusal", parsed.Answers[0].Type, wantType)
	}
	if parsed.Usage.TotalTokens <= 0 {
		t.Fatalf("usage.total_tokens = %d, want > 0", parsed.Usage.TotalTokens)
	}
	return parsed
}

func TestCreateDecision_Predicate(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, model := mustE2EEnv(t)

	rsp := postDecisions(t, baseURL, apiKey, loadFixture(t, "predicate", model))
	parsed := assertDecisionResponse(t, rsp, "predicate", model)
	if parsed.Answers[0].Probability == nil {
		t.Fatal("predicate answer must carry probability")
	}
}

func TestCreateDecision_ChoiceWithBooleanValue(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, model := mustE2EEnv(t)

	rsp := postDecisions(t, baseURL, apiKey, loadFixture(t, "choice", model))
	parsed := assertDecisionResponse(t, rsp, "choice", model)
	if parsed.Answers[0].Confidence == nil {
		t.Fatal("choice answer must carry confidence")
	}
}

func TestCreateDecision_Score(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, model := mustE2EEnv(t)

	rsp := postDecisions(t, baseURL, apiKey, loadFixture(t, "score", model))
	parsed := assertDecisionResponse(t, rsp, "score", model)
	if parsed.Answers[0].Score == nil {
		t.Fatal("score answer must carry score")
	}
}

func TestCreateDecision_UnknownModelReturnsError(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, _ := mustE2EEnv(t)

	body := []byte(`{"model":"definitely-not-a-configured-alias","input":"x","questions":[{"type":"predicate","instructions":"y"}]}`)
	rsp := postDecisions(t, baseURL, apiKey, body)
	defer func() { _ = rsp.Body.Close() }() //nolint:errcheck // test cleanup

	respBody, _ := io.ReadAll(rsp.Body) //nolint:errcheck // best-effort read
	if rsp.StatusCode != http.StatusNotFound && rsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s, want 404/400", rsp.StatusCode, respBody)
	}
}
```

- [ ] **Step 3: 离线跑通 skip 路径与构建**

Run: `go test -count=1 -v ./test/e2e/openai_decision/`
Expected: 全部 `SKIP`（无 `BASE_URL`/`API_KEY`/`DECISION_MODEL`），无编译错误

- [ ] **Step 4: Commit**

```bash
git add test/e2e/openai_decision
git commit -m "test(e2e): 新增 OpenAI Decision API 端到端用例"
```

---

### Task 11: 文档回写与全量验证

**Files:**
- Modify: `CONTEXT.md`
- Test: 全量 `make lint` / `make test`

**Interfaces:**
- Consumes: Task 1–10 全部产出
- Produces: 词汇表与代码一致；全量门禁通过；经验沉淀

- [ ] **Step 1: 回写 CONTEXT.md**

`CONTEXT.md` 的 **ProtocolType（协议类型）** 条目改为四成员：

```markdown
**ProtocolType（协议类型）**:
网关支持的四种上游 LLM 协议：`openai-chat-completion`（OpenAI Chat Completions）、`openai-response`（OpenAI Response API）、`anthropic-message`（Anthropic Messages）、`openai-decision`（OpenAI Decision API）。决定请求的序列化/反序列化方式和传输通道。网关支持跨协议转换（如 OpenAI 接口调用 Anthropic 上游），但 Decision 仅支持原生转发（不做跨协议转换）。
```

`CONTEXT.md` 的 **Endpoint（上游端点）** 条目把接口支持标记改为四个，并在其后新增术语：

```markdown
**DecisionAPI（决策接口）**:
OpenAI `POST /v1/decisions`，对同一 `input`（文本或含文本+图片的 user 消息）批量回答分类/打分问题，按提问顺序返回 `answers`。问题三型：`predicate`（估算陈述为真的概率）/ `choice`（从 2~255 个选项中选择，值可为 string 或 bool）/ `score`（按有序等级打分）；答案四态：`predicate` / `choice` / `score` / `refusal`（模型拒答）。网关仅支持原生转发，接口无流式形态。
_Avoid_: decision completion, classify api
```

- [ ] **Step 2: 全量门禁**

Run: `make lint && make test`
Expected: lint 全绿；测试全绿（E2E 无环境变量时 skip）

- [ ] **Step 3: 过度工程审查**

Run: 使用 `ponytail-review` skill 审查本次 diff（spec §2.2「明确不做」清单可作对照）
Expected: 无可删的投机抽象/重复造轮子/死代码；若有，就地修复并重跑 Step 2

- [ ] **Step 4: 沉淀工程经验**

用 Serena `write_memory`（名称 `llmproxy/openai-decision-api-2026-10-09`）记录：Endpoint 能力开关的 12 处改动清单、Decision native-only 的取舍理由、`input_tokens_details` 两维按子集处理的假设、与 `feature/endpoint-scheduling-pricing-2026-10-09` 的冲突文件清单。

- [ ] **Step 5: Commit**

```bash
git add CONTEXT.md
git commit -m "docs(agents): CONTEXT 补充 Decision API 与四类协议术语"
```

- [ ] **Step 6: 部署后跑真实 E2E（需用户提供支持 /decisions 的上游模型别名）**

先合并/推送 `master` 触发 `docker-publish.yml`，再：

Run: `BASE_URL=https://api.lvlvko.top API_KEY=<user-key> DECISION_MODEL=<alias> go test -count=1 -v ./test/e2e/openai_decision/`
Expected: 4 个用例全部 PASS

若失败：取响应头 `X-Trace-Id`，按 `query-prod-log` 在 `ap-guangzhou` 追链路定位根因。

- [ ] **Step 7: 人工复核审计口径（缓存维假设）**

在 Web 审计页筛 `APIProtocol = openai-decision` 的记录，比对上游返回的 `usage.input_tokens_details` 与审计行的「输入/缓存读/缓存写」三维之和。
Expected: `净输入 + 缓存读 + 缓存写 = 上游 input_tokens`；若不成立，说明 `cache_write_tokens` 不是 `input_tokens` 子集，需按实测调整 `SetTokensFromDecisionUsage`（只扣 `cached_tokens`）。

---

## 附：任务依赖顺序

```
Task 1 (后端配置) ──┬─> Task 2 (前端配置)          [仅依赖字段名]
                   └─> Task 5 (协议枚举/路线)
Task 3 (DTO) ──────┬─> Task 4 (审计归一化)
                   └─> Task 6 (传输层) ──> Task 7 (用例核心) ──> Task 8 (触发词) ──> Task 9 (路由) ──> Task 10 (E2E) ──> Task 11 (验证)
```

Task 1 与 Task 3 无依赖，可并行；Task 2 只依赖 Task 1 的 JSON 字段名。
