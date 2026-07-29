# 模型能力（ModelCapabilities）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 models 管理新增模型能力（输入模态：文本/图片）的配置、展示，并让 OpenCode / Pi 导出脚本按能力生成对应配置项。

**Architecture:** DB 新增单个 `capabilities` text 列（`serializer:json`，默认 `["text"]`）→ 聚合 `Model` 加 `[]enum.InputModality` 并强制「非空且含 text」→ 应用层/DTO/Handler 透传 → 前端表单两个开关 + 列表新「能力」列徽标 → 导出脚本：OpenCode 写 `modalities.input`（含 image 加 `attachment: true`）、Pi 写 `input` 数组。Claude Code / Codex 配置 schema 无 per-model 模态项，不动。

**Tech Stack:** Go 1.25 + GORM(serializer:json) + huma v2 + ierr + samber/lo；Next.js 静态导出 + Tailwind + shadcn/ui。

**Spec:** `docs/superpowers/specs/2026-07-29-model-capabilities-design.md`

## Global Constraints

- 工作区：worktree `.worktrees/feature-model-capabilities-2026-07-29`，分支 `feature/model-capabilities-2026-07-29`。**所有命令在该 worktree 根目录执行。**
- 能力集合校验：**非空、必须包含 `text`、成员必须属于已知枚举**（当前 `text` / `image`）。
- 测试契约：仅用标准库 `testing` + `bytedance/sonic`（禁止 encoding/json / testify / time.Sleep 同步）；单测放 `test/unit/<topic>/`，E2E 放 `test/e2e/<topic>/` 且 env-gated（缺 env 时 `t.Skip`）。
- 业务错误走 `internal/common/ierr`；日志 `logger.WithCtx(ctx)` 前缀 `[PascalCaseModule]`。
- DTO 遵守 huma Body 包装规范（参考 `internal/dto/model.go` 现有结构）；DTO 禁 import dbmodel。
- 函数式子：优先 `samber/lo`（`lo.Contains` / `lo.Every`）。
- i18n 三语（zh / en / ja）；i18n 键在 `models.*` 命名空间。
- `internal/web/dist` 被 git 追踪——验证构建后 `git status` 不应多出它的改动。

## 实现期对 spec 的修订（已确认，写入任务 7 的 spec 回写）

1. **枚举落位**：`InputModality` 放在 `internal/common/enum/`（非 `vo` 包），遵循「业务包禁止本地 const 块，用 common/enum」契约；类型为 `= string` 别名（同 `enum.ModalityType` 先例），免去各层 `[]string ↔ []InputModality` 转换样板。
2. **列类型 text 而非 jsonb**：生产库实测所有 `serializer:json` 列均为 `text`（`messages.message`、`sessions.message_ids/models/metadata`）。jsonb 需额外 `type:jsonb` 且对 map-update 手动序列化的字符串写入有 cast 风险，text 列与项目惯例一致。
3. **GORM 陷阱（关键）**：`Updates(map[string]any)` **不经过 field serializer**（raw 值进 SQL 参数）。因此 `modelRepository.Update` 的 updates map 中 capabilities 必须**手动 `sonic.Marshal`** 为 JSON 字符串；struct 写入（Create）路径 serializer 会自动生效，无需手动处理。（此现象同样存在于 `session_repository.updateSession` 的 `message_ids` map-update，属存量隐患、不在本次范围。）

---

### Task 1: 存储层 — DB 字段、列常量、字段清单

**Files:**
- Modify: `internal/infrastructure/database/model/model.go`
- Modify: `internal/common/constant/sql.go`
- Modify: `internal/common/constant/model.go`
- Test: `test/unit/model_repository/model_update_test.go`

**Interfaces:**
- Produces: DB 模型字段 `dbmodel.Model.Capabilities []string`；`constant.FieldModelCapabilities = "capabilities"`（已含于 `constant.ModelRepoFieldsFull`）。

- [ ] **Step 1: 扩展列常量回归测试（先红）**

在 `test/unit/model_repository/model_update_test.go` 的 `checks` 列表末尾追加：

```go
		{goField: "Capabilities", constant: constant.FieldModelCapabilities},
```

- [ ] **Step 2: 运行测试确认编译失败**

Run: `go test -count=1 ./test/unit/model_repository/ 2>&1 | head -5`
Expected: FAIL — `undefined: constant.FieldModelCapabilities`

- [ ] **Step 3: 加列常量 + repo 字段清单**

`internal/common/constant/sql.go`：
- 在 `FieldModelModelName` 等同组常量附近与现有 model 字段常量合并区域补：

```go
	FieldModelCapabilities = "capabilities"
```

（先 grep 定位 `FieldModelModelName` / `FieldModelContextLength` 定义处并插入相邻行。）
- `ModelRepoFieldsFull` 追加 `FieldModelCapabilities`（插入 `FieldModelMaxOutputTokens` 之后）：

```go
	ModelRepoFieldsFull = []string{FieldID, FieldAlias, FieldModel, FieldEndpointID, FieldEnabled, FieldModelContextLength, FieldModelMaxOutputTokens, FieldModelCapabilities, FieldCreatedAt, FieldUpdatedAt}
```

`internal/common/constant/model.go` 追加（写前读全文件确认现有结构）：

```go
// DefaultModelCapabilities 模型默认能力（仅文本输入）
var DefaultModelCapabilities = []string{"text"}
```

- [ ] **Step 4: DB 模型加字段**

`internal/infrastructure/database/model/model.go` 的 `Model` struct，`MaxOutputTokens` 之后：

```go
	Capabilities    []string `json:"capabilities" gorm:"column:capabilities;not null;default:'[\"text\"]';comment:模型能力（输入模态集合，如 text/image）;serializer:json"`
```

> 注意：Go 反引号 struct tag 内 `"` 原样书写，不需要 `\"` 转义。`serializer:json` 决定列 DDL 为 `text`（与 `sessions.models` 一致）。

- [ ] **Step 5: 单元测试转绿**

Run: `go test -count=1 ./test/unit/model_repository/`
Expected: PASS

- [ ] **Step 6: 本地迁移验证（临时 PG 容器）** —— ⚠️ 执行环境无 docker CLI / 无本地 PG，**顺延到部署期**：`script/deploy-k8s.sh` 的 K8s `db-migrate` Job 会先于新 Deployment 跑 `database migrate`，部署后 E2E（Task 4）验证列行为与默认值。以下为参考命令：

```bash
docker run -d --name aris-cap-pg -e POSTGRES_USER=captest -e POSTGRES_PASSWORD=captest -e POSTGRES_DB=captestdb -p 55432:5432 postgres:16-alpine
# 等容器 ready
docker exec aris-cap-pg pg_isready -U captest
# 在项目根目录执行迁移（env 键名对齐 env/api.env.template）
POSTGRES_USER=captest POSTGRES_PASSWORD=captest POSTGRES_DATABASE=captestdb POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=55432 POSTGRES_SSLMODE=disable \
  go run ./cmd/server database migrate
# 断言：capabilities 列为 text、not null、默认 '["text"]'
docker exec aris-cap-pg psql -U captest -d captestdb -c "SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name='models' AND column_name='capabilities';"
# 断言：不带 capabilities 的插入 GET 到默认值
docker exec aris-cap-pg psql -U captest -d captestdb -c "INSERT INTO models (alias, model, endpoint_id) VALUES ('cap-check', 'cap-check', 1) RETURNING capabilities;"
docker exec aris-cap-pg psql -U captest -d captestdb -c "DELETE FROM models WHERE alias='cap-check';"
# 清理
docker rm -f aris-cap-pg
```

Expected: 列类型 `text`、`is_nullable=NO`、`column_default='["text"]'`；INSERT 返回 `["text"]`。

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/database/model/model.go internal/common/constant/sql.go internal/common/constant/model.go test/unit/model_repository/model_update_test.go
git commit -m "feat(model): add capabilities json column with text/image default"
```

---

### Task 2: 领域层 — InputModality 枚举 + 聚合校验

**Files:**
- Create: `internal/common/enum/input_modality.go`
- Modify: `internal/domain/llmproxy/aggregate/model.go`
- Modify（构造器签名扩散的调用点）:
  - `internal/infrastructure/repository/endpoint_repository.go`（`toModelAggregate`）
  - `internal/application/model/command/create_model.go`
  - `test/unit/endpoint_resolver/endpoint_resolver_test.go`（5 处调用）
  - `test/unit/llmproxy_usecase/openai_forward_test.go`（1 处）
  - `test/unit/llmproxy_usecase/anthropic_forward_test.go`（1 处）
- Test: `test/unit/domain_llmproxy/model_capabilities_test.go`（新建，package `domain_llmproxy`）

**Interfaces:**
- Produces:
  - `enum.InputModality = string`；`enum.InputModalityText = "text"`、`enum.InputModalityImage = "image"`；`enum.InputModalities` 已知集合
  - `aggregate.CreateModel(id uint, alias vo.EndpointAlias, model string, endpointID uint, enabled bool, contextLength, maxOutputTokens int, capabilities []enum.InputModality) (*Model, error)`
  - `Model.Capabilities() []enum.InputModality`
  - `Model.Update(alias *vo.EndpointAlias, model *string, endpointID *uint, enabled *bool, contextLength, maxOutputTokens *int, capabilities *[]enum.InputModality) error`（**新增 error 返回值**）

- [ ] **Step 1: 写聚合校验单测（先红）**

新建 `test/unit/domain_llmproxy/model_capabilities_test.go`：

```go
package domain_llmproxy

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

func baseCaps() []enum.InputModality {
	return []enum.InputModality{enum.InputModalityText}
}

// 合法集合：text / text+image 均通过
func TestCreateModel_Capabilities_Valid(t *testing.T) {
	t.Parallel()
	for _, caps := range [][]enum.InputModality{
		baseCaps(),
		{enum.InputModalityText, enum.InputModalityImage},
	} {
		m, err := aggregate.CreateModel(1, vo.EndpointAlias("a"), "m", 1, true, 128000, 64000, caps)
		if err != nil {
			t.Fatalf("valid capabilities %v should pass: %v", caps, err)
		}
		if got := m.Capabilities(); len(got) != len(caps) {
			t.Fatalf("capabilities round trip mismatch: got %v want %v", got, caps)
		}
	}
}

// 非法集合：空 / 不含 text / 未知成员 均报错
func TestCreateModel_Capabilities_Invalid(t *testing.T) {
	t.Parallel()
	for _, caps := range [][]enum.InputModality{
		{},
		{enum.InputModalityImage},
		{enum.InputModalityText, "blob"},
	} {
		if _, err := aggregate.CreateModel(1, vo.EndpointAlias("a"), "m", 1, true, 128000, 64000, caps); err == nil {
			t.Fatalf("invalid capabilities %v should fail", caps)
		}
	}
}

// Update：nil 不变更，合法值整体替换，非法值报错
func TestModelUpdate_Capabilities(t *testing.T) {
	t.Parallel()
	m, err := aggregate.CreateModel(1, vo.EndpointAlias("a"), "m", 1, true, 128000, 64000, baseCaps())
	if err != nil {
		t.Fatalf("CreateModel failed: %v", err)
	}
	if uerr := m.Update(nil, nil, nil, nil, nil, nil, nil); uerr != nil {
		t.Fatalf("nil capabilities update should be no-op: %v", uerr)
	}
	if got := m.Capabilities(); len(got) != 1 || got[0] != enum.InputModalityText {
		t.Fatalf("capabilities should stay text-only, got %v", got)
	}
	next := []enum.InputModality{enum.InputModalityText, enum.InputModalityImage}
	if uerr := m.Update(nil, nil, nil, nil, nil, nil, &next); uerr != nil {
		t.Fatalf("valid capabilities update failed: %v", uerr)
	}
	if got := m.Capabilities(); len(got) != 2 {
		t.Fatalf("capabilities should be replaced, got %v", got)
	}
	bad := []enum.InputModality{enum.InputModalityImage}
	if uerr := m.Update(nil, nil, nil, nil, nil, nil, &bad); uerr == nil {
		t.Fatal("capabilities without text must be rejected")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/domain_llmproxy/ 2>&1 | head -8`
Expected: FAIL — `not enough arguments in call to aggregate.CreateModel` / `Model.Capabilities undefined`

- [ ] **Step 3: 新建 enum**

`internal/common/enum/input_modality.go`：

```go
package enum

// InputModality 模型支持的输入模态（模型能力集合成员）
//
//	@author centonhuang
//	@update 2026-07-29 18:00:00
type InputModality = string

const (

	// InputModalityText 文本输入
	//
	//	@author centonhuang
	//	@update 2026-07-29 18:00:00
	InputModalityText InputModality = "text"

	// InputModalityImage 图片输入
	//
	//	@author centonhuang
	//	@update 2026-07-29 18:00:00
	InputModalityImage InputModality = "image"
)

// InputModalities 全部已知输入模态（新增模态时仅在此扩展 + 前端加开关）
//
//	@author centonhuang
//	@update 2026-07-29 18:00:00
var InputModalities = []InputModality{InputModalityText, InputModalityImage}
```

- [ ] **Step 4: 聚合加字段 + 校验 + Update 返回 error**

`internal/domain/llmproxy/aggregate/model.go`：

- import 追加 `"github.com/hcd233/aris-proxy-api/internal/common/enum"` 和 `"github.com/samber/lo"`。
- struct 加字段：

```go
	capabilities    []enum.InputModality
```

- `CreateModel` 签名末尾加 `capabilities []enum.InputModality`；零值校验块之后追加：

```go
	if err := validateCapabilities(capabilities); err != nil {
		return nil, err
	}
```

并在赋值 struct 中加 `capabilities: capabilities`。

- 新增私有校验函数（文件末尾）：

```go
// validateCapabilities 校验模型能力集合：非空、必须含 text、成员必须合法
func validateCapabilities(capabilities []enum.InputModality) error {
	if len(capabilities) == 0 {
		return ierr.New(ierr.ErrValidation, "model capabilities cannot be empty")
	}
	if !lo.Contains(capabilities, enum.InputModalityText) {
		return ierr.New(ierr.ErrValidation, "model capabilities must contain text")
	}
	if !lo.Every(enum.InputModalities, capabilities) {
		return ierr.New(ierr.ErrValidation, "model capabilities contain invalid input modality")
	}
	return nil
}
```

- getter：

```go
func (m *Model) Capabilities() []enum.InputModality { return m.capabilities }
```

- `Update` 改签名 `... capabilities *[]enum.InputModality) error`，末尾追加：

```go
	if capabilities != nil {
		if err := validateCapabilities(*capabilities); err != nil {
			return err
		}
		m.capabilities = *capabilities
	}
	return nil
```

（原有末尾隐性 `}` 前返回 nil。）

- [ ] **Step 5: 签名扩散点跟进**

- `internal/infrastructure/repository/endpoint_repository.go` `toModelAggregate`：

```go
	model, err := aggregate.CreateModel(m.ID, vo.EndpointAlias(m.Alias), m.ModelName, m.EndpointID, m.Enabled, m.ContextLength, m.MaxOutputTokens, m.Capabilities)
```

- `internal/application/model/command/create_model.go` 第 51 行调用点：先补默认值逻辑（见 Task 3 Step 2 完整代码，这里只需让编译过：传 `capabilities` 变量，本任务在 Task 3 先行合入；若拆分执行，此调用点改为传 `cmd.Capabilities` + import enum）。
- 三处测试文件：每个 `aggregate.CreateModel(...)` 调用实参末尾追加 `, []enum.InputModality{enum.InputModalityText}`（需为每个测试文件补 `enum` import）。

- [ ] **Step 6: 全量编译 + 单测转绿**

Run: `go build ./cmd/server && go test -count=1 ./test/unit/domain_llmproxy/ ./test/unit/endpoint_resolver/ ./test/unit/llmproxy_usecase/ ./test/unit/model_repository/`
Expected: PASS（若 create_model 调用点暂用 `nil` 兜底导致 domain 校验失败，说明应按 Task 3 Step 2 提前合入默认逻辑：空集合转 `["text"]`——一致性优先，直接提前做）

- [ ] **Step 7: Commit**

```bash
git add internal/common/enum/input_modality.go internal/domain/llmproxy/aggregate/model.go internal/infrastructure/repository/endpoint_repository.go internal/application/model/command/create_model.go test/unit/
git commit -m "feat(model): add capabilities to model aggregate with validation"
```

---

### Task 3: 应用层透传 — port / command / query / handler / repository

**Files:**
- Modify: `internal/application/model/port/handler.go`
- Modify: `internal/application/model/command/create_model.go`
- Modify: `internal/application/model/command/update_model.go`
- Modify: `internal/application/model/query/list_models.go`
- Modify: `internal/dto/model.go`
- Modify: `internal/handler/model.go`
- Modify: `internal/infrastructure/repository/endpoint_repository.go`

**Interfaces:**
- Consumes: Task 2 的 `enum.InputModality`、聚合 getter/校验。
- Produces:
  - `port.CreateModelCommand.Capabilities []enum.InputModality`
  - `port.UpdateModelCommand.Capabilities *[]enum.InputModality`
  - `port.ModelView.Capabilities []enum.InputModality`
  - DTO：`CreateModelReqBody.Capabilities []enum.InputModality`、`UpdateModelReqBody.Capabilities *[]enum.InputModality`、`ModelItem.Capabilities []enum.InputModality`（JSON 键统一 `"capabilities"`）

- [ ] **Step 1: DTO**

`internal/dto/model.go` import 加 enum；三处：

```go
	// CreateModelReqBody 追加
	Capabilities    []enum.InputModality `json:"capabilities,omitempty" doc:"模型能力（输入模态集合；合法值 text/image；必须包含 text；缺省为 [text]）"`

	// UpdateModelReqBody 追加
	Capabilities    *[]enum.InputModality `json:"capabilities,omitempty" doc:"模型能力（输入模态集合；合法值 text/image；必须包含 text）"`

	// ModelItem 追加（MaxOutputTokens 之后）
	Capabilities    []enum.InputModality `json:"capabilities" doc:"模型能力（输入模态集合）"`
```

- [ ] **Step 2: port + commands**

`internal/application/model/port/handler.go`：import enum；三个 struct 各加字段（对齐 Step 1 类型）。

`command/create_model.go`：默认值兜底（在 maxOutputTokens 兜底块后）：

```go
	capabilities := cmd.Capabilities
	if len(capabilities) == 0 {
		capabilities = constant.DefaultModelCapabilities
	}
```

`aggregate.CreateModel(...)` 末尾参数改为 `capabilities`。

`command/update_model.go`：`Handle` 中调用改为（必须捕获 error）：

```go
	if uerr := m.Update(aliasPtr, cmd.ModelName, cmd.EndpointID, cmd.Enabled, cmd.ContextLength, cmd.MaxOutputTokens, cmd.Capabilities); uerr != nil {
		return uerr
	}
```

- [ ] **Step 3: query / handler**

`query/list_models.go` 的 `ModelView{...}` 字面量加：

```go
			Capabilities:    m.Capabilities(),
```

`handler/model.go`：
- `HandleCreateModel`：command 字面量加 `Capabilities: req.Body.Capabilities,`
- `HandleUpdateModel`：command 字面量加 `Capabilities: req.Body.Capabilities,`
- `HandleListModels` 的 `dto.ModelItem{...}` 加 `Capabilities: v.Capabilities,`

- [ ] **Step 4: repository 转换与更新**

`internal/infrastructure/repository/endpoint_repository.go`：

- `toModelDBModel` 加 `Capabilities: lo.Ternary`…不需要，直接：

```go
		Capabilities:    m.Capabilities(),
```

（需补 `sonic` import 给下一步用）
- `Update` 整段替换为（关键：map-update 不经 serializer，capabilities 必须手动 marshal 成 JSON 字符串再进 map；其余字段原样保留）：

```go
// Update 更新模型（仅更新非零值字段）
func (r *modelRepository) Update(ctx context.Context, m *aggregate.Model) error {
	db := r.db.WithContext(ctx)
	capJSON, _ := sonic.Marshal(m.Capabilities()) //nolint:errcheck // []string 序列化不会失败；值已经聚合校验
	updates := map[string]any{
		constant.FieldModelAlias:           m.Alias().String(),
		constant.FieldModelModelName:       m.ModelName(),
		constant.FieldModelEndpointID:      m.EndpointID(),
		constant.FieldModelEnabled:         m.Enabled(),
		constant.FieldModelContextLength:   m.ContextLength(),
		constant.FieldModelMaxOutputTokens: m.MaxOutputTokens(),
		constant.FieldModelCapabilities:    string(capJSON),
	}
	if err := db.Model(&dbmodel.Model{}).Where(constant.WhereIDEquals, m.AggregateID()).Updates(updates).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update model")
	}
	return nil
}
```

（`sonic` 需补 import：生产代码统一用 `github.com/bytedance/sonic`，禁 `encoding/json`。）

- [ ] **Step 5: 编译 + 全量单测 + lint**

Run: `go build ./cmd/server && go test -count=1 ./test/unit/... && make lint 2>&1 | tail -5`
Expected: build OK；unit tests PASS；lint 无新增违规

- [ ] **Step 6: Commit**

```bash
git add internal/ && git commit -m "feat(model): pass capabilities through dto/application/handler/repository"
```

---

### Task 4: E2E 用例 — create / list / update 全链路

**Files:**
- Create: `test/e2e/model_capabilities/model_capabilities_test.go`

**Interfaces:**
- Consumes: Task 3 的 API 行为。huma **unwrap Body**：响应 JSON 直接是 rsp 结构（无 `data` 外层）。

E2E 需要 admin JWT。用例（三个 test，全部先列 endpoint 取 `endpointID`）：

1. `TestModelCapabilities_RoundTrip`：create（capabilities=[text,image]，alias 带 UnixNano 后缀防冲突）→ `GET /model/list?query=<alias>` 断言含 image → `PATCH /model?id=<id>` body `{capabilities:["text"]}` → 再 list 断言只剩 text → defer DELETE 清理
2. `TestModelCapabilities_CreateDefaultsToText`：create 不传 capabilities → list 断言 == `["text"]` → 清理
3. `TestModelCapabilities_CreateRejectsInvalid`：body `[image]`、body `[text, blob]` 两个子用例都断言 `error != nil`（handler 把业务错误写进 `{"error":...}` 且 HTTP 200）

骨架照抄 `test/e2e/session_share/session_share_test.go`（`mustE2EEnv` 只取 `BASE_URL` + `JWT_TOKEN`；`newE2EClient`；`sonic.Marshal` 请求体；`constant.HTTPHeaderAuthorization` + `constant.HTTPAuthBearerPrefix`；错误时打 `X-Trace-Id` header）。响应解码结构（贴合 huma unwrap）：

```go
type bizErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type endpointListItem struct {
	ID uint `json:"id"`
}

type modelListItem struct {
	ID           uint     `json:"id"`
	Alias        string   `json:"alias"`
	Capabilities []string `json:"capabilities"`
}
```

- [ ] **Step 1: 写全文件**（每个用例独立 create+delete，互不依赖；alias 用 UnixNano 后缀隔离）

```go
// Package model_capabilities 验证模型能力（capabilities）配置的管理 API 全链路行为。
//
// 需求背景（feature/model-capabilities）：
//   - models 表新增 capabilities 列（输入模态集合，serializer:json，默认 ["text"]）；
//   - 管理 API create / list / update 需对 capabilities 做全链路 round-trip；
//   - 非法集合（空 / 不含 text / 未知成员）必须被业务层拒绝。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT（必填，调用 model 管理接口需 admin 权限）
package model_capabilities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 30 * time.Second

type bizError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type listEndpointsRsp struct {
	Endpoints []struct {
		ID uint `json:"id"`
	} `json:"endpoints"`
	Error *bizError `json:"error,omitempty"`
}

type modelItem struct {
	ID           uint     `json:"id"`
	Alias        string   `json:"alias"`
	Capabilities []string `json:"capabilities"`
}

type listModelsRsp struct {
	Models []modelItem `json:"models"`
	Error  *bizError   `json:"error,omitempty"`
}

type commandRsp struct {
	Error *bizError `json:"error,omitempty"`
}

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL and JWT_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), jwtToken
}

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

// doJSON 发出请求并返回状态码、TraceID 与原始响应体。
func doJSON(t *testing.T, client *http.Client, method, url, jwtToken string, reqBody map[string]any) (int, string, []byte) {
	t.Helper()
	var reader io.Reader
	if reqBody != nil {
		body, err := sonic.Marshal(reqBody)
		if err != nil {
			t.Fatalf("marshal request body failed: %v", err)
		}
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, reader)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+jwtToken)
	if reqBody != nil {
		req.Header.Set("Content-Type", constant.HTTPContentTypeJSON)
	}
	httpResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s failed: %v", method, url, err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return httpResp.StatusCode, httpResp.Header.Get(constant.HTTPHeaderTraceID), bodyBytes
}

// pickEndpointID 选一个可用 endpoint 挂载模型。
func pickEndpointID(t *testing.T, baseURL, jwtToken string, client *http.Client) uint {
	t.Helper()
	status, traceID, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/endpoint/list?page=1&pageSize=1", jwtToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list endpoints status=%d traceID=%s body=%s", status, traceID, string(body))
	}
	var rsp listEndpointsRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal endpoints failed: %v body=%s", err, string(body))
	}
	if rsp.Error != nil {
		t.Fatalf("list endpoints error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}
	if len(rsp.Endpoints) == 0 {
		t.Skip("no endpoint available to attach model")
	}
	return rsp.Endpoints[0].ID
}

// createModel 创建模型；capabilities 传 nil 表示不显式发送该字段。
func createModel(t *testing.T, baseURL, jwtToken string, client *http.Client, endpointID uint, alias string, capabilities []string) (*commandRsp, string) {
	t.Helper()
	body := map[string]any{
		"alias":           alias,
		"modelName":       "e2e-upstream-model",
		"endpointID":      endpointID,
		"contextLength":   128000,
		"maxOutputTokens": 64000,
	}
	if capabilities != nil {
		body["capabilities"] = capabilities
	}
	status, traceID, raw := doJSON(t, client, http.MethodPost, baseURL+"/api/v1/model", jwtToken, body)
	if status != http.StatusOK {
		t.Fatalf("create model alias=%s status=%d traceID=%s body=%s", alias, status, traceID, string(raw))
	}
	var rsp commandRsp
	if err := sonic.Unmarshal(raw, &rsp); err != nil {
		t.Fatalf("unmarshal create rsp failed: %v body=%s", err, string(raw))
	}
	return &rsp, traceID
}

// getModelByAlias 按别名查模型；未命中返回 nil。
func getModelByAlias(t *testing.T, baseURL, jwtToken string, client *http.Client, alias string) *modelItem {
	t.Helper()
	status, traceID, raw := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/model/list?page=1&pageSize=50&query="+alias, jwtToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list models status=%d traceID=%s body=%s", status, traceID, string(raw))
	}
	var rsp listModelsRsp
	if err := sonic.Unmarshal(raw, &rsp); err != nil {
		t.Fatalf("unmarshal models failed: %v body=%s", err, string(raw))
	}
	for i := range rsp.Models {
		if rsp.Models[i].Alias == alias {
			return &rsp.Models[i]
		}
	}
	return nil
}

func updateCapabilities(t *testing.T, baseURL, jwtToken string, client *http.Client, modelID uint, capabilities []string) {
	t.Helper()
	status, traceID, raw := doJSON(t, client, http.MethodPatch, fmt.Sprintf("%s/api/v1/model?id=%d", baseURL, modelID), jwtToken, map[string]any{"capabilities": capabilities})
	if status != http.StatusOK {
		t.Fatalf("update model status=%d traceID=%s body=%s", status, traceID, string(raw))
	}
	var rsp commandRsp
	if err := sonic.Unmarshal(raw, &rsp); err != nil {
		t.Fatalf("unmarshal update rsp failed: %v body=%s", err, string(raw))
	}
	if rsp.Error != nil {
		t.Fatalf("update model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}
}

func cleanupModel(t *testing.T, baseURL, jwtToken string, client *http.Client, modelID *uint) {
	t.Helper()
	if modelID == nil || *modelID == 0 {
		return
	}
	status, traceID, raw := doJSON(t, client, http.MethodDelete, fmt.Sprintf("%s/api/v1/model?id=%d", baseURL, *modelID), jwtToken, nil)
	if status != http.StatusOK {
		t.Logf("cleanup delete model id=%d failed: status=%d traceID=%s body=%s", *modelID, status, traceID, string(raw))
	}
}

// TestModelCapabilities_RoundTrip 验证 create → list → update → list 的 capabilities 全链路读写一致。
func TestModelCapabilities_RoundTrip(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	alias := fmt.Sprintf("e2e-cap-roundtrip-%d", time.Now().UnixNano())
	var modelID uint
	defer func() { cleanupModel(t, baseURL, jwtToken, client, &modelID) }()

	rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias, []string{"text", "image"})
	if rsp.Error != nil {
		t.Fatalf("create model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}

	item := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if item == nil {
		t.Fatalf("created model alias=%s not found in list", alias)
	}
	modelID = item.ID
	if !slices.Equal(item.Capabilities, []string{"text", "image"}) {
		t.Fatalf("list after create: capabilities=%v want [text image]", item.Capabilities)
	}

	updateCapabilities(t, baseURL, jwtToken, client, modelID, []string{"text"})

	updated := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if updated == nil || !slices.Equal(updated.Capabilities, []string{"text"}) {
		t.Fatalf("list after update: capabilities=%v want [text]", updated)
	}
}

// TestModelCapabilities_CreateDefaultsToText 验证不传 capabilities 时服务端兜底为 ["text"]。
func TestModelCapabilities_CreateDefaultsToText(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	alias := fmt.Sprintf("e2e-cap-default-%d", time.Now().UnixNano())
	var modelID uint
	defer func() { cleanupModel(t, baseURL, jwtToken, client, &modelID) }()

	rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias, nil)
	if rsp.Error != nil {
		t.Fatalf("create model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}

	item := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if item == nil {
		t.Fatalf("created model alias=%s not found in list", alias)
	}
	modelID = item.ID
	if !slices.Equal(item.Capabilities, []string{"text"}) {
		t.Fatalf("default capabilities=%v want [text]", item.Capabilities)
	}
}

// TestModelCapabilities_CreateRejectsInvalid 验证非法能力集合被业务层拒绝（HTTP 200 + error 负载）。
func TestModelCapabilities_CreateRejectsInvalid(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	for _, caps := range [][]string{{"image"}, {"text", "blob"}} {
		alias := fmt.Sprintf("e2e-cap-bad-%d", time.Now().UnixNano())
		rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias, caps)
		if rsp.Error == nil {
			if m := getModelByAlias(t, baseURL, jwtToken, client, alias); m != nil {
				id := m.ID
				cleanupModel(t, baseURL, jwtToken, client, &id)
			}
			t.Fatalf("expected business error for capabilities=%v but got success (traceID=%s)", caps, traceID)
		}
	}
}
```

- [ ] **Step 2: 编译检查 + 无 env 时 Skip**

Run: `go vet ./test/e2e/model_capabilities/ && go test -count=1 -v ./test/e2e/model_capabilities/ 2>&1 | tail -5`
Expected: 无 env 时 `SKIP`

- [ ] **Step 3: Commit**

```bash
git add test/e2e/model_capabilities/ && git commit -m "test(e2e): add model capabilities round trip suite"
```

> 真实环境验证（`BASE_URL=... JWT_TOKEN=<admin> go test -v ./test/e2e/model_capabilities/`）在部署后进行，见收尾任务。

---

### Task 5: 前端管理页 — 表单开关 + 能力列 + i18n

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/app/(dashboard)/models/page.tsx`
- Modify: `web/src/locales/zh.json` / `en.json` / `ja.json`

**Interfaces:**
- Consumes：后端 `ModelItem.capabilities: ("text"|"image")[]`
- Produces（Task 6 消费）：`ModelItem.capabilities: ModelCapability[]`、`export type ModelCapability`

- [ ] **Step 1: types**

`web/src/lib/types.ts` 的 `// ─── Model ───` 段：

```ts
export type ModelCapability = "text" | "image";

export interface ModelItem {
  // ...原字段...
  capabilities: ModelCapability[];
}

export interface CreateModelReqBody { /* ... */ capabilities?: ModelCapability[] }
export interface UpdateModelReqBody { /* ... */ capabilities?: ModelCapability[] }
```

- [ ] **Step 2: models page 表单**

`ModelForm` 加 `supportText: boolean; supportImage: boolean`；`emptyForm` 默认 `supportText: true, supportImage: false`。
`openEdit` 回填：`supportText: (model.capabilities ?? ["text"]).includes("text")`、`supportImage: …includes("image")`。
`openCreate` 不需要改（用 emptyForm）。
`handleSave` 前置校验：

```ts
if (!form.supportText) {
  toast.error(t("models.capabilities_require_text"));
  return;
}
```

create / update 请求体加：

```ts
capabilities: [
  ...(form.supportText ? (["text"] as const) : []),
  ...(form.supportImage ? (["image"] as const) : []),
],
```

表单 UI（放在上下文字段 grid 之后、endpoint select 之前）：

```tsx
<div className="space-y-1">
  <Label>{t("models.capabilities")}</Label>
  <div className="grid grid-cols-2 gap-3">
    <div className="flex items-center justify-between rounded-lg border border-input px-3 py-2">
      <span className="flex items-center gap-1.5 text-sm">
        <Type className="size-3.5 text-muted-foreground" />
        {t("models.capability_text")}
      </span>
      <Switch size="sm" checked={form.supportText} onCheckedChange={(v) => setForm((f) => ({ ...f, supportText: v }))} />
    </div>
    <div className="flex items-center justify-between rounded-lg border border-input px-3 py-2">
      <span className="flex items-center gap-1.5 text-sm">
        <ImageIcon className="size-3.5 text-muted-foreground" />
        {t("models.capability_image")}
      </span>
      <Switch size="sm" checked={form.supportImage} onCheckedChange={(v) => setForm((f) => ({ ...f, supportImage: v }))} />
    </div>
  </div>
</div>
```

lucide import 追加：`Type`、`Image as ImageIcon`。

- [ ] **Step 3: 列表「能力」列 + 移动端徽标**

桌面上一种紧凑图标徽标（复用 limits 徽标样式）。文件内新增小组件+小 helper（紧跟 `formatTokens` 后）：

```tsx
// 能力徽标：按模型输入模态渲染图标（text / image），未知模态回退为 Type 图标
function CapabilityBadges({ capabilities }: { capabilities?: string[] }) {
  const caps = capabilities && capabilities.length > 0 ? capabilities : ["text"];
  return (
    <div className="flex items-center gap-1.5">
      {caps.map((cap) => (
        <span
          key={cap}
          className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground"
          title={cap}
        >
          {cap === "image" ? (
            <ImageIcon className="size-3 text-muted-foreground" />
          ) : (
            <Type className="size-3 text-muted-foreground" />
          )}
        </span>
      ))}
    </div>
  );
}
```

桌面表格：`<TableHead>{t("models.limits")}</TableHead>` 后插入 `<TableHead>{t("models.capabilities")}</TableHead>`；对应 TableCell 内放 `<CapabilityBadges capabilities={model.capabilities} />`。
移动端卡片：limits 徽标的 flex-wrap 容器内两个 limit span 之后加 `<CapabilityBadges capabilities={model.capabilities} />`。

- [ ] **Step 4: i18n 三语补键**（插入到各文件 `"models.max_output_hint"` 行之后）

zh.json：

```json
  "models.capabilities": "能力",
  "models.capability_text": "文本输入",
  "models.capability_image": "图片输入",
  "models.capabilities_require_text": "必须支持文本输入",
```

en.json：

```json
  "models.capabilities": "Capabilities",
  "models.capability_text": "Text input",
  "models.capability_image": "Image input",
  "models.capabilities_require_text": "Text input support is required",
```

ja.json：

```json
  "models.capabilities": "機能",
  "models.capability_text": "テキスト入力",
  "models.capability_image": "画像入力",
  "models.capabilities_require_text": "テキスト入力のサポートは必須です",
```

- [ ] **Step 5: lint + 构建 + 确认 dist 无变更**

Run: `cd web && npm run lint && npm run build && cd .. && git status --porcelain internal/web/dist`
Expected: lint/build 通过；最后一行输出为空

- [ ] **Step 6: Commit**

```bash
git add web/ && git commit -m "feat(web): model capabilities form switches and table badges"
```

---

### Task 6: 导出脚本 — OpenCode `modalities` / Pi `input`

**Files:**
- Modify: `web/src/components/export-dialog-shared.tsx`（共享 helper）
- Modify: `web/src/components/export-dialog.tsx`（OpenCode）
- Modify: `web/src/components/export-pi-dialog.tsx`（Pi）
- 不动：`export-claudecode-dialog.tsx` / `export-codex-dialog.tsx`（schema 无 per-model 模态项）

**Interfaces:**
- Consumes: Task 5 的 `ModelItem.capabilities`
- Produces: `modelCapabilities(model: ModelItem): string[]`（导出弹窗共用）

- [ ] **Step 1: 共享 helper**

`export-dialog-shared.tsx`，加在 `useFilteredModels` 附近：

```tsx
// 解析模型输入模态集合（能力）；空值/缺省兜底为 ["text"]，与服务端 DB 默认值一致
export function modelCapabilities(model: ModelItem): string[] {
  return model.capabilities && model.capabilities.length > 0 ? model.capabilities : ["text"];
}
```

- [ ] **Step 2: OpenCode 每模型条目补 `modalities` + 条件 `attachment`**

`export-dialog.tsx` 的 `generateScript` 目前用 `Object.fromEntries` 构造 modelsJson。改为：

```tsx
  const modelsJson = JSON.stringify(
    Object.fromEntries(
      selectedModels.map((m) => {
        const caps = modelCapabilities(m);
        return [
          m.alias,
          {
            name: m.alias.charAt(0).toUpperCase() + m.alias.slice(1),
            // 仅当模型支持图片输入时放开 OpenCode UI 的图片附件上传
            ...(caps.includes("image") ? { attachment: true } : {}),
            modalities: { input: caps, output: ["text"] },
            limit: {
              context: m.contextLength > 0 ? m.contextLength : 128000,
              output: m.maxOutputTokens > 0 ? m.maxOutputTokens : 64000,
            },
            temperature: true,
            tool_call: true,
          },
        ];
      })
    ),
    null,
    4
  );
```

import 行加 `modelCapabilities`。

- [ ] **Step 3: Pi 的 `input` 数组按能力生成**

`export-pi-dialog.tsx` 的 `buildPiModels`：

```tsx
function buildPiModels(models: ModelItem[]) {
  return models.map((model) => ({
    id: model.alias,
    name: model.alias,
    reasoning: true,
    input: modelCapabilities(model),
    // ...其余原样...
  }));
}
```

import 行加 `modelCapabilities`。

- [ ] **Step 4: lint + 构建**

Run: `cd web && npm run lint && npm run build`
Expected: 通过

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ && git commit -m "feat(web): export opencode modalities and pi input from model capabilities"
```

---

### Task 7: 文档回写 + chrome mcp 验证 + 收尾

**Files:**
- Modify: `CONTEXT.md`（新增 ModelCapabilities 词条、更新 ClientConfigExport 词条）
- Modify: `docs/superpowers/specs/2026-07-29-model-capabilities-design.md`（补「实现期修订」附录：enum 落位、text 列型、map-update 手动序列化）
- 产出：serena memory（沉淀工程经验）

- [ ] **Step 1: CONTEXT.md 两节更新**（LLM Proxy 段）

新增词条（放 `ClientConfigExport` 词条之前）：

```markdown
**ModelCapabilities（模型能力）**:
模型支持的输入模态集合，持久化为 `models.capabilities` text 列（serializer:json，如 `["text","image"]`），成员为已知枚举 `InputModality`（`text` / `image`，后续可扩展更多模态）。集合必须非空且包含 `text`。与 Endpoint 的协议能力（支持哪些 LLM 接口协议）是不同概念。管理页以开关配置、徽标展示；ClientConfigExport 据此生成 OpenCode `modalities.input`（含 `image` 时附 `attachment: true`）与 Pi `input` 数组。
_Avoid_: model features, model flags
```

`ClientConfigExport` 词条末尾补一句：

```markdown
OpenCode 模型条目含 `modalities` 字段（且图片输入模型附 `attachment: true`），Pi 模型含 `input` 数组，两者均由 **ModelCapabilities** 生成。
```

- [ ] **Step 2: spec 文档补「实现期修订」附录**（3 点：enum 落位 / text 列 / map-update 手动序列化 + session repo 存量隐患备注）

- [ ] **Step 3: chrome mcp 交互验证**

前提（缺一可降级）：本地 `cd web && npm run dev` 起站点且可访问后端；用 chrome mcp 打开 models 页：
1. 创建模型弹窗：两开关展示，图片开 → 保存 → 列表「能力」列出现 Type+Image 两个徽标
2. 打开 OpenCode 导出弹窗选中该模型 → 预览脚本 JSON 含 `modalities.input: ["text","image"]` 与 `attachment: true`
3. 打开 Pi 导出弹窗选中该模型 → `input: ["text","image"]`
4. 关闭图片输入再保存 → 两脚本对应字段回退 `["text"]`、OpenCode 不输出 `attachment` 键
（不可行降级：`npm run build` 通过后由用户自测，报告里注明。）

- [ ] **Step 4: 全量回归 + ponytail-review**

Run: `go test -count=1 ./cmd/... ./internal/... ./test/... && make lint && cd web && npm run lint && npm run build`
Expected: 全绿。随后对 `git diff master...HEAD` 走 ponytail-review（审视投机抽象/死代码），发现问题就地修。

- [ ] **Step 5: serena 沉淀 + Commit**

`serena_write_memory`：内容为「GORM `Updates(map)` 不经过 serializer:json（raw 值进 SQL）——手动 marshal」+「生产 serializer 列均为 text」+「aggregate.CreateModel 签名扩散点清单」。随后：

```bash
git add CONTEXT.md docs/ && git commit -m "docs: record model capabilities glossary and implementation deviations"
```

- [ ] **Step 6: 询问用户合并/MR；部署后跑真实 E2E**

合并 master → docker-publish 自动部署 → `BASE_URL=<prod> JWT_TOKEN=<admin> go test -v -count=1 ./test/e2e/model_capabilities/`；失败则取 `X-Trace-Id` 走 cls-log-bugfix。
