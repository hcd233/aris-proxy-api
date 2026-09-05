# aris version 子命令 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `aris version` / `aris --version` 输出版本号；release 构建注入 tag，本地构建默认 `dev`。

**Architecture:** `cmd/client/version.go` 持有注入点 `var version = "dev"`，Makefile 经 `CLIENT_LDFLAGS` 注入，release workflow 传 `GITHUB_REF_NAME`；测试在 e2e 内自建二进制（默认 + 注入两连击）闭环验证。

**Tech Stack:** cobra、go build -ldflags -X、标准库 testing。

**Spec:** `docs/superpowers/specs/2026-09-06-aris-version-command-design.md`

## Global Constraints

- commands.md：新增 cobra 命令需用户明确要求（已满足）；编辑 Go 前跑 use-modern-go `list`；加载 `golang-spf13-cobra` / `golang-naming` / `golang-code-style`。
- 测试只用标准库 testing；`*_test.go` 只放 `test/unit|e2e/<topic>/`；exec 用 `CommandContext`。
- 每 commit 前 `git rev-parse -q --verify MERGE_HEAD` 必须为空；master 直开；只本地提交，push 仅限特性分支（PR）。

---

### Task 1: version 子命令（TDD）

**Files:**
- Create: `test/e2e/clientcmd/version_test.go`
- Create: `cmd/client/version.go`
- Modify: `cmd/client/root.go:12-15`（AddCommand + root.Version）
- Modify: `Makefile`（`VERSION ?= dev`、`CLIENT_LDFLAGS`、两个 client 目标）
- Modify: `.github/workflows/release-client.yml:23`（`VERSION=${GITHUB_REF_NAME} make build-client-all`）

**Interfaces:**
- Produces: `main.version`（ldflags 注入点，默认 `"dev"`）；`aris version` / `aris --version` 打印该值。

- [ ] **Step 1: 写失败测试** `test/e2e/clientcmd/version_test.go`：helper 用 `go build`（默认与 `-ldflags "-X main.version=v9.9.9"`）产出两个临时二进制，断言 `version` 子命令输出分别为 `dev`、`v9.9.9`，且 `--version` 与 `version` 输出一致。
- [ ] **Step 2:** `go test -count=1 ./test/e2e/clientcmd/ -v` → FAIL（命令不存在，子命令报 unknown command）。
- [ ] **Step 3:** 实现 `cmd/client/version.go` + `root.go` 注册 + `root.Version = version`。
- [ ] **Step 4:** `go test -count=1 ./test/e2e/clientcmd/ -v` → PASS。
- [ ] **Step 5:** Makefile：`VERSION ?= dev`；`CLIENT_LDFLAGS := $(LDFLAGS) -X main.version=$(VERSION)`；`build-client`/`build-client-all` 的 `-ldflags="$(LDFLAGS)"` 改为 `-ldflags="$(CLIENT_LDFLAGS)"`；`release-client.yml` 构建行加 `VERSION=${GITHUB_REF_NAME}` 前缀。
- [ ] **Step 6:** 冒烟 `make build-client && ./aris version && ./aris --version` → 均 `dev`；lint `make lint` 通过。
- [ ] **Step 7:** 沉淀 serena memory；提交 `feat(client): 新增 aris version 子命令并注入 release 版本号`。

### 收尾

- [ ] 分支 `feature/aris-version-command-2026-09-06`，rebase origin/master 线性化，push + `gh pr create`（正文含验证证据与「合并后下次打 tag 生效」说明）。
