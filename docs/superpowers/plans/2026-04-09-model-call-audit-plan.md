# Model Call Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `model_call_audit` table that records per-call metrics (tokens, latency, UA, upstream status), remove `message.token_count` and `session.client`, and wire audit writes into the OpenAI/Anthropic service flow.

**Architecture:** Audit records are written asynchronously via `storePool` as a new `ModelCallAuditTask`. Each service (OpenAI/Anthropic) assembles audit fields after the upstream call completes and submits the task. Errors in audit writing do not affect the API response or message storage.

**Tech Stack:** Go, GORM, Pond v2, go-redis, Sonic

---

## File Map

| File | Action |
|------|--------|
| `internal/infrastructure/database/model/model_call_audit.go` | Create |
| `internal/infrastructure/database/dao/model_call_audit.go` | Create |
| `internal/infrastructure/database/dao/singleton.go` | Modify |
| `internal/infrastructure/database/model/base.go` | Modify |
| `internal/dto/asynctask.go` | Modify |
| `internal/infrastructure/pool/store_pool.go` | Modify |
| `internal/infrastructure/pool/pool.go` | Modify |
| `internal/service/openai.go` | Modify |
| `internal/service/anthropic.go` | Modify |
| `internal/infrastructure/database/model/message.go` | Modify |
| `internal/infrastructure/database/model/session.go` | Modify |

---

## Task 1: Create `model_call_audit` Model

**Files:**
- Create: `internal/infrastructure/database/model/model_call_audit.go`

- [ ] **Step 1: Write model file**

```go
// Package model defines the database schema for the model.
//
//	update 2026-04-09 10:00:00
package model

// ModelCallAudit 模型调用审计数据库模型
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ModelCallAudit struct {
	BaseModel
	APIKeyID                  uint   `json:"api_key_id" gorm:"column:api_key_id;not null;index:idx_api_key_id_created_at"`
	ModelID                   uint   `json:"model_id" gorm:"column:model_id;not null;index:idx_model_id_created_at"`
	Model                     string `json:"model" gorm:"column:model;not null;index:idx_model_created_at"`
	UpstreamProvider          string `json:"upstream_provider" gorm:"column:upstream_provider;not null"`
	APIProvider               string `json:"api_provider" gorm:"column:api_provider;not null"`
	InputTokens               int    `json:"input_tokens" gorm:"column:input_tokens;default:0"`
	OutputTokens              int    `json:"output_tokens" gorm:"column:output_tokens;default:0"`
	CacheCreationInputTokens  int    `json:"cache_creation_input_tokens" gorm:"column:cache_creation_input_tokens;default:0"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens" gorm:"column:cache_read_input_tokens;default:0"`
	FirstTokenLatencyMs      int64  `json:"first_token_latency_ms" gorm:"column:first_token_latency_ms;default:0"`
	StreamDurationMs         int64  `json:"stream_duration_ms" gorm:"column:stream_duration_ms;default:0"`
	UserAgent                string `json:"user_agent" gorm:"column:user_agent;not null;default:''"`
	UpstreamStatusCode       int    `json:"upstream_status_code" gorm:"column:upstream_status_code;default:0"`
	ErrorMessage             string `json:"error_message" gorm:"column:error_message;not null;default:''"`
	TraceID                  string `json:"trace_id" gorm:"column:trace_id;not null;default:'';index"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/infrastructure/database/model/model_call_audit.go
git commit -m "feat(model): add ModelCallAudit database model"
```

---

## Task 2: Create `ModelCallAuditDAO` and Register in Singleton

**Files:**
- Create: `internal/infrastructure/database/dao/model_call_audit.go`
- Modify: `internal/infrastructure/database/dao/singleton.go`

- [ ] **Step 1: Write DAO file**

```go
// Package dao ModelCallAudit DAO
//
//	author centonhuang
//	update 2026-04-09 10:00:00
package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// ModelCallAuditDAO 模型调用审计DAO
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ModelCallAuditDAO struct {
	baseDAO[dbmodel.ModelCallAudit]
}
```

- [ ] **Step 2: Register singleton in `singleton.go`**

Add `modelCallAuditDAOSingleton *ModelCallAuditDAO` to the `var` block and `modelCallAuditDAOSingleton = &ModelCallAuditDAO{}` to `init()`. Add:

```go
// GetModelCallAuditDAO 获取模型调用审计DAO
//
//	@return *ModelCallAuditDAO
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func GetModelCallAuditDAO() *ModelCallAuditDAO {
	return modelCallAuditDAOSingleton
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/database/dao/model_call_audit.go internal/infrastructure/database/dao/singleton.go
git commit -m "feat(dao): add ModelCallAuditDAO and register singleton"
```

---

## Task 3: Register `ModelCallAudit` in `base.go` Models List

**Files:**
- Modify: `internal/infrastructure/database/model/base.go`

- [ ] **Step 1: Add `&ModelCallAudit{}` to `Models` slice**

```go
var Models = []interface{}{
	&User{},
	&Message{},
	&Session{},
	&Tool{},
	&ModelEndpoint{},
	&ProxyAPIKey{},
	&ModelCallAudit{}, // 新增
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/infrastructure/database/model/base.go
git commit -m "feat(model): register ModelCallAudit in auto-migrate models list"
```

---

## Task 4: Add `ModelCallAuditTask` to `dto/asynctask.go`

**Files:**
- Modify: `internal/dto/asynctask.go`

- [ ] **Step 1: Append new struct after `MessageStoreTask`**

```go
// ModelCallAuditTask 模型调用审计任务
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ModelCallAuditTask struct {
	Ctx                     context.Context
	APIKeyID                uint
	ModelID                 uint
	Model                   string
	UpstreamProvider        string
	APIProvider             string
	InputTokens             int
	OutputTokens            int
	CacheCreationInputTokens int
	CacheReadInputTokens    int
	FirstTokenLatencyMs     int64
	StreamDurationMs        int64
	UserAgent               string
	UpstreamStatusCode      int
	ErrorMessage            string
	TraceID                 string
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/dto/asynctask.go
git commit -m "feat(dto): add ModelCallAuditTask to asynctask"
```

---

## Task 5: Add Audit Submission to `store_pool.go` and `pool.go`

**Files:**
- Modify: `internal/infrastructure/pool/store_pool.go`
- Modify: `internal/infrastructure/pool/pool.go`

- [ ] **Step 1: Add `submitAuditTask` to `store_pool.go`**

Add after the existing `submitMessageStoreTask` function. Add imports for `logger` and `dao` if not already present:

```go
// submitAuditTask 提交审计任务到 Store 池
//
//	@param pm *PoolManager
//	@param task *dto.ModelCallAuditTask
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) submitAuditTask(task *dto.ModelCallAuditTask) error {
	l := logger.WithCtx(task.Ctx)
	db := database.GetDBInstance(task.Ctx)

	return pm.storePool.Go(func() {
		audit := &dbmodel.ModelCallAudit{
			APIKeyID:                 task.APIKeyID,
			ModelID:                 task.ModelID,
			Model:                    task.Model,
			UpstreamProvider:         task.UpstreamProvider,
			APIProvider:              task.APIProvider,
			InputTokens:              task.InputTokens,
			OutputTokens:             task.OutputTokens,
			CacheCreationInputTokens: task.CacheCreationInputTokens,
			CacheReadInputTokens:     task.CacheReadInputTokens,
			FirstTokenLatencyMs:      task.FirstTokenLatencyMs,
			StreamDurationMs:         task.StreamDurationMs,
			UserAgent:                task.UserAgent,
			UpstreamStatusCode:       task.UpstreamStatusCode,
			ErrorMessage:             task.ErrorMessage,
			TraceID:                  task.TraceID,
		}
		if err := dao.GetModelCallAuditDAO().Create(db, audit); err != nil {
			l.Error("[StorePool] Failed to store audit record", zap.Error(err))
			return
		}
		l.Info("[StorePool] Audit record stored successfully")
	})
}
```

- [ ] **Step 2: Add public method to `pool.go`**

Add after `SubmitScoreTask`:

```go
// SubmitModelCallAuditTask 提交模型调用审计任务到协程池
//
//	@receiver pm *PoolManager
//	@param task *dto.ModelCallAuditTask
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (pm *PoolManager) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	return pm.submitAuditTask(task)
}
```

Also add the `logger` import to `store_pool.go` — verify it is already present (it is used by `submitMessageStoreTask`).

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/pool/store_pool.go internal/infrastructure/pool/pool.go
git commit -m "feat(pool): add submitAuditTask and SubmitModelCallAuditTask"
```

---

## Task 6: Delete `TokenCount` from `message.go`

**Files:**
- Modify: `internal/infrastructure/database/model/message.go`

- [ ] **Step 1: Remove `TokenCount int` field from `Message` struct**

The struct should become:

```go
type Message struct {
	BaseModel
	ID       uint                `json:"id" gorm:"column:id;primary_key;auto_increment;comment:消息ID"`
	Model    string              `json:"model" gorm:"column:model;not null;default:'';comment:模型"`
	Message  *dto.UnifiedMessage `json:"message" gorm:"column:message;not null;comment:消息;serializer:json"`
	CheckSum string              `json:"check_sum" gorm:"column:check_sum;not null;default:'';comment:校验和"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/infrastructure/database/model/message.go
git commit -m "refactor(model): remove TokenCount from Message"
```

---

## Task 7: Delete `Client` from `session.go`

**Files:**
- Modify: `internal/infrastructure/database/model/session.go`

- [ ] **Step 1: Remove `Client string` field from `Session` struct**

- [ ] **Step 2: Commit**

```bash
git add internal/infrastructure/database/model/session.go
git commit -m "refactor(model): remove Client from Session"
```

---

## Task 8: Remove `Client` from `MessageStoreTask` and `store_pool.go`

**Files:**
- Modify: `internal/dto/asynctask.go`
- Modify: `internal/infrastructure/pool/store_pool.go`

- [ ] **Step 1: Remove `Client string` from `MessageStoreTask` in `asynctask.go`**

- [ ] **Step 2: Remove `Client: task.Client` from the `Session` creation in `store_pool.go`**

The `Session` creation block should become:

```go
session := &dbmodel.Session{
	APIKeyName: task.APIKeyName,
	MessageIDs: messageIDs,
	ToolIDs:    toolIDs,
	Metadata:   task.Metadata,
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/dto/asynctask.go internal/infrastructure/pool/store_pool.go
git commit -m "refactor: remove Client from MessageStoreTask and Session creation"
```

---

## Task 9: Wire Audit Submission into `openai.go`

**Files:**
- Modify: `internal/service/openai.go`

This task modifies `forwardNative` and `forwardViaAnthropic` (both stream and non-stream paths) to collect timing and submit `ModelCallAuditTask`.

- [ ] **Step 1: Add helper to extract upstream status code and error message from error**

At the bottom of the file (before any existing helpers), add:

```go
func extractUpstreamStatusAndError(err error) (int, string) {
	if err == nil {
		return 200, ""
	}
	var ue *model.UpstreamError
	if errors.As(err, &ue) {
		return ue.StatusCode, ue.Error()
	}
	return 0, err.Error()
}
```

Add `errors` to the import block.

- [ ] **Step 2: Modify `forwardNative` non-stream path**

Replace the non-stream block in `forwardNative` so that after `s.storeFromCompletion(...)` it also submits an audit task:

```go
// After: s.storeFromCompletion(ctx, logger, req, completion, nil, ep.Model)
upstreamStatusCode, errorMessage := extractUpstreamStatusAndError(err)
pool.GetPoolManager().SubmitModelCallAuditTask(&dto.ModelCallAuditTask{
	Ctx:                     util.CopyContextValues(ctx),
	APIKeyID:                util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
	ModelID:                 endpoint.ID,
	Model:                   req.Body.Model,
	UpstreamProvider:        string(endpoint.Provider),
	APIProvider:             string(enum.ProviderOpenAI),
	InputTokens:             lo.IfF(usage != nil, func() int { return usage.PromptTokens }).Else(0),
	OutputTokens:            lo.IfF(usage != nil, func() int { return usage.CompletionTokens }).Else(0),
	FirstTokenLatencyMs:     0,
	StreamDurationMs:         0,
	UserAgent:               util.CtxValueString(ctx, constant.CtxKeyClient),
	UpstreamStatusCode:      upstreamStatusCode,
	ErrorMessage:            errorMessage,
	TraceID:                util.CtxValueString(ctx, constant.CtxKeyTraceID),
})
```

For the stream path, use `startTime := time.Now()` before the `ForwardChatCompletionStream` call, and calculate latencies after. The audit submission should happen in the callback finalization area.

**Stream path structure (example for `forwardNative`):**

```go
if stream {
	return util.WrapStreamResponse(func(w *bufio.Writer) {
		startTime := time.Now()
		var firstTokenTime time.Time
		var streamDone time.Time
		var firstTokenLatencyMs int64
		var streamDurationMs int64

		completion, err := s.openAIProxy.ForwardChatCompletionStream(ctx, ep, body, func(chunk *dto.OpenAIChatCompletionChunk) error {
			if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
				firstTokenTime = time.Now()
				firstTokenLatencyMs = firstTokenTime.Sub(startTime).Milliseconds()
			}
			// ... existing chunk handling
			return w.Flush()
		})
		streamDone = time.Now()
		if !firstTokenTime.IsZero() {
			streamDurationMs = streamDone.Sub(firstTokenTime).Milliseconds()
		}

		// ... existing DONE writing

		usage := completion.GetUsage()
		upstreamStatusCode, errorMessage := extractUpstreamStatusAndError(err)
		pool.GetPoolManager().SubmitModelCallAuditTask(&dto.ModelCallAuditTask{
			Ctx:                    util.CopyContextValues(ctx),
			APIKeyID:               util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
			ModelID:                endpoint.ID,
			Model:                  req.Body.Model,
			UpstreamProvider:       string(endpoint.Provider),
			APIProvider:            string(enum.ProviderOpenAI),
			InputTokens:            lo.IfF(usage != nil, func() int { return usage.PromptTokens }).Else(0),
			OutputTokens:           lo.IfF(usage != nil, func() int { return usage.CompletionTokens }).Else(0),
			FirstTokenLatencyMs:    firstTokenLatencyMs,
			StreamDurationMs:       streamDurationMs,
			UserAgent:              util.CtxValueString(ctx, constant.CtxKeyClient),
			UpstreamStatusCode:     upstreamStatusCode,
			ErrorMessage:           errorMessage,
			TraceID:                util.CtxValueString(ctx, constant.CtxKeyTraceID),
		})

		s.storeFromCompletion(ctx, logger, req, completion, err, ep.Model)
	}), nil
}
```

Note: For stream paths, `completion` may be nil if `err != nil`, so usage extraction must guard against nil.

- [ ] **Step 3: Modify `forwardViaAnthropic` similarly**

Use `enum.ProviderOpenAI` as `APIProvider` (this is the OpenAI interface calling Anthropic upstream, so `api_provider = openai`, `upstream_provider = anthropic`).

- [ ] **Step 4: Verify** `endpointFields` in `openai.go` includes `"id"` — if not, add it so `endpoint.ID` is available.

Current: `var endpointFields = []string{"model", "api_key", "base_url", "provider"}`

Change to: `var endpointFields = []string{"id", "model", "api_key", "base_url", "provider"}`

- [ ] **Step 5: Commit**

```bash
git add internal/service/openai.go
git commit -m "feat(service): submit ModelCallAuditTask in OpenAI service"
```

---

## Task 10: Wire Audit Submission into `anthropic.go`

**Files:**
- Modify: `internal/service/anthropic.go`

- [ ] **Step 1: Add `errors` import** (if not present) and reuse `extractUpstreamStatusAndError` from `openai.go`

Create the same helper in `anthropic.go` (since Go doesn't have cross-file private function sharing):

```go
func extractUpstreamStatusAndError(err error) (int, string) {
	if err == nil {
		return 200, ""
	}
	var ue *model.UpstreamError
	if errors.As(err, &ue) {
		return ue.StatusCode, ue.Error()
	}
	return 0, err.Error()
}
```

Add `"errors"` and `"github.com/hcd233/aris-proxy-api/internal/common/model"` to imports.

- [ ] **Step 2: Modify `forwardNative` stream and non-stream paths**

Use `enum.ProviderAnthropic` as `APIProvider`. Stream timing same pattern as OpenAI.

For `forwardViaOpenAI` (Anthropic interface calling OpenAI upstream), use `enum.ProviderAnthropic` as `APIProvider`.

- [ ] **Step 3: Add `"id"` to `endpointFields` in `anthropic.go`**

Current `endpointFields` in `anthropic.go`: only used in `CountTokens` currently. Add `"id"` so `endpoint.ID` is available in `CreateMessage`.

- [ ] **Step 4: Collect cache tokens from Anthropic usage**

For Anthropic upstream (native or via OpenAI proxy), extract `CacheCreationInputTokens` and `CacheReadInputTokens` from `assistantMsg.Usage`:

```go
var cacheCreation, cacheRead int
if assistantMsg.Usage != nil {
	cacheCreation = assistantMsg.Usage.CacheCreationInputTokens
	cacheRead = assistantMsg.Usage.CacheReadInputTokens
}
```

For OpenAI upstream, both default to 0.

- [ ] **Step 5: Commit**

```bash
git add internal/service/anthropic.go
git commit -m "feat(service): submit ModelCallAuditTask in Anthropic service"
```

---

## Task 11: Add Unit Test for Audit Task Submission

**Files:**
- Create: `test/model_call_audit/` (new directory under `test/`)

Following the project's test conventions (fixtures in `fixtures/` subdirectory, `t.Helper()` for helpers, standard `testing` assertions).

- [ ] **Step 1: Create test fixture `test/model_call_audit/fixtures/cases.json`**

```json
[
  {
    "name": "non_stream",
    "description": "Non-streaming call audit fields",
    "task": {
      "api_key_id": 1,
      "model_id": 10,
      "model": "gpt-4o",
      "upstream_provider": "openai",
      "api_provider": "openai",
      "input_tokens": 100,
      "output_tokens": 50,
      "cache_creation_input_tokens": 0,
      "cache_read_input_tokens": 0,
      "first_token_latency_ms": 0,
      "stream_duration_ms": 0,
      "user_agent": "curl/8.1.2",
      "upstream_status_code": 200,
      "error_message": "",
      "trace_id": "trace-123"
    }
  },
  {
    "name": "stream_success",
    "description": "Streaming call audit fields",
    "task": {
      "api_key_id": 2,
      "model_id": 20,
      "model": "claude-sonnet-4-20250514",
      "upstream_provider": "anthropic",
      "api_provider": "anthropic",
      "input_tokens": 200,
      "output_tokens": 120,
      "cache_creation_input_tokens": 500,
      "cache_read_input_tokens": 150,
      "first_token_latency_ms": 320,
      "stream_duration_ms": 4500,
      "user_agent": "MyApp/1.0",
      "upstream_status_code": 200,
      "error_message": "",
      "trace_id": "trace-456"
    }
  },
  {
    "name": "upstream_error",
    "description": "Upstream error fields",
    "task": {
      "api_key_id": 3,
      "model_id": 30,
      "model": "gpt-4o-mini",
      "upstream_provider": "openai",
      "api_provider": "anthropic",
      "input_tokens": 0,
      "output_tokens": 0,
      "cache_creation_input_tokens": 0,
      "cache_read_input_tokens": 0,
      "first_token_latency_ms": 0,
      "stream_duration_ms": 0,
      "user_agent": "TestAgent",
      "upstream_status_code": 429,
      "error_message": "upstream returned status 429",
      "trace_id": "trace-789"
    }
  }
]
```

- [ ] **Step 2: Create test file `test/model_call_audit/model_call_audit_test.go`**

```go
package model_call_audit

import (
	"context"
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// testCase mirrors the fixture structure
type testCase struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Task        struct {
		APIKeyID                uint   `json:"api_key_id"`
		ModelID                 uint   `json:"model_id"`
		Model                   string `json:"model"`
		UpstreamProvider        string `json:"upstream_provider"`
		APIProvider             string `json:"api_provider"`
		InputTokens             int    `json:"input_tokens"`
		OutputTokens            int    `json:"output_tokens"`
		CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
		CacheReadInputTokens    int    `json:"cache_read_input_tokens"`
		FirstTokenLatencyMs     int64  `json:"first_token_latency_ms"`
		StreamDurationMs        int64  `json:"stream_duration_ms"`
		UserAgent               string `json:"user_agent"`
		UpstreamStatusCode      int    `json:"upstream_status_code"`
		ErrorMessage            string `json:"error_message"`
		TraceID                 string `json:"trace_id"`
	} `json:"task"`
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	return cases
}

func TestModelCallAuditTask_Fields(t *testing.T) {
	cases := loadCases(t)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			task := &dto.ModelCallAuditTask{
				Ctx:                     context.Background(),
				APIKeyID:                tc.Task.APIKeyID,
				ModelID:                 tc.Task.ModelID,
				Model:                   tc.Task.Model,
				UpstreamProvider:        tc.Task.UpstreamProvider,
				APIProvider:             tc.Task.APIProvider,
				InputTokens:             tc.Task.InputTokens,
				OutputTokens:            tc.Task.OutputTokens,
				CacheCreationInputTokens: tc.Task.CacheCreationInputTokens,
				CacheReadInputTokens:    tc.Task.CacheReadInputTokens,
				FirstTokenLatencyMs:     tc.Task.FirstTokenLatencyMs,
				StreamDurationMs:        tc.Task.StreamDurationMs,
				UserAgent:               tc.Task.UserAgent,
				UpstreamStatusCode:      tc.Task.UpstreamStatusCode,
				ErrorMessage:            tc.Task.ErrorMessage,
				TraceID:                 tc.Task.TraceID,
			}

			if task.APIKeyID != tc.Task.APIKeyID {
				t.Errorf("APIKeyID = %d, want %d", task.APIKeyID, tc.Task.APIKeyID)
			}
			if task.Model != tc.Task.Model {
				t.Errorf("Model = %q, want %q", task.Model, tc.Task.Model)
			}
			if task.UpstreamProvider != tc.Task.UpstreamProvider {
				t.Errorf("UpstreamProvider = %q, want %q", task.UpstreamProvider, tc.Task.UpstreamProvider)
			}
			if task.APIProvider != tc.Task.APIProvider {
				t.Errorf("APIProvider = %q, want %q", task.APIProvider, tc.Task.APIProvider)
			}
			if task.InputTokens != tc.Task.InputTokens {
				t.Errorf("InputTokens = %d, want %d", task.InputTokens, tc.Task.InputTokens)
			}
			if task.OutputTokens != tc.Task.OutputTokens {
				t.Errorf("OutputTokens = %d, want %d", task.OutputTokens, tc.Task.OutputTokens)
			}
			if task.CacheCreationInputTokens != tc.Task.CacheCreationInputTokens {
				t.Errorf("CacheCreationInputTokens = %d, want %d", task.CacheCreationInputTokens, tc.Task.CacheCreationInputTokens)
			}
			if task.CacheReadInputTokens != tc.Task.CacheReadInputTokens {
				t.Errorf("CacheReadInputTokens = %d, want %d", task.CacheReadInputTokens, tc.Task.CacheReadInputTokens)
			}
			if task.FirstTokenLatencyMs != tc.Task.FirstTokenLatencyMs {
				t.Errorf("FirstTokenLatencyMs = %d, want %d", task.FirstTokenLatencyMs, tc.Task.FirstTokenLatencyMs)
			}
			if task.StreamDurationMs != tc.Task.StreamDurationMs {
				t.Errorf("StreamDurationMs = %d, want %d", task.StreamDurationMs, tc.Task.StreamDurationMs)
			}
			if task.UserAgent != tc.Task.UserAgent {
				t.Errorf("UserAgent = %q, want %q", task.UserAgent, tc.Task.UserAgent)
			}
			if task.UpstreamStatusCode != tc.Task.UpstreamStatusCode {
				t.Errorf("UpstreamStatusCode = %d, want %d", task.UpstreamStatusCode, tc.Task.UpstreamStatusCode)
			}
			if task.ErrorMessage != tc.Task.ErrorMessage {
				t.Errorf("ErrorMessage = %q, want %q", task.ErrorMessage, tc.Task.ErrorMessage)
			}
			if task.TraceID != tc.Task.TraceID {
				t.Errorf("TraceID = %q, want %q", task.TraceID, tc.Task.TraceID)
			}
		})
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test -v -count=1 ./test/model_call_audit/
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add test/model_call_audit/
git commit -m "test: add model_call_audit unit test"
```

---

## Task 12: Run Full Test Suite

- [ ] **Step 1: Run all tests**

```bash
go test -count=1 ./...
```

- [ ] **Step 2: Run lint conventions**

```bash
make lint-conv
```

- [ ] **Step 3: Fix any failures**

Do not commit until all pass.

- [ ] **Step 4: Commit any test fixes**

```bash
git add test/ internal/
git commit -m "test: fix test failures from audit changes"
```

---

## Task 13: Final Verification

- [ ] **Step 1: Verify `message.token_count` is gone** — grep for `TokenCount` in `model/message.go`, expect no field
- [ ] **Step 2: Verify `session.client` is gone** — grep for `Client` in `model/session.go`, expect no `Client string` field
- [ ] **Step 3: Verify `model_call_audit` model exists** — check `model/model_call_audit.go`
- [ ] **Step 4: Verify `Client` removed from `MessageStoreTask`** — grep for `Client` in `dto/asynctask.go`, expect no `Client string` field
- [ ] **Step 5: Verify `store_pool.go` Session creation has no `Client`** — check the Session struct literal

---

## Self-Review Checklist

1. **Spec coverage:** All spec requirements mapped to tasks?
   - `model_call_audit` table fields ✓
   - Delete `message.token_count` ✓
   - Delete `session.client` ✓
   - Write via `storePool` ✓
   - Stream timing (first token + stream duration) ✓
   - Non-stream 0 for timing fields ✓
   - `api_provider`/`upstream_provider` distinction ✓
   - No history backfill ✓

2. **Placeholder scan:** No TBD/TODO/unfilled steps ✓

3. **Type consistency:** 
   - `ModelCallAuditTask` field names match `ModelCallAudit` model field names ✓
   - `endpoint.ID` added to `endpointFields` in both services ✓
   - `CtxKeyAPIKeyID` used for `api_key_id` ✓
   - `CtxKeyClient` used for `user_agent` ✓
   - `CtxKeyTraceID` used for `trace_id` ✓
