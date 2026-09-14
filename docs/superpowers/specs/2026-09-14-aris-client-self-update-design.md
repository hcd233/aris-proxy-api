# aris 客户端更新检查与自更新设计

> 日期：2026-09-14
> 状态：已批准（设计评审通过）
> 范围：`aris` 客户端「使用中检查更新」+ `aris update` 自更新命令
> 前置文档：`2026-09-06-aris-version-command-design.md`（版本号注入；当时把「查 GitHub latest」列为 YAGNI，本次需求显式推翻该边界）

## 背景与目标

客户端二进制固定安装在 `~/.aris/bin/aris`（`GET /install.sh` 安装脚本写入），版本由 `-ldflags -X main.version=<tag>` 注入，本地构建回退 `dev`。当前用户不会收到任何更新提示，也无法自我升级——只能重新执行安装脚本。

目标：

1. **使用中检查更新**：正常使用 CLI 时自动发现新版本并提示，不阻塞、不干扰命令本身。
2. **`aris update`**：一条命令把自身升级到最新 release。

## 决策记录

| 决策点 | 结论 | 理由 |
|--------|------|------|
| 更新源 | GitHub Releases `releases/latest/download`（与安装脚本同源） | 安装脚本已依赖该源；产物由 `release-client.yml` 发布；不新增服务端接口 |
| 最新版本探测方式 | `HEAD {base}/aris-{os}-{arch}.tar.gz` 抓第一跳重定向 URL 取 tag | 免 GitHub API、免 API 限流；实测返回 302 → `.../releases/download/v0.2.2/...`，path 段直接含 tag |
| 本地缓存 | **不做**（无 state 文件、无 TTL） | 检查是异步的、用户感知延迟为 0；正常手动使用一天仅数次 HEAD。缓存只省下一次后台请求，却引入 staleness（24h 内发版会提示滞后）、状态文件与并发写。用户明确否决 |
| 检查时机 | 后台 goroutine 与命令并行，命令结束后最多等 `ArisClientUpdateNoticeWait`(400ms) 打印提示 | 不阻塞、不改变命令退出码；慢网络时本轮不提示，下次再说 |
| 提示输出 | stderr，一行 | 不污染 stdout（`--json`、管道场景安全） |
| 触发范围 | 仅 TTY 交互式的非 hook 命令 | hook 路径（`trace`）每事件调用一次，脚本/CI 可能循环调用，都不能打 GitHub |
| 「已是最新」判定 | `IsNewer(latest, current)` 数字段比较，不用字符串不等 | 避免 `v0.10.0` 与 `v0.9.9` 误判，也避免把本地 `dev`/预发布构建提示成"降级" |
| 自更新落盘 | 自身二进制同目录写临时文件（0700）→ `rename` 原子覆盖 | 失败不破坏现有二进制；同目录保证同文件系统 |
| 二进制定位 | 复用 `trace.ExecutablePath()`（`os.Executable` + `EvalSymlinks`） | 已存在且被 status/hook 安装器使用；解析符号链接正是自替换需要的语义 |
| sha256 校验 | **保留**（tar.gz + `.sha256` 同源） | 不提供防篡改（同源），但与 `install_aris_client.sh.tmpl` 行为一致，且能拦下截断/损坏产物 |
| 命令新增 | 新增 `aris update` | 用户明确要求，属 `commands.md`「非用户明确要求不允许新增 cobra 命令」的例外 |
| 交互确认 | 不做，`aris update` 直接执行 | 是用户显式动作；复用 `ui.RunWithSpinner` 显示进度（非 TTY 自动静默） |

## 架构

新增 `internal/client/update/`（纯客户端逻辑，不链接 server 侧、不依赖 `~/.aris` 状态目录）：

| 文件 | 内容 |
|------|------|
| `version.go` | `parseVersion`（`vX.Y.Z`，去 `v` 前缀，缺省段补 0，非数字即不可解析）与 `IsNewer(latest, current string) bool`；`ShouldCheck(current string, interactive bool) bool`（`dev`/空/不可解析 → false；`ARIS_NO_UPDATE_CHECK` 为真值 → false；非交互 → false） |
| `update.go` | `Latest(ctx)`（HEAD 抓 tag）、`Download(ctx)`（tar.gz + `.sha256`）、`extractBinary`（`archive/tar` + `compress/gzip`，按 `constant.ArisClientBinaryFileName` 取成员，限制解压大小）、`replaceSelf`（临时文件 + rename）、`Run(ctx, Options)` |
| `check.go` | `StartCheck(ctx, CheckOptions) func()`：启动 goroutine 解析 latest，返回的函数等待最多 400ms 并在有新版时打印提示；任何错误全部静默 |

命令壳：

- `cmd/client/update.go`：`aris update` 子命令（`Args: cobra.NoArgs`，`RunE` 调 `update.Run`）。
- `cmd/client/root.go`：注册子命令；`execute()` 在 `root.Execute()` 前后包一层「启动检查 → 等提示」，并判定命令是否参与检查。

常量集中在新增 `internal/common/constant/clientupdate.go`（更新源 URL、env 名、超时、资产名模板、权限位、全部文案），满足 `lintconv` 的硬编码 URL / magic string 规则。

### 关键 API

```go
// update.go
type Options struct {
	Current    string    // 当前版本（main.version）
	In         io.Reader // spinner 输入
	Out        io.Writer // spinner 输出
	BaseURL    string    // 空 → ARIS_UPDATE_BASE_URL → constant 默认
	BinaryPath string    // 空 → trace.ExecutablePath()（测试注入，避免覆盖测试二进制）
	HTTPClient *http.Client
}
func Run(ctx context.Context, opts Options) error

// check.go
type CheckOptions struct {
	Current    string
	Out        io.Writer
	BaseURL    string
	HTTPClient *http.Client
}
func StartCheck(ctx context.Context, opts CheckOptions) func()
```

## 数据流

### 使用中检查

```
execute()
  ├─ ShouldCheck(version, isTTY(os.Stderr)) 且命令不在跳过清单
  │     → finish := update.StartCheck(ctx, {Current: version, Out: os.Stderr})
  ├─ root.Execute()                            // 检查与命令并行
  └─ finish()                                  // 最多等 400ms；有新版则 stderr 打印一行
```

跳过清单（`cmd/client` 判定，走 `root.Find(os.Args[1:])` 的命令路径）：

- 命令路径中任一层是 `trace`（hook 每事件调用）/ `update`（自我递归）/ `version`
- 命中 root 本身（裸 `aris`、`aris --version`、`aris --help`）
- stderr 非 TTY（脚本、CI、管道）
- `version` 为 `dev` 或不可解析（本地/预发布构建）
- `ARIS_NO_UPDATE_CHECK` 为真值（`1/true/yes`）

提示文案（新建常量，英文，与既有客户端文案一致）：

```text
! A new version <latest> is available (current <current>) — run aris update
```

### `aris update`

```
Run()
  ├─ Latest(ctx)                         // HEAD 取 tag；取不到 → 明确报错
  ├─ 当前版本可解析且 !IsNewer(latest, current) → 打印 "aris is up to date (<current>)" 结束（不下载）
  ├─ Download(ctx)                       // spinner: "Downloading aris <latest>..."
  │    ├─ GET {base}/aris-{os}-{arch}.tar.gz   (限时 5min，限制体积)
  │    └─ GET {base}/aris-{os}-{arch}.tar.gz.sha256 → 解析并比对，失败即报错（不替换）
  ├─ extractBinary()                     // 从 tar.gz 取 aris 成员
  ├─ replaceSelf()                       // 同目录临时文件 0700 → rename 覆盖自身
  └─ 打印 "Updated aris <current> → <latest> (<path>)"
```

平台映射：`runtime.GOOS/GOARCH` → 既有 `constant.ArisClientOS{Darwin,Linux}` / `ArisClientArch{AMD64,ARM64}`；其他组合直接报错（产物只有四平台）。

## 错误处理

- **使用中检查链路**：任何错误（网络、解析、非 2xx、无重定向）全部静默，不写日志、不影响退出码。
- **`aris update`**：错误统一 `ierr.Wrap(ierr.ErrProxySend / ierr.ErrValidation / ierr.ErrInternal, ...)`，禁止 `fmt.Errorf`；错误路径绝不 `rename`，因此现网二进制保持可用。
- 取不到 tag（GitHub 不再重定向、返回 200）：`aris update` 报 `Failed to determine the latest aris version.`；使用中检查静默跳过。
- 二进制所在目录不可写（如 `0755` 系统目录）：`os.CreateTemp` 报错转换为可读信息，不破坏原文件。
- 版本比较不可解析（如 `dev`）：`IsNewer` 返回 false → 只在不下载的路径上体现为「不提示 / 直接升级」，不会误判为降级。

## 测试

### 单元测试 `test/unit/client/update/`

- `version_test.go`：`IsNewer` 表驱动（`v0.2.2 > v0.2.1`、相等、`v0.10.0 > v0.9.9`、`dev`、空串、`v1`/`v1.2` 缺省段）；`ShouldCheck` 表驱动（`dev`、空、不可解析、非交互、`ARIS_NO_UPDATE_CHECK=1`、正常）。
- `update_test.go`（`httptest` 提供 release 资产，`BinaryPath` 指向临时文件，绝不触碰测试二进制）：
  1. 最新 > 当前 → 目标文件内容被替换为新二进制、内容正确；
  2. 最新 == 当前 → 只输出 up to date，且不发下载请求（服务端断言请求数）；
  3. checksum 不匹配 → 返回错误且目标文件保持原内容；
  4. 302 无 tag / 非 2xx → `Latest` 报错；
  5. 损坏 tar.gz → 报错且目标文件不变。
- `check_test.go`：`StartCheck` 在有新版时向传入 writer 打印提示；无新版/报错时零输出；不阻塞超过等待上限。

### E2E `test/e2e/clientcmd/update_test.go`

用 `exec.CommandContext` 构建真实二进制（`-ldflags "-X main.version=v0.1.0"`）到临时目录，本地 `httptest` 提供 `aris-{os}-{arch}.tar.gz`（内含 `-X main.version=v9.9.9` 的二进制）+ 正确 `.sha256`，通过 `ARIS_UPDATE_BASE_URL` 注入：

1. `aris update` 成功 → 再次执行 `aris version` 输出 `v9.9.9`（验证真实自替换）；
2. 服务端返回 checksum 不匹配的产物 → `aris update` 非零退出，`aris version` 仍为 `v0.1.0`（验证失败不破坏二进制）。

**覆盖边界（已知）**：使用中提示要求 stderr 是 TTY，`exec` 路径下没有 pty，因此 E2E 不覆盖提示打印；提示文案与 TTY 门控由单元测试覆盖。

## 文档同步

- `CONTEXT.md` 的 `ArisClient` 条目：命令清单补 `update`，并补一句「使用中更新检查」语义（后台异步、仅 TTY、`ARIS_NO_UPDATE_CHECK` 可关、`ARIS_UPDATE_BASE_URL` 可换源）。
- `README.md` 客户端 CLI 清单补 `aris update`。

## 不做的事（YAGNI）

- 不自动下载 / 静默自动升级（只提示，升级由用户显式触发）。
- 不加 `--force` / `--check` 参数（`aris update` 本身已含"已是最新"分支；`aris version` 已存在）。
- 不做本地缓存 / TTL / 状态文件。
- 不做服务端版本接口、不做私有镜像源配置（仅留 `ARIS_UPDATE_BASE_URL` 环境变量）。
- 不支持 Windows（无产物）。
- 不对 `aris update` 做交互确认或回滚点管理。

## 已知限制

- 更新源是 GitHub，国内网络可能不稳；安装脚本本就依赖它，本次不改变（可用 `ARIS_UPDATE_BASE_URL` 指向镜像）。慢网络下"使用中提示"多半不会出现，属预期降级。
- tag 解析依赖 GitHub 的 `releases/latest/download` 重定向行为；行为变化时 `aris update` 报错、自动提示静默，不影响其他命令。

## 验证

1. `go test -count=1 ./test/unit/client/update/...`（先红后绿）
2. `go test -count=1 ./test/e2e/clientcmd/...`（含既有 `version_test.go` 回归）
3. `make lint` 通过
4. 冒烟：`make build-client && ./aris version`（`dev`，不提示）；`./aris update`（真实 GitHub，会把本地 dev 构建替换为 release 产物，属预期；需要还原时重跑 `make build-client`）
5. 合并后打 tag 发布 → 装旧版客户端跑一次交互命令，确认提示与 `aris update` 生效（发布后人工验证）
