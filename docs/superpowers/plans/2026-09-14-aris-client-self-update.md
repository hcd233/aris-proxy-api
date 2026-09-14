# aris 客户端更新检查与自更新 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `aris` 客户端在正常使用时后台检查新版本并提示，并新增 `aris update` 把自身升级到最新 release。

**Architecture:** 新增 `internal/client/update/` 包（版本比较 + 启停判定 / 取 tag + 下载校验解包 + 原子自替换 / 后台提示），通过 `HEAD {base}/aris-{os}-{arch}.tar.gz` 抓第一跳重定向 URL 取最新 tag（免 GitHub API），下载 release 资产后校验 sha256、以同目录临时文件 + `rename` 原子替换自身。`cmd/client` 只做接线：新增 `update` 子命令，并在 `execute()` 中对符合条件的交互式命令启动一次后台检查。

**Tech Stack:** Go 1.25 / `spf13/cobra`（命令树）/ 标准库 `archive/tar`+`compress/gzip`+`crypto/sha256`+`net/http` / `charmbracelet/bubbletea` 经既有 `internal/client/ui.RunWithSpinner` / 项目内 `internal/common/ierr`、`internal/common/constant`。

**Spec:** `docs/superpowers/specs/2026-09-14-aris-client-self-update-design.md`

## Global Constraints

- 所有字符串常量（含更新源 URL、env 名、文案、路径模板、权限位）必须放 `internal/common/constant/`；`internal/**` 与 `cmd/**` 下的业务包**禁止本地 `const` 块**（`lint conv` 强制）。
- 错误创建/包装统一走 `github.com/hcd233/aris-proxy-api/internal/common/ierr` 的 `ierr.New` / `ierr.Newf` / `ierr.Wrap`；禁止 `fmt.Errorf`、`errors.New`。
- 禁止 `encoding/json`（统一 `github.com/bytedance/sonic`）；本计划不涉及 JSON。
- 测试只能用标准库 `testing`；禁止 testify / gomock；禁止 `time.Sleep` 做同步。测试文件只能放 `test/unit/<topic>/` 与 `test/e2e/<topic>/`。
- 测试必须 `t.Parallel()`、用 `t.Context()`；测试 helper 必须 `t.Helper()`（`paralleltest` / `usetesting` / `thelper` 强制）。
- `errcheck` 开启 `check-blank`：忽略 error 的语句要带 `//nolint:errcheck // 原因` 注释。
- 客户端面向用户的文案用英文（沿用既有 `internal/common/constant/client*.go` 风格）；代码注释用中文。
- 更新源固定 `https://github.com/hcd233/aris-proxy-api/releases/latest/download`，仅允许 `ARIS_UPDATE_BASE_URL` 覆盖、`ARIS_NO_UPDATE_CHECK` 关闭检查；不新增其他开关。
- 不做本地缓存/TTL/状态文件；不自动安装；不做交互确认。
- 所有 git 操作前缀 `rtk`；开发在 worktree `.worktrees/client-update`（分支 `feature/client-self-update-2026-09-14`）内进行。

## 文件结构

| 文件 | 责任 |
|------|------|
| `internal/common/constant/clientupdate.go`（新建） | 更新源、env 名、超时、体积上限、权限位、命令名、全部文案 |
| `internal/common/constant/http.go`（不改） | 已存在 `HTTPHeaderAuthorization` 等；测试里 `Location` 用字面量（测试目录不受 magic string 规则约束） |
| `internal/client/update/version.go`（新建） | `parseVersion` / `IsNewer` / `IsUpToDate` / `ShouldCheck` / `ShouldCheckCommand` / `envTrue` |
| `internal/client/update/update.go`（新建） | `Run` / `Latest` / `assetName` / `downloadAsset` / `getBody` / `verifyChecksum` / `parseChecksum` / `extractBinary` / `replaceSelf` / `tagFromURL` / `resolveBaseURL` |
| `internal/client/update/check.go`（新建） | `CheckOptions` / `StartCheck`：后台解析 latest，返回等待并打印提示的函数 |
| `cmd/client/update.go`（新建） | `aris update` 子命令壳 |
| `cmd/client/root.go`（修改） | 注册 `update` 子命令；`execute()` 中启动/收尾后台更新检查 |
| `cmd/client/version.go`（修改 1 行） | `var version = constant.ArisClientDevVersion`（与 `dev` 判定单一来源） |
| `test/unit/client/update/version_test.go`（新建） | 版本比较、启停判定表驱动 |
| `test/unit/client/update/run_test.go`（新建） | `Run`/`Latest` 走 `httptest`：替换成功、已最新不下载、checksum 失败、损坏包、无重定向 |
| `test/unit/client/update/check_test.go`（新建） | `StartCheck` 提示、静默、等待上限 |
| `test/e2e/clientcmd/update_test.go`（新建） | 真实二进制自升级 + 失败不破坏现有二进制 |
| `CONTEXT.md` / `README.md`（修改） | 客户端命令清单与 `ArisClient` 词条 |

---

### Task 1: 常量归位 + 版本比较与启停判定

**Files:**
- Create: `internal/common/constant/clientupdate.go`
- Create: `internal/client/update/version.go`
- Create: `test/unit/client/update/version_test.go`
- Modify: `cmd/client/version.go`（`var version = "dev"` → `constant.ArisClientDevVersion`）

**Interfaces:**
- Consumes: 无（首个任务）
- Produces:
  - `constant.ArisClientReleaseBaseURL string`、`ArisClientReleaseAssetFormat`、`ArisClientReleaseChecksumSuffix`、`ArisClientReleaseDownloadSegment`、`ArisClientUpdateBaseURLEnv`、`ArisClientNoUpdateCheckEnv`、`ArisClientEnvValueTrueList`、`ArisClientEnvValueSeparator`、`ArisClientUpdateCheckTimeout time.Duration`、`ArisClientUpdateNoticeWait time.Duration`、`ArisClientUpdateDownloadTimeout time.Duration`、`ArisClientUpdateMaxArchiveBytes int`、`ArisClientUpdateTempPattern`、`ArisClientUpdateBinaryMode`、`ArisClientDevVersion`、`ArisClientCommandTrace|Update|Version`、`ArisClientVersionPrefixV`
  - `update.IsNewer(latest, current string) bool`
  - `update.IsUpToDate(latest, current string) bool`
  - `update.ShouldCheck(current string, interactive bool) bool`

- [ ] **Step 1: 写失败测试**

创建 `test/unit/client/update/version_test.go`：

```go
package update

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestIsNewer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{name: "patch bump", latest: "v0.2.2", current: "v0.2.1", expected: true},
		{name: "minor bump", latest: "v0.3.0", current: "v0.2.9", expected: true},
		{name: "double digit minor is not lexical", latest: "v0.10.0", current: "v0.9.9", expected: true},
		{name: "missing patch segment", latest: "v1", current: "v0.9.9", expected: true},
		{name: "equal versions", latest: "v0.2.2", current: "v0.2.2", expected: false},
		{name: "older latest", latest: "v0.2.1", current: "v0.2.2", expected: false},
		{name: "dev build", latest: "v0.2.2", current: constant.ArisClientDevVersion, expected: false},
		{name: "empty latest", latest: "", current: "v0.2.2", expected: false},
		{name: "prerelease not comparable", latest: "v0.3.0-rc.1", current: "v0.2.2", expected: false},
		{name: "non numeric segment", latest: "v0.3.x", current: "v0.2.2", expected: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := update.IsNewer(testCase.latest, testCase.current); got != testCase.expected {
				t.Fatalf("IsNewer(%q, %q) = %v, want %v", testCase.latest, testCase.current, got, testCase.expected)
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{name: "equal versions", latest: "v0.2.2", current: "v0.2.2", expected: true},
		{name: "current ahead", latest: "v0.2.2", current: "v0.3.0", expected: true},
		{name: "current behind", latest: "v0.2.2", current: "v0.2.1", expected: false},
		{name: "dev build is never up to date", latest: "v0.2.2", current: constant.ArisClientDevVersion, expected: false},
		{name: "empty current", latest: "v0.2.2", current: "", expected: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := update.IsUpToDate(testCase.latest, testCase.current); got != testCase.expected {
				t.Fatalf("IsUpToDate(%q, %q) = %v, want %v", testCase.latest, testCase.current, got, testCase.expected)
			}
		})
	}
}

func TestShouldCheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		current     string
		interactive bool
		env         string
		expected    bool
	}{
		{name: "release build on tty", current: "v0.2.2", interactive: true, expected: true},
		{name: "dev build", current: constant.ArisClientDevVersion, interactive: true, expected: false},
		{name: "empty version", current: "", interactive: true, expected: false},
		{name: "unparsable version", current: "v0.2.2-rc.1", interactive: true, expected: false},
		{name: "not interactive", current: "v0.2.2", interactive: false, expected: false},
		{name: "opt out with 1", current: "v0.2.2", interactive: true, env: "1", expected: false},
		{name: "opt out with true", current: "v0.2.2", interactive: true, env: "true", expected: false},
		{name: "opt out with yes", current: "v0.2.2", interactive: true, env: "YES", expected: false},
		{name: "opt in value with zero", current: "v0.2.2", interactive: true, env: "0", expected: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			t.Setenv(constant.ArisClientNoUpdateCheckEnv, testCase.env)
			if got := update.ShouldCheck(testCase.current, testCase.interactive); got != testCase.expected {
				t.Fatalf("ShouldCheck(%q, %v) with %s=%q = %v, want %v",
					testCase.current, testCase.interactive, constant.ArisClientNoUpdateCheckEnv, testCase.env, got, testCase.expected)
			}
		})
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd .worktrees/client-update && go test -count=1 ./test/unit/client/update/`
Expected: FAIL —— `no required module provides package .../internal/client/update` / `undefined: update.IsNewer`

- [ ] **Step 3: 新增常量文件**

创建 `internal/common/constant/clientupdate.go`：

```go
package constant

import "time"

// aris 客户端自更新（aris update）与使用中更新检查常量
const (
	// ArisClientReleaseBaseURL release 资产源；与 install_aris_client.sh.tmpl 的 gh_base 同源
	ArisClientReleaseBaseURL = "https://github.com/hcd233/aris-proxy-api/releases/latest/download"

	// ArisClientReleaseAssetFormat release 资产名模板（os, arch）
	ArisClientReleaseAssetFormat = "aris-%s-%s.tar.gz"
	// ArisClientReleaseChecksumSuffix 资产校验文件后缀
	ArisClientReleaseChecksumSuffix = ".sha256"
	// ArisClientReleaseDownloadSegment 重定向地址中携带 tag 的路径段：releases/download/<tag>/<asset>
	ArisClientReleaseDownloadSegment = "download"

	// ArisClientUpdateBaseURLEnv 覆盖更新源（镜像/测试用）
	ArisClientUpdateBaseURLEnv = "ARIS_UPDATE_BASE_URL"
	// ArisClientNoUpdateCheckEnv 置为真值即关闭使用中更新检查
	ArisClientNoUpdateCheckEnv = "ARIS_NO_UPDATE_CHECK"
	// ArisClientEnvValueTrueList 环境变量视为真值的取值（大小写不敏感）
	ArisClientEnvValueTrueList = "1,true,yes,on"
	// ArisClientEnvValueSeparator 真值列表分隔符
	ArisClientEnvValueSeparator = ","

	// ArisClientUpdateCheckTimeout 使用中检查（HEAD 取 tag）超时
	ArisClientUpdateCheckTimeout = 1500 * time.Millisecond
	// ArisClientUpdateNoticeWait 命令结束后等待提示的最长时间
	ArisClientUpdateNoticeWait = 400 * time.Millisecond
	// ArisClientUpdateDownloadTimeout 下载、校验与替换的整体超时
	ArisClientUpdateDownloadTimeout = 5 * time.Minute
	// ArisClientUpdateMaxArchiveBytes 归档下载与解包的体积上限
	ArisClientUpdateMaxArchiveBytes = 64 << 20

	// ArisClientUpdateTempPattern 自替换临时文件模板（与目标二进制同目录）
	ArisClientUpdateTempPattern = ".aris-update-*"
	// ArisClientUpdateBinaryMode 安装后的二进制权限
	ArisClientUpdateBinaryMode = 0o700

	// ArisClientDevVersion 本地构建的版本号（release 构建经 -ldflags -X main.version 覆盖）
	ArisClientDevVersion = "dev"
	// ArisClientVersionPrefixV 版本 tag 的 v 前缀
	ArisClientVersionPrefixV = "v"

	// ArisClientCommandTrace hook 高频命令，不参与使用中更新检查
	ArisClientCommandTrace = "trace"
	// ArisClientCommandUpdate 自更新命令，不参与使用中更新检查
	ArisClientCommandUpdate = "update"
	// ArisClientCommandVersion 版本命令，不参与使用中更新检查
	ArisClientCommandVersion = "version"
)

// aris 客户端更新提示与 aris update 输出文案
const (
	ArisClientUpdateAvailableFormat    = "! A new version %s is available (current %s) — run `aris update`"
	ArisClientUpdateUpToDateFormat     = "aris is up to date (%s)"
	ArisClientUpdateDoneFormat         = "Updated aris %s → %s (%s)"
	ArisClientUpdateDownloadingFormat  = "Downloading aris %s..."
	ArisClientUpdateVersionUnknownMessage     = "Failed to determine the latest aris version."
	ArisClientUpdateChecksumMessage           = "Checksum verification failed."
	ArisClientUpdateArchiveMemberMessage      = "Downloaded archive does not contain the aris binary."
	ArisClientUpdateUnsupportedPlatformFormat = "Unsupported platform: %s/%s"
)
```

- [ ] **Step 4: 实现 `internal/client/update/version.go`**

```go
// Package update 实现 aris 客户端的使用中更新检查与 aris update 自更新。
package update

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// parseVersion 解析 vX.Y.Z / X.Y.Z 版本号；缺省段补 0，含非数字段（如 dev、rc）即不可解析
func parseVersion(version string) ([3]int, bool) {
	var parts [3]int
	trimmed := strings.TrimPrefix(version, constant.ArisClientVersionPrefixV)
	if trimmed == "" {
		return parts, false
	}
	fields := strings.Split(trimmed, ".")
	if len(fields) > len(parts) {
		return parts, false
	}
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 0 {
			return parts, false
		}
		parts[index] = value
	}
	return parts, true
}

// IsNewer 判断 latest 是否比 current 更新；任一版本不可解析时返回 false（避免误判降级与预发布）
func IsNewer(latest, current string) bool {
	latestParts, latestOK := parseVersion(latest)
	currentParts, currentOK := parseVersion(current)
	if !latestOK || !currentOK {
		return false
	}
	return slices.Compare(latestParts[:], currentParts[:]) > 0
}

// IsUpToDate 判断 current 是否已不低于 latest；任一版本不可解析时返回 false
func IsUpToDate(latest, current string) bool {
	latestParts, latestOK := parseVersion(latest)
	currentParts, currentOK := parseVersion(current)
	if !latestOK || !currentOK {
		return false
	}
	return slices.Compare(currentParts[:], latestParts[:]) >= 0
}

// ShouldCheck 判断当前构建与终端环境是否参与使用中更新检查
func ShouldCheck(current string, interactive bool) bool {
	if !interactive || current == "" || current == constant.ArisClientDevVersion {
		return false
	}
	if _, ok := parseVersion(current); !ok {
		return false
	}
	return !envTrue(constant.ArisClientNoUpdateCheckEnv)
}

// envTrue 判断布尔语义环境变量是否为真值（大小写不敏感）
func envTrue(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return false
	}
	values := strings.Split(constant.ArisClientEnvValueTrueList, constant.ArisClientEnvValueSeparator)
	return slices.Contains(values, value)
}
```

- [ ] **Step 5: 让 `dev` 判定单一来源**

修改 `cmd/client/version.go`：把 `var version = "dev"` 改为 `var version = constant.ArisClientDevVersion`，并新增 import `"github.com/hcd233/aris-proxy-api/internal/common/constant"`。

- [ ] **Step 6: 跑测试确认通过**

Run: `cd .worktrees/client-update && go test -count=1 ./test/unit/client/update/`
Expected: PASS（`ok ... test/unit/client/update`）

- [ ] **Step 7: 提交**

```bash
rtk git add internal/common/constant/clientupdate.go internal/client/update/version.go test/unit/client/update/version_test.go cmd/client/version.go
rtk git commit -m "feat(client): 新增 aris 更新常量与版本比较判定"
```

---

### Task 2: 取最新 tag + 下载校验解包 + 原子自替换

**Files:**
- Create: `internal/client/update/update.go`
- Create: `test/unit/client/update/run_test.go`

**Interfaces:**
- Consumes: Task 1 的常量、`parseVersion`、`IsUpToDate`、`resolveBaseURL`（本任务内新增，供 Task 4 复用）
- Produces:
  - `update.Options{Current string; In io.Reader; Out io.Writer; BaseURL string; BinaryPath string; HTTPClient *http.Client}`
  - `update.Run(ctx context.Context, opts Options) error`
  - `update.Latest(ctx context.Context, base string, hc *http.Client) (string, error)`

- [ ] **Step 1: 写失败测试**

创建 `test/unit/client/update/run_test.go`：

```go
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// releaseRequests 记录本地 release 服务的请求次数
type releaseRequests struct {
	head atomic.Int32
	get  atomic.Int32
}

// tarGz 将单个 aris 二进制打包为 release 资产
func tarGz(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{
		Name: constant.ArisClientBinaryFileName,
		Mode: constant.ArisClientUpdateBinaryMode,
		Size: int64(len(binary)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// sha256Hex 计算归档摘要
func sha256Hex(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// newReleaseServer 本地 release 服务：HEAD 资产地址返回 302（Location 含 tag），GET 返回归档与摘要
func newReleaseServer(t *testing.T, latest string, archive []byte, checksum string) (*httptest.Server, *releaseRequests) {
	t.Helper()
	counts := &releaseRequests{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			counts.head.Add(1)
			writer.Header().Set("Location", "/releases/download/"+latest+request.URL.Path)
			writer.WriteHeader(http.StatusFound)
			return
		}
		counts.get.Add(1)
		if strings.HasSuffix(request.URL.Path, constant.ArisClientReleaseChecksumSuffix) {
			_, _ = writer.Write([]byte(checksum))
			return
		}
		_, _ = writer.Write(archive)
	}))
	t.Cleanup(server.Close)
	return server, counts
}

// writeTarget 在临时目录里放置一个"当前二进制"
func writeTarget(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aris")
	if err := os.WriteFile(path, []byte(content), constant.ArisClientUpdateBinaryMode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunReplacesBinaryWithLatestRelease(t *testing.T) {
	t.Parallel()
	newBinary := []byte("new-binary-content")
	archive := tarGz(t, newBinary)
	server, counts := newReleaseServer(t, "v9.9.9", archive, sha256Hex(t, archive))
	target := writeTarget(t, "old-binary-content")

	if err := update.Run(t.Context(), update.Options{
		Current:    "v0.1.0",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, newBinary) {
		t.Fatalf("installed binary = %q, want %q", installed, newBinary)
	}
	if counts.head.Load() != 1 || counts.get.Load() != 2 {
		t.Fatalf("requests = head %d, get %d, want head 1, get 2", counts.head.Load(), counts.get.Load())
	}
}

func TestRunSkipsDownloadWhenUpToDate(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("new-binary-content"))
	server, counts := newReleaseServer(t, "v9.9.9", archive, sha256Hex(t, archive))
	target := writeTarget(t, "current-binary-content")
	var out bytes.Buffer

	if err := update.Run(t.Context(), update.Options{
		Current:    "v9.9.9",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        &out,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if counts.get.Load() != 0 {
		t.Fatalf("download requests = %d, want 0", counts.get.Load())
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Fatalf("output = %q, want up-to-date notice", out.String())
	}
	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != "current-binary-content" {
		t.Fatalf("binary changed to %q", installed)
	}
}

func TestRunKeepsBinaryWhenChecksumMismatch(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("new-binary-content"))
	otherDigest := sha256Hex(t, []byte("other-content"))
	server, _ := newReleaseServer(t, "v9.9.9", archive, otherDigest)
	target := writeTarget(t, "current-binary-content")

	err := update.Run(t.Context(), update.Options{
		Current:    "v0.1.0",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want checksum failure")
	}
	installed, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(installed) != "current-binary-content" {
		t.Fatalf("binary changed to %q", installed)
	}
}

func TestRunKeepsBinaryWhenArchiveBroken(t *testing.T) {
	t.Parallel()
	archive := []byte("not-a-gzip-archive")
	server, _ := newReleaseServer(t, "v9.9.9", archive, sha256Hex(t, archive))
	target := writeTarget(t, "current-binary-content")

	err := update.Run(t.Context(), update.Options{
		Current:    "v0.1.0",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want archive failure")
	}
	installed, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(installed) != "current-binary-content" {
		t.Fatalf("binary changed to %q", installed)
	}
}

func TestLatestRejectsResponseWithoutRedirect(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	if _, err := update.Latest(t.Context(), server.URL, server.Client()); err == nil {
		t.Fatal("Latest() error = nil, want version resolution failure")
	}
}

func TestLatestReadsTagFromRedirect(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v9.9.9", nil, "")
	latest, err := update.Latest(t.Context(), server.URL, server.Client())
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if latest != "v9.9.9" {
		t.Fatalf("Latest() = %q, want v9.9.9", latest)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd .worktrees/client-update && go test -count=1 ./test/unit/client/update/ -run 'TestRun|TestLatest'`
Expected: FAIL —— `undefined: update.Run` / `undefined: update.Options` / `undefined: update.Latest`

- [ ] **Step 3: 实现 `internal/client/update/update.go`**

```go
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Options aris update 运行参数；BaseURL/BinaryPath/HTTPClient 为空时使用默认实现
type Options struct {
	Current    string
	In         io.Reader
	Out        io.Writer
	BaseURL    string
	BinaryPath string
	HTTPClient *http.Client
}

// Run 将当前 aris 二进制更新到最新 release；已是最新时只提示不下载
func Run(ctx context.Context, opts Options) error {
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	base := resolveBaseURL(opts.BaseURL)
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: constant.ArisClientUpdateDownloadTimeout}
	}

	latest, err := Latest(ctx, base, hc)
	if err != nil {
		return err
	}
	if IsUpToDate(latest, opts.Current) {
		_, err = fmt.Fprintf(out, constant.ArisClientUpdateUpToDateFormat+"\n", opts.Current)
		return err
	}

	var binary []byte
	title := fmt.Sprintf(constant.ArisClientUpdateDownloadingFormat, latest)
	err = ui.RunWithSpinner(in, out, title, func() error {
		archive, downloadErr := downloadAsset(ctx, base, hc)
		if downloadErr != nil {
			return downloadErr
		}
		if verifyErr := verifyChecksum(ctx, base, hc, archive); verifyErr != nil {
			return verifyErr
		}
		binary, downloadErr = extractBinary(archive)
		return downloadErr
	})
	if err != nil {
		return err
	}

	installed, err := replaceSelf(opts.BinaryPath, binary)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, constant.ArisClientUpdateDoneFormat+"\n", opts.Current, latest, installed)
	return err
}

// Latest 解析最新 release tag：HEAD 资产地址，从第一跳重定向地址中取出版本号
func Latest(ctx context.Context, base string, hc *http.Client) (string, error) {
	asset, err := assetName()
	if err != nil {
		return "", err
	}
	requestCtx, cancel := context.WithTimeout(ctx, constant.ArisClientUpdateCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodHead, base+"/"+asset, http.NoBody)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrBadRequest, err, "create latest release request")
	}
	var redirectURL *url.URL
	client := *hc
	client.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		redirectURL = next.URL
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrProxySend, err, "request latest release")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close

	tag := tagFromURL(redirectURL)
	if _, ok := parseVersion(tag); !ok {
		return "", ierr.New(ierr.ErrValidation, constant.ArisClientUpdateVersionUnknownMessage)
	}
	return tag, nil
}

// assetName 当前平台对应的 release 资产名；不支持的平台返回错误
func assetName() (string, error) {
	switch runtime.GOOS {
	case constant.ArisClientOSDarwin, constant.ArisClientOSLinux:
	default:
		return "", ierr.Newf(ierr.ErrValidation, constant.ArisClientUpdateUnsupportedPlatformFormat, runtime.GOOS, runtime.GOARCH)
	}
	switch runtime.GOARCH {
	case constant.ArisClientArchAMD64, constant.ArisClientArchARM64:
	default:
		return "", ierr.Newf(ierr.ErrValidation, constant.ArisClientUpdateUnsupportedPlatformFormat, runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf(constant.ArisClientReleaseAssetFormat, runtime.GOOS, runtime.GOARCH), nil
}

// tagFromURL 从 releases/download/<tag>/<asset> 形式的地址中取出版本 tag
func tagFromURL(target *url.URL) string {
	if target == nil {
		return ""
	}
	segments := strings.Split(target.Path, "/")
	index := slices.Index(segments, constant.ArisClientReleaseDownloadSegment)
	if index < 0 || index+1 >= len(segments) {
		return ""
	}
	return segments[index+1]
}

// downloadAsset 下载当前平台的 release 归档
func downloadAsset(ctx context.Context, base string, hc *http.Client) ([]byte, error) {
	asset, err := assetName()
	if err != nil {
		return nil, err
	}
	return getBody(ctx, hc, base+"/"+asset, constant.ArisClientUpdateMaxArchiveBytes)
}

// verifyChecksum 下载 .sha256 并比对归档摘要
func verifyChecksum(ctx context.Context, base string, hc *http.Client, archive []byte) error {
	asset, err := assetName()
	if err != nil {
		return err
	}
	checksumURL := base + "/" + asset + constant.ArisClientReleaseChecksumSuffix
	data, err := getBody(ctx, hc, checksumURL, constant.ArisClientUpdateMaxArchiveBytes)
	if err != nil {
		return err
	}
	expected := parseChecksum(string(data))
	if expected == "" {
		return ierr.New(ierr.ErrValidation, constant.ArisClientUpdateChecksumMessage)
	}
	sum := sha256.Sum256(archive)
	if !strings.EqualFold(expected, hex.EncodeToString(sum[:])) {
		return ierr.New(ierr.ErrValidation, constant.ArisClientUpdateChecksumMessage)
	}
	return nil
}

// parseChecksum 从 sha256 文件内容中取第一段合法摘要
func parseChecksum(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		digest, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		if len(digest) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(digest); err == nil {
			return digest
		}
	}
	return ""
}

// getBody 发起 GET 并读取响应体（超出 maxBytes 截断）
func getBody(ctx context.Context, hc *http.Client, target string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrBadRequest, err, "create release asset request")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxySend, err, "download release asset")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, ierr.Newf(ierr.ErrProxySend, "release asset rejected with status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxySend, err, "read release asset")
	}
	return data, nil
}

// extractBinary 从 tar.gz 归档中取出 aris 二进制
func extractBinary(archive []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "open release archive")
	}
	defer func() { _ = gzipReader.Close() }() //nolint:errcheck // best-effort close

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ierr.Wrap(ierr.ErrValidation, err, "read release archive")
		}
		if header.Name != constant.ArisClientBinaryFileName {
			continue
		}
		binary, err := io.ReadAll(io.LimitReader(tarReader, constant.ArisClientUpdateMaxArchiveBytes))
		if err != nil {
			return nil, ierr.Wrap(ierr.ErrValidation, err, "extract aris binary")
		}
		return binary, nil
	}
	return nil, ierr.New(ierr.ErrValidation, constant.ArisClientUpdateArchiveMemberMessage)
}

// replaceSelf 用 binary 原子替换目标二进制；targetPath 为空时解析当前可执行文件
func replaceSelf(targetPath string, binary []byte) (string, error) {
	if targetPath == "" {
		resolved, err := trace.ExecutablePath()
		if err != nil {
			return "", err
		}
		targetPath = resolved
	}
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, constant.ArisClientUpdateTempPattern)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "create update temporary file")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() //nolint:errcheck // best-effort cleanup

	if err := tmp.Chmod(constant.ArisClientUpdateBinaryMode); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return "", ierr.Wrap(ierr.ErrInternal, err, "secure update temporary file")
	}
	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return "", ierr.Wrap(ierr.ErrInternal, err, "write update temporary file")
	}
	if err := tmp.Close(); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "close update temporary file")
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "replace aris binary")
	}
	return targetPath, nil
}

// resolveBaseURL 解析更新源：显式参数 > ARIS_UPDATE_BASE_URL > 内置默认
func resolveBaseURL(base string) string {
	if base != "" {
		return strings.TrimRight(base, "/")
	}
	if fromEnv := strings.TrimSpace(os.Getenv(constant.ArisClientUpdateBaseURLEnv)); fromEnv != "" {
		return strings.TrimRight(fromEnv, "/")
	}
	return constant.ArisClientReleaseBaseURL
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd .worktrees/client-update && go test -count=1 ./test/unit/client/update/`
Expected: PASS（Task 1 与 Task 2 的用例全绿）

- [ ] **Step 5: 提交**

```bash
rtk git add internal/client/update/update.go test/unit/client/update/run_test.go
rtk git commit -m "feat(client): 实现 aris 自更新下载校验与原子替换"
```

---

### Task 3: `aris update` 命令 + E2E

**Files:**
- Create: `cmd/client/update.go`
- Modify: `cmd/client/root.go`（`AddCommand`）
- Create: `test/e2e/clientcmd/update_test.go`（复用同包既有 `buildClient` / `projectRoot`）

**Interfaces:**
- Consumes: `update.Run`、`update.Options`（Task 2）；`version`（`cmd/client/version.go`）
- Produces: `aris update` 子命令（`Args: cobra.NoArgs`）

- [ ] **Step 1: 写失败测试**

创建 `test/e2e/clientcmd/update_test.go`：

```go
package clientcmd_e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// releaseArchive 将二进制打包为 release 资产并返回归档与摘要
func releaseArchive(t *testing.T, binary []byte) (archive []byte, checksum string) {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{Name: constant.ArisClientBinaryFileName, Mode: constant.ArisClientUpdateBinaryMode, Size: int64(len(binary))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	archive = buffer.Bytes()
	sum := sha256.Sum256(archive)
	return archive, hex.EncodeToString(sum[:])
}

// newReleaseServer 本地 release 服务：HEAD 资产地址返回 302（含 tag），GET 返回归档与摘要
func newReleaseServer(t *testing.T, latest string, archive []byte, checksum string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			writer.Header().Set("Location", "/releases/download/"+latest+request.URL.Path)
			writer.WriteHeader(http.StatusFound)
			return
		}
		if strings.HasSuffix(request.URL.Path, constant.ArisClientReleaseChecksumSuffix) {
			_, _ = writer.Write([]byte(checksum))
			return
		}
		_, _ = writer.Write(archive)
	}))
	t.Cleanup(server.Close)
	return server
}

// runClient 以指定更新源执行 aris 子命令，返回 stdout+stderr 与错误
func runClient(t *testing.T, binary, baseURL string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, args...)
	cmd.Env = append(os.Environ(), constant.ArisClientUpdateBaseURLEnv+"="+baseURL)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestUpdateCommand_InstallsLatestRelease(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	binary := buildClient(t, root, "-X main.version=v0.1.0")
	newBinary, err := os.ReadFile(buildClient(t, root, "-X main.version=v9.9.9"))
	if err != nil {
		t.Fatal(err)
	}
	archive, checksum := releaseArchive(t, newBinary)
	server := newReleaseServer(t, "v9.9.9", archive, checksum)

	output, err := runClient(t, binary, server.URL, constant.ArisClientCommandUpdate)
	if err != nil {
		t.Fatalf("aris update: %v\n%s", err, output)
	}
	if !strings.Contains(output, "v9.9.9") {
		t.Fatalf("aris update output = %q, want new version", output)
	}

	versionOutput, err := runClient(t, binary, server.URL, constant.ArisClientCommandVersion)
	if err != nil {
		t.Fatalf("aris version: %v\n%s", err, versionOutput)
	}
	if strings.TrimSpace(versionOutput) != "v9.9.9" {
		t.Fatalf("aris version = %q, want v9.9.9", strings.TrimSpace(versionOutput))
	}
}

func TestUpdateCommand_KeepsBinaryOnChecksumMismatch(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	binary := buildClient(t, root, "-X main.version=v0.1.0")
	brokenArchive, _ := releaseArchive(t, []byte("broken-binary-content"))
	server := newReleaseServer(t, "v9.9.9", brokenArchive, strings.Repeat("0", 64))

	output, err := runClient(t, binary, server.URL, constant.ArisClientCommandUpdate)
	if err == nil {
		t.Fatalf("aris update should fail on checksum mismatch, output = %q", output)
	}

	versionOutput, err := runClient(t, binary, server.URL, constant.ArisClientCommandVersion)
	if err != nil {
		t.Fatalf("aris version: %v\n%s", err, versionOutput)
	}
	if strings.TrimSpace(versionOutput) != "v0.1.0" {
		t.Fatalf("aris version = %q, want unchanged v0.1.0", strings.TrimSpace(versionOutput))
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd .worktrees/client-update && go test -count=1 -run TestUpdateCommand ./test/e2e/clientcmd/`
Expected: FAIL —— `aris update` 未注册，输出 `unknown command "update"`，退出码非 0

- [ ] **Step 3: 实现命令壳并注册**

创建 `cmd/client/update.go`：

```go
package main

import (
	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update aris to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return update.Run(cmd.Context(), update.Options{
				Current: version,
				In:      cmd.InOrStdin(),
				Out:     cmd.OutOrStdout(),
			})
		},
	}
}
```

修改 `cmd/client/root.go` 的 `newRootCommand()`，在 `newVersionCommand()` 之后加一行：

```go
	root.AddCommand(newUpdateCommand())
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd .worktrees/client-update && go test -count=1 ./test/e2e/clientcmd/`
Expected: PASS（含既有 `version_test.go` 回归）

- [ ] **Step 5: 提交**

```bash
rtk git add cmd/client/update.go cmd/client/root.go test/e2e/clientcmd/update_test.go
rtk git commit -m "feat(client): 新增 aris update 命令与自升级 E2E"
```

---

### Task 4: 使用时后台检查更新并提示

**Files:**
- Create: `internal/client/update/check.go`
- Modify: `internal/client/update/version.go`（新增 `ShouldCheckCommand`）
- Modify: `cmd/client/root.go`（`execute()` 接线）
- Create: `test/unit/client/update/check_test.go`
- Modify: `test/unit/client/update/version_test.go`（追加 `TestShouldCheckCommand`）

**Interfaces:**
- Consumes: `resolveBaseURL`、`Latest`、`IsNewer`（Task 1/2）；`constant.ArisClientCommand*`、`ArisClientUpdateNoticeWait`、`ArisClientUpdateCheckTimeout`
- Produces:
  - `update.CheckOptions{Current string; Out io.Writer; BaseURL string; HTTPClient *http.Client}`
  - `update.StartCheck(ctx context.Context, opts CheckOptions) func()`
  - `update.ShouldCheckCommand(commandPath []string) bool`
  - `cmd/client.startUpdateCheck(root *cobra.Command) func()`、`cmd/client.isUpdateCheckCommand(root *cobra.Command) bool`、`cmd/client.isStderrTerminal() bool`

- [ ] **Step 1: 写失败测试**

创建 `test/unit/client/update/check_test.go`：

```go
package update

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
)

func TestStartCheckPrintsNoticeForNewerRelease(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v9.9.9", nil, "")
	var out bytes.Buffer

	wait := update.StartCheck(t.Context(), update.CheckOptions{
		Current: "v0.1.0",
		Out:     &out,
		BaseURL: server.URL,
	})
	wait()

	if !strings.Contains(out.String(), "v9.9.9") || !strings.Contains(out.String(), "v0.1.0") {
		t.Fatalf("notice = %q, want latest and current version", out.String())
	}
}

func TestStartCheckIsSilentWhenUpToDate(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v0.2.2", nil, "")
	var out bytes.Buffer

	wait := update.StartCheck(t.Context(), update.CheckOptions{
		Current: "v0.2.2",
		Out:     &out,
		BaseURL: server.URL,
	})
	wait()

	if out.Len() != 0 {
		t.Fatalf("notice = %q, want empty output", out.String())
	}
}

func TestStartCheckIsSilentOnFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	var out bytes.Buffer

	wait := update.StartCheck(t.Context(), update.CheckOptions{
		Current: "v0.1.0",
		Out:     &out,
		BaseURL: server.URL,
	})
	wait()

	if out.Len() != 0 {
		t.Fatalf("notice = %q, want empty output", out.String())
	}
}
```

在 `test/unit/client/update/version_test.go` 末尾追加：

```go
func TestShouldCheckCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		commandPath []string
		expected    bool
	}{
		{name: "status participates", commandPath: []string{"status"}, expected: true},
		{name: "init participates", commandPath: []string{"init"}, expected: true},
		{name: "model export participates", commandPath: []string{"model", "export"}, expected: true},
		{name: "trace ingest skipped", commandPath: []string{constant.ArisClientCommandTrace, "ingest"}, expected: false},
		{name: "trace install skipped", commandPath: []string{constant.ArisClientCommandTrace, "install"}, expected: false},
		{name: "update skipped", commandPath: []string{constant.ArisClientCommandUpdate}, expected: false},
		{name: "version skipped", commandPath: []string{constant.ArisClientCommandVersion}, expected: false},
		{name: "empty path skipped", commandPath: nil, expected: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := update.ShouldCheckCommand(testCase.commandPath); got != testCase.expected {
				t.Fatalf("ShouldCheckCommand(%v) = %v, want %v", testCase.commandPath, got, testCase.expected)
			}
		})
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd .worktrees/client-update && go test -count=1 ./test/unit/client/update/`
Expected: FAIL —— `undefined: update.StartCheck` / `undefined: update.ShouldCheckCommand`

- [ ] **Step 3: 实现 `check.go` 与 `ShouldCheckCommand`**

创建 `internal/client/update/check.go`：

```go
package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// CheckOptions 使用中更新检查参数；Out/BaseURL/HTTPClient 为空时使用默认实现
type CheckOptions struct {
	Current    string
	Out        io.Writer
	BaseURL    string
	HTTPClient *http.Client
}

// StartCheck 后台解析最新版本，返回等待并打印提示的函数；任何错误静默，等待上限 ArisClientUpdateNoticeWait
func StartCheck(ctx context.Context, opts CheckOptions) func() {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: constant.ArisClientUpdateCheckTimeout}
	}
	base := resolveBaseURL(opts.BaseURL)
	checkCtx, cancel := context.WithTimeout(ctx, constant.ArisClientUpdateCheckTimeout)

	result := make(chan string, 1)
	go func() {
		latest, err := Latest(checkCtx, base, hc)
		if err != nil {
			result <- ""
			return
		}
		result <- latest
	}()

	return func() {
		defer cancel()
		timer := time.NewTimer(constant.ArisClientUpdateNoticeWait)
		defer timer.Stop()
		select {
		case latest := <-result:
			if IsNewer(latest, opts.Current) {
				_, _ = fmt.Fprintf(out, constant.ArisClientUpdateAvailableFormat+"\n", latest, opts.Current) //nolint:errcheck // best-effort notice
			}
		case <-timer.C:
		}
	}
}
```

在 `internal/client/update/version.go` 末尾追加：

```go
// ShouldCheckCommand 判断命令路径是否参与使用中更新检查：hook 高频命令、自更新与版本命令不参与
func ShouldCheckCommand(commandPath []string) bool {
	if len(commandPath) == 0 {
		return false
	}
	for _, name := range commandPath {
		switch name {
		case constant.ArisClientCommandTrace, constant.ArisClientCommandUpdate, constant.ArisClientCommandVersion:
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd .worktrees/client-update && go test -count=1 ./test/unit/client/update/`
Expected: PASS

- [ ] **Step 5: 接线到客户端入口**

修改 `cmd/client/root.go`：

```go
package main

import (
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "aris",
		Short:         "Aris client",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// 与 version 子命令输出保持一致：裸版本字符串。
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newInitCommand())
	root.AddCommand(newStatusCommand())
	root.AddCommand(newTraceCommand())
	root.AddCommand(newModelCommand())
	root.AddCommand(newVersionCommand())
	root.AddCommand(newUpdateCommand())
	return root
}

func execute() error {
	root := newRootCommand()
	finishCheck := startUpdateCheck(root)
	err := root.Execute()
	finishCheck()
	return err
}

// startUpdateCheck 对符合条件的交互式命令启动一次后台更新检查；返回等待并打印提示的函数
func startUpdateCheck(root *cobra.Command) func() {
	if !update.ShouldCheck(version, isStderrTerminal()) || !isUpdateCheckCommand(root) {
		return func() {}
	}
	return update.StartCheck(root.Context(), update.CheckOptions{Current: version, Out: os.Stderr})
}

// isUpdateCheckCommand 判断本次调用的命令是否参与更新检查（裸 aris 与 --version 不参与）
func isUpdateCheckCommand(root *cobra.Command) bool {
	target, _, err := root.Find(os.Args[1:])
	if err != nil || target == root {
		return false
	}
	return update.ShouldCheckCommand(strings.Fields(target.CommandPath())[1:])
}

// isStderrTerminal 判断 stderr 是否为交互终端（非交互场景不发起网络检查）
func isStderrTerminal() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}
```

- [ ] **Step 6: 跑测试与构建确认通过**

Run: `cd .worktrees/client-update && go test -count=1 ./test/unit/client/update/ && go build ./cmd/client`
Expected: PASS 且构建无输出

- [ ] **Step 7: 提交**

```bash
rtk git add internal/client/update/check.go internal/client/update/version.go cmd/client/root.go test/unit/client/update/check_test.go test/unit/client/update/version_test.go
rtk git commit -m "feat(client): 使用时后台检查更新并提示"
```

---

### Task 5: 文档同步与整体验证

**Files:**
- Modify: `CONTEXT.md`（`ArisClient` 词条）
- Modify: `README.md`（客户端 CLI 清单，141 行附近）

**Interfaces:**
- Consumes: 前四个任务的产物（命令与行为已定稿）
- Produces: 文档与最终验证证据（无新代码接口）

- [ ] **Step 1: 更新 `CONTEXT.md` 的 `ArisClient` 词条**

在 `CONTEXT.md` 的 `**ArisClient（Aris 客户端）**:` 词条里：命令清单补 `update`（把自身升级到最新 release：HEAD `releases/latest/download` 取 tag → 下载 `aris-<os>-<arch>.tar.gz` → 校验 sha256 → 同目录临时文件 + rename 原子替换自身），并补一句使用中更新检查语义（符合条件时后台异步解析最新版本，命令结束后最多等 400ms 在 stderr 提示一行；仅 stderr 为 TTY 的非 `trace`/`update`/`version` 命令参与；`ARIS_NO_UPDATE_CHECK` 关闭、`ARIS_UPDATE_BASE_URL` 覆盖更新源；不做缓存、不自动安装）。

- [ ] **Step 2: 更新 `README.md` 的客户端 CLI 清单**

把 `README.md:141` 的 `Trace 客户端 CLI：aris init、aris model export、aris trace ingest、aris trace install、aris status（cmd/client）` 改为在末尾补 `、aris update`。

- [ ] **Step 3: 全量验证**

Run:

```bash
cd .worktrees/client-update && go test -count=1 ./test/unit/client/... ./test/e2e/clientcmd/... && make lint
```

Expected: 测试全 PASS；`make lint` 输出 `lint conv` 与静态检查均通过（无 error/warning）

- [ ] **Step 4: 冒烟（真实 GitHub，可选中途跳过）**

Run: `cd .worktrees/client-update && make build-client && ./aris version && ./aris update`
Expected: `./aris version` 输出 `dev`；`./aris update` 输出 `Updated aris dev → <最新 tag> (<路径>)`（会把仓库根目录的 dev 构建替换为 release 产物，属预期；需要还原时重跑 `make build-client`）。若本地网络不通 GitHub，记录实际报错并在汇报中说明，跳过此步。

- [ ] **Step 5: 提交**

```bash
rtk git add CONTEXT.md README.md
rtk git commit -m "docs: 同步 aris update 与使用中更新检查说明"
```

- [ ] **Step 6: 清理构建产物**

Run: `cd .worktrees/client-update && rm -f aris && rm -rf build`（worktree 只保留源码，避免磁盘膨胀）

---

## 自审结论

- **Spec 覆盖**：更新源与探测方式（Task 2 `Latest`）、无缓存（全计划无状态文件）、跳过清单（Task 1 `ShouldCheck` + Task 4 `ShouldCheckCommand`/`isStderrTerminal`）、提示（Task 4 `StartCheck`）、`aris update` 全流程与 sha256（Task 2 `Run`/`verifyChecksum`）、原子替换（Task 2 `replaceSelf`）、命令壳（Task 3）、测试三层（Task 1/2/4 单测 + Task 3 E2E）、文档同步（Task 5）、YAGNI 清单（未出现 `--force`/`--check`/服务端接口/缓存）、已知限制（E2E 不覆盖 TTY 提示，已在 spec 标注并在 Task 4 单测覆盖）。
- **占位符扫描**：无 TBD/TODO，所有代码块为可直接落盘的完整实现，所有命令给出预期结果。
- **类型一致性**：`Options`/`CheckOptions` 字段、`Latest`/`Run`/`StartCheck`/`ShouldCheck`/`ShouldCheckCommand`/`IsNewer`/`IsUpToDate` 签名在任务间一致；`test/unit/client/update` 用外部测试包 `package update`（沿用 `test/unit/client/status` 既有先例），因此只依赖上述导出符号。
- **偏离说明**：`ARIS_NO_UPDATE_CHECK` 真值除 spec 所列 `1/true/yes` 外额外接受 `on`（常量 `ARIS_CLIENT` 真值列表），与 spec 无冲突。
