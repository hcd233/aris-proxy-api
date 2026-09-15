# 2026-09 CR 跟进修复设计

> 日期：2026-09-15
> 状态：已批准（用户指定修复清单）
> 范围：`d46941d1..ca9117ab` 代码审查结论中的 C1、I1-I4、I7、I8 与全部 Minor 项
> 关联文档：`docs/superpowers/specs/2026-09-14-aris-client-self-update-design.md`、`2026-09-14-agent-base-url-proxy-prefix-design.md`

## 背景

近 7 天提交的代码审查给出 1 项 Critical、9 项 Important、若干 Minor。本次修其中
C1、I1、I2、I3、I4、I7、I8 与全部 Minor；显式不修 I5（状态码基数上限）、
I6（前端状态码配色）、I9（TPM 口径），沿用用户裁定。

## 决策记录

| 决策点 | 结论 | 理由 / 被否决的备选 |
|--------|------|--------------------|
| Codex 清理范围（C1） | 反转白名单：除 `[profiles.*]` 外，任何表内的 `model` / `model_provider` / `model_context_window` 行一律剔除 | 旧规则只认 `model_providers.*`，末表是 `[features]`/`[tui]`/`[mcp_servers.*]` 的存量文件修不干净。备选「同表内出现重名才剔除」修不掉单份脏键留在非 provider 表的情况，且仍把脏键留在错误表内 |
| 多行字符串（M） | 清理时跟踪 `"""` / `'''` 三引号状态，字符串内部不判定 root 键 | 白名单反转后误删面扩大，没有状态机会把用户多行字符串里的同形行删掉 |
| 探测超时（I1） | `Latest` 不再内建超时，尊重调用方 ctx；使用中检查包 1.5s，`aris update` 包 30s | 备选「给 `Latest` 加 timeout 参数」把两种语义塞进同一签名，调用点容易再混 |
| 更新源信任链（I2） | `resolveBaseURL` 结果必须通过 scheme 校验：https 放行，http 仅限 loopback（测试/本地镜像）；下载用 client 限制重定向跳数并逐跳复用同一校验 | 完全禁 http 会打断 `httptest` 回归；只校验首跳挡不住 https→http 降级 |
| checksum 口径（I2） | 与 `install_aris_client.sh.tmpl` 对齐：恰好一行、64 位 hex；带文件名时必须与当前资产名一致 | 两套口径会让镜像/CI 行为分叉 |
| 自更新落盘（I3） | 临时文件 `Sync` 后再 `rename`，rename 后 best-effort `Sync` 父目录 | 目录 fsync 失败不阻塞安装（darwin 部分文件系统不支持），错误忽略但留注释 |
| 流取消顺序（I4） | `drainCancelBody` 先 `cancel()` 再 `Close()`（防御性） | 原判断为"h1 客户端 body 未读到 EOF 时 Close 会同步 drain"，实测不成立：Go 1.25 的 h1 客户端 body 带 `earlyCloseFn`，早关直接 abort 连接（微秒级返回，不 drain）。仍按 cancel 先行，因为对会 drain 的 body 实现（`http.ReadResponse` 直接构造、未来换实现）这才是安全顺序；同时补契约用例锁定"Close 后上游请求立刻结束、不阻塞" |
| 并发改名（I7） | `UpdateWithHistorySync` 走乐观锁：`WHERE id = ? AND model_id = ?` + `RowsAffected != 1` 返回冲突 | 备选 `SELECT ... FOR UPDATE` 需要额外读语句与行锁窗口；乐观锁是与现有 `Updates(map)` 最贴合的最小改法 |
| gofmt 段（I8） | pre-commit 的 gofmt 段与 prettier 段同构：只格式化「完全暂存」的 Go 文件，逐个引号追加，`git add --` 单文件 | 保留 `gofmt -w .` + 整表 re-stage 会把未暂存内容带进提交，正是本次为 prettier 修掉的同类缺陷 |
| 指标快照分层（M） | `Snapshot` / `RangeWindow` / `ResolveRange` 下沉到 `internal/application/metrics/port`，infrastructure 反向依赖 port | 备选「只加接口」不能消除应用层对 label 取值格式的依赖 |
| 可执行文件路径（M） | 新增 `internal/client/executable.Path()`，删除 `setup` / `trace` 两份重复实现 | 复用现有包都会引入反向依赖（`update → trace`、`update → setup → trace`） |
| 根 context（M） | `ctx` 在 `cmd/client/main.go` 创建并作为参数传入 `execute`；`isUpdateCheckCommand` 接收显式 args | 入口层建根 context 是项目允许的场景，`os.Args` 直读不可测 |

## 验证方式

- 每个修复先补可失败用例再改实现（C1 回放、I7 并发/删除、I2 解析与 scheme、I8 沙箱断言）。
- 全量 `go test -count=1 ./...`、`go run ./cmd/lint ./...`、`gofmt -l` 三绿。
- E2E `test/e2e/clientcmd` 补安装后权限位断言。

## 与初始审查结论的偏差

- **I4 降级为防御性顺序**：原始 CR 把「先 Close 再 cancel 会同步 drain 整条流」当作已发生缺陷。实测（Go 1.25 / darwin，`net/http` 源码 `bodyEOFSignal.Close` + `earlyCloseFn`）早关不 drain，而是 abort 连接并把 body 标记为未消费；因此该项从「修复已观测缺陷」改为「顺序防御 + 契约用例」，其余 I 项结论复核后保持不变。
- **`e2e/cross_tenant_reference` 偶发失败与本次改动无关**：并行跑 `test/e2e/...` 时该包会因 SQLite `database table is locked: models` 失败；在未改动的 master 上同样复现（同样报错、不同子用例），属既有 flake。
