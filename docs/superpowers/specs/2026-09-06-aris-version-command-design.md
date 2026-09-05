# aris version 子命令设计

> 日期：2026-09-06
> 状态：已批准
> 范围：客户端版本号注入 + `aris version` / `aris --version`

## 背景与目标

客户端二进制当前不含任何版本信息（无版本常量、构建不注入）。目标：`aris version` 与 `aris --version` 能输出版本号，Release 产物注入 tag 版本，本地构建回退 `dev`。

## 决策记录

| 决策点 | 结论 | 理由 |
|--------|------|------|
| 版本来源 | ldflags `-X main.version=<tag>` 注入 | 标准做法；`debug.ReadBuildInfo` 对普通 `go build` 仅 `(devel)` 不可靠；硬编码常量每版改码会漂移 |
| 输出格式 | 裸版本字符串（`v0.2.0` / `dev`） | 管道/脚本友好，人读也清晰；保留 tag 的 `v` 前缀 |
| root `--version` | 顺带启用（`root.Version = version`） | cobra 惯例，一行成本 |
| 契约 | 用户明确要求新增 cobra 子命令 | commands.md「非用户明确要求不允许新增 cobra 命令」的例外成立 |

## 改动清单

1. **新增 `cmd/client/version.go`**：包级 `var version = "dev"`（release 注入点）+ `newVersionCommand()`（`Use: "version"`、`Args: cobra.NoArgs`、`RunE` 向 `cmd.OutOrStdout()` 打印 `version`）。风格对齐现有 `status.go` 等命令文件。
2. **`cmd/client/root.go`**：`root.AddCommand(newVersionCommand())`、`root.Version = version`。
3. **`Makefile`**：`VERSION ?= dev`；`CLIENT_LDFLAGS := $(LDFLAGS) -X main.version=$(VERSION)`；`build-client` / `build-client-all` 改用 `CLIENT_LDFLAGS`（server 的 `LDFLAGS` 不动）。
4. **`.github/workflows/release-client.yml`**：构建步骤改为 `VERSION=${GITHUB_REF_NAME} make build-client-all`。

## 测试（TDD，全自动化）

新增 `test/e2e/clientcmd/version_test.go`：测试内 `go build` 两次并断言 stdout——
1. 默认构建（无 ldflags）→ `aris version` 输出 `dev`；
2. `-ldflags "-X main.version=v9.9.9"` 构建 → 输出 `v9.9.9`（覆盖 release 注入路径）；
3. `aris --version` 输出与 `version` 子命令一致。
构建用 `exec.CommandContext`（过 noctx lint）。

## 不做的事（YAGNI）

- 不查 GitHub latest 做更新检查
- 不加 commit/date/JSON 输出
- 不给 server 注入版本、不改 build-and-publish.yml
- server 不复用 `main.version` 符号（client 的 `-X` 对 server 无意义，两者构建 ldflags 分离）

## 验证

1. `go test -count=1 ./test/e2e/clientcmd/` 红→绿
2. `make lint` 通过
3. 冒烟：`make build-client && ./aris version` → `dev`
4. 合并后下次打 tag，Release 产物执行 `aris version` 输出对应 tag（发布后人工验证）
