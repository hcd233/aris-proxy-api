# 常量按领域重组设计

## 问题

`internal/common/constant/` 下常量按类型拆分（`string.go`、`number.go`、`time.go`、`rate.go`、`database.go`、`http.go`、`agent.go`、`ctx.go`），其中 `string.go` 已膨胀至 1040 行，混杂 HTTP Header、SSE 协议、Redis Key、OAuth、API Key、CORS、路由路径、CLS 字段、Cron、DB 字段、上游 API 路径等完全不相关的常量，维护困难。

## 方案

按**领域**而非**类型**组织文件。每个文件只包含一个领域相关的常量集合。所有文件同属 `package constant`，**不改变常量名，不改变包名**，外部文件无需修改 import。

## 设计

### 保留的文件（领域已纯，仅微调）

| 文件 | 内容 | 变更 |
|------|------|------|
| `ctx.go` | Context Key (`CtxKey*`) | 不变 |
| `agent.go` | Agent 名称/描述/指令 | 不变 |
| `http.go` | HTTP Header 名/值、Content-Type、HTTP 连接池、客户端超时 | 从 `string.go`/`time.go` 迁入 |
| `database.go` | DB 连接池、字段名、查询条件、聚合根类型 | 从 `string.go`/`time.go` 迁入 |

### 新增的文件

| 文件 | 容纳的常量 | 来源 |
|------|-----------|------|
| `route.go` | 路由路径（`RoutePath*`） | `string.go` |
| `sse.go` | SSE 协议前缀、帧模板、心跳间隔/次数 | `string.go`/`number.go`/`time.go` |
| `upstream.go` | 上游 API 路径、API 版本、版本头、错误类型/消息模板、ID 模板 | `string.go` |
| `apikey.go` | API Key 前缀/字符集/随机长度/最大数量/管理限频 | `string.go`/`number.go`/`rate.go` |
| `oauth.go` | OAuth Provider 标识/URL、State 字节数/限频/TTL/清理间隔 | `string.go`/`number.go`/`rate.go`/`time.go` |
| `rediskey.go` | Redis Key 模板（JWT 缓存、令牌桶、扫描封禁/违规计数、锁） | `string.go` |
| `session.go` | Session 摘要/评分/去重 Agent 配置+重试+Token+cron | `string.go`/`number.go`/`agent.go` |
| `cron.go` | 通用 cron 模块名/表达式（不含 session 专属的） | `string.go` |
| `ratelimit.go` | 限频 Period/Limit（代理 LLM、Token 刷新、API Key 管理、Guard） | `rate.go` |
| `guard.go` | 路由扫描封禁阈值/窗口/时长 | `rate.go`/`time.go` |
| `log.go` | 日志文件名、轮转大小/备份数/天数、CLS 级别/字段 | `number.go`/`string.go` |
| `objectstorage.go` | COS/MinIO URL 模板、目录模板、连接池、预签名 | `string.go`/`time.go`/`database.go` |
| `security.go` | 掩码模板/占位符/最低长度、字节范围 | `number.go`/`string.go` |
| `conversation.go` | 消息格式模板（`MessageFormat*`）、`MessageContentSeparator` | `string.go` |

### `string.go` 保留的常量

精简为通用工具性常量：
- `ProjectName`
- 格式化模板：`FormatDefault`、`FormatDecimal`、`FormatFloatCompact`
- 截断提示：`TruncateSuffixPrefix`、`TruncateSuffixPostfix`
- 常用字符串：`NewlineString`、`NewlineCRLF`、`ZeroString`、`OneString`、`NullJSONLiteral`、`QuoteString`、`ColonMessageTemplate`
- JSON Schema 类型：`JSONSchemaTypeString`/`Number`/`Boolean`/`Array`/`Object`
- 基础设施模板：`HostPortTemplate`、`PostgresDSNTemplate`、`DefaultFormatJSON`
- Data URL：`DataURLTemplate`、`DataURLPrefix`、`DataURLBase64Separator`、`Base64SourceType`、`URLSourceType`
- DI 名称：`DigNameAccessSigner`、`DigNameRefreshSigner`
- OpenAPI：`OpenAPISchemasPrefix`、`OpenAPIDocsPath`、`OpenAPISchemasPath`

### 删除的文件

| 文件 | 原因 |
|------|------|
| `number.go` | 所有常量已分配到 `sse.go`/`apikey.go`/`oauth.go`/`security.go`/`log.go`/`session.go` |
| `rate.go` | 所有常量已分配到 `ratelimit.go`/`apikey.go`/`oauth.go`/`guard.go` |
| `time.go` | 所有常量已分配到 `http.go`/`database.go`/`sse.go`/`oauth.go`/`objectstorage.go`/`guard.go`/`cors.go` |

## 影响范围

- **包名不变**：`package constant`，外部 import 路径不变
- **常量名不变**：所有引用处无需修改代码
- **无运行时影响**：纯文件组织变更，Go build 和测试可完全验证
- **总文件数**：8 → 19（保留 4 + 新增 11 + 删除 4）

## 验证方式

1. `go vet ./internal/common/constant/...` — 无编译/声明冲突
2. `go test -count=1 ./...` — 全量测试通过
3. `make lint-conv` — 自定义规范通过
