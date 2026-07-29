# 模型能力（ModelCapabilities）配置、展示与导出设计

> 2026-07-29 · 状态：已评审 · 分支：`feature/model-capabilities-2026-07-29`

## 1. 背景与目标

models 管理（`web/src/app/(dashboard)/models/page.tsx` + `internal/**` 的 Model DDD 链路）目前只有别名、上游模型名、上下文长度、最大输出等基础配置。本需求为模型增加**能力（capabilities）配置与展示**：

- 当前配置项两项：**是否支持文本输入**、**是否支持图片输入**
- 后续需零迁移成本拓展到更多模态（audio / pdf / video 等）
- 「导出到编程工具」（ClientConfigExport，纯前端能力）需根据模型能力写入对应配置项

## 2. 关键决策

| 决策点 | 结论 | 理由 |
|--------|------|------|
| 存储模型 | `model` 表新增 **单个 jsonb 列 `capabilities`**，语义为输入模态集合（如 `["text","image"]`） | 新模态仅需新增枚举值 + UI 开关，**零 DB migration**；与 OpenCode / Pi 配置的集合形状 1:1 对应。否决逐能力布尔列方案（每加模态需贯穿 DB→聚合→DTO→UI 加列） |
| 默认值 | 存量与新模型默认 `["text"]`（支持文本、不支持图片） | 与现存所有模型现状一致 |
| 校验 | 集合**非空、必须包含 text**、成员必须属于已知枚举 | 本代理下 LLM 均接受文本输入；防止导出工具拿到 `["image"]` 之类的无效配置。输入校验属不可简化场景 |
| 导出映射 | 仅 **OpenCode**（`modalities.input` + image 时 `attachment: true`）与 **Pi**（`input` 数组）消费能力配置 | Claude Code（tier→env 映射）与 Codex（config.toml）的配置 schema 无 per-model 模态配置项，保持不动 |

## 3. 领域概念与命名

- **ModelCapabilities（模型能力）**：模型支持的输入模态集合，成员为已知枚举 `InputModality`（`text` / `image`，可扩展）。与 Endpoint 的协议能力（`endpoints.capabilities`，指 OpenAI/Anthropic 各接口支持标记）是不同概念——前者描述模型能"吃什么"模态输入，后者描述端点能"讲什么"协议。`CONTEXT.md` 两词条同步维护。
- VO：`internal/domain/llmproxy/vo` 新增 `InputModality`（string 枚举）。
- i18n 键挂在 `models.*` 命名空间（`models.capabilities` / `models.capability_text_input` / `models.capability_image_input`）。

## 4. 详细设计

### 4.1 存储层

- DB 模型 `internal/infrastructure/database/model/model.go` 的 `Model` 新增字段：
  - `Capabilities []string`，列 `capabilities`，jsonb，`NOT NULL DEFAULT '["text"]'`，沿用项目现有 `serializer:json` 模式（同 `message` 表的 `serializer:json` 惯例）
  - `constant/sql.go` 新增 `FieldModelCapabilities`，并加入 `ModelRepoFieldsFull`
- GORM `AutoMigrate` 自动加列并回填存量行默认值，无需手写迁移脚本；需本地 docker PG 验证迁移后存量行 `capabilities = ["text"]`
- API JSON 形状：`"capabilities": ["text","image"]`（扁平字符串数组；未来出现非模态类能力时再演化为嵌套结构，当前 YAGNI）

### 4.2 后端链路（沿现有 DDD 分层透传，不新增结构）

| 层 | 文件 | 改动 |
|----|------|------|
| VO | `internal/domain/llmproxy/vo/` | 新文件 `input_modality.go`：`InputModality` 枚举 + 集合校验（非空、含 text、成员合法） |
| 聚合 | `internal/domain/llmproxy/aggregate/model.go` | `Model` 加 `capabilities []InputModality`；`CreateModel` 校验能力集合；`Update` 支持 nil 不改/非 nil 整体替换 |
| 命令 | `internal/application/model/port/handler.go`、`command/create_model.go`、`command/update_model.go` | Command/View 透传能力集合；create 时空值兜底 `["text"]`（同 contextLength 默认值模式） |
| DTO | `internal/dto/model.go` | `CreateModelReqBody` / `UpdateModelReqBody` / `ModelItem` 加 `capabilities`（`[]string`） |
| Handler | `internal/handler/model.go` | 四处映射透传 |
| Repository | `internal/infrastructure/repository/endpoint_repository.go`（modelRepository 段） | `toModelAggregate` / `toModelDBModel` / `Update` 的 updates map 同步 |

### 4.3 管理页 UI

- `web/src/lib/types.ts`：`ModelItem` / `CreateModelReqBody` / `UpdateModelReqBody` 加 `capabilities?: string[]`
- **表单**（创建/编辑 Dialog）：两个开关——
  - 「文本输入」默认开
  - 「图片输入」默认关
  编辑态按 `model.capabilities` 回显；文本开关关闭时保存被前端阻断（必须含 text）
- **列表展示**：表格新增「能力」列，紧凑图标徽标展示文本/图片（复用现有 limits 徽标的样式语言）；移动端卡片同步加徽标
- i18n：zh / en / ja 三个 locale 文件补键

### 4.4 导出脚本（ClientConfigExport，纯前端）

| 工具 | 文件 | 改动 |
|------|------|------|
| OpenCode | `web/src/components/export-dialog.tsx` | 每模型条目新增 `modalities: { input: caps, output: ["text"] }`；当 `caps` 含 `image` 时追加 `attachment: true`（OpenCode UI 据此放开图片附件上传） |
| Pi | `web/src/components/export-pi-dialog.tsx` | `buildPiModels` 的 `input: ["text"]` 硬编码改为从 `model.capabilities` 生成 |
| Claude Code | `web/src/components/export-claudecode-dialog.tsx` | 无改动（配置 schema 无 per-model 模态项） |
| Codex | `web/src/components/export-codex-dialog.tsx` | 无改动（config.toml 无 per-model 模态项） |

caps 计算统一为一个共享 helper（放 `export-dialog-shared.tsx`）：`modelCapabilities(m) => string[]`，空值兜底 `["text"]`。

## 5. 测试与验证

- **单测**（`test/unit/`）：
  - 聚合校验：空集合 / 不含 text / 非法成员 → 报错；合法集合通过
  - create 命令：空值兜底 `["text"]`
  - repository 列常量一致性：扩展现有 `test/unit/model_repository/model_update_test.go` 的 `TestModelUpdateColumnConstantsMatchGORMTags` 模式，把 `FieldModelCapabilities` 加入校验
- **E2E**（`test/e2e/model_capabilities/`）：用 admin `JWT_TOKEN` 调管理 API，create（含 image）→ list 断言 capabilities round-trip → update 关闭 image → 再断言；create 不传 capabilities → 断言默认 `["text"]`
- **迁移验证**：本地 docker PG 起服务，确认 AutoMigrate 后存量行 `capabilities = ["text"]`
- **Web 验证**（chrome mcp）：models 页表单开关、列表徽标；OpenCode 导出脚本含 `modalities` + `attachment`；Pi 导出脚本 `input` 按能力生成

## 6. 文档回写

- `CONTEXT.md`：新增 **ModelCapabilities** 词条；更新 **ClientConfigExport** 词条（OpenCode / Pi 导出已感知模型能力）
- `web/CONTEXT.md`：无需新增词条（能力徽标属 models 页局部展示，不构成新领域概念）

## 7. 不做的事（YAGNI）

- 不做输出模态（output modalities）配置——导出 `modalities.output` 固定 `["text"]`
- 不做 reasoning / tool_call 等非模态类能力配置——保持当前导出脚本硬编码
- 不做能力驱动的网关侧行为（如按能力拦截图片请求）——本需求仅配置、展示、导出
- 无 per-model Claude Code / Codex 能力配置项——目标格式不支持
