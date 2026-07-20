# 仓库与 CI

> **使用场景**：涉及 git 操作、CI workflow、K8s 部署、Git 分支管理、周期性技术债清理时加载。

## CI 与仓库

- `.github/workflows/docker-publish.yml` 在推送到 `master`、`v*.*.*` tag、PR 到 `master` 和定时任务时构建多架构 GHCR 镜像。
- 影响镜像构建的 path filter 包含 `internal/**`、`docker/**`、`cmd/**`、`go.mod`、`go.sum`。
- 本地 hook 可通过 `bash .githooks/setup.sh` 安装；除非用户明确要求，不要绕过 hook。
- 使用 `.worktrees/` 作为 git worktree 目录。
- `AGENTS.md`、`CLAUDE.md`、`CODEBUDDY.md` 是项目级持久规范，修改其中一个时保持同步。
- 编写文档必须使用中文

## K8s 部署

- Deployment：`k8s/deployment.yaml`，副本数 2，`maxUnavailable: 0` 蓝绿更新。
- 优雅关闭：`terminationGracePeriodSeconds: 660`（11 分钟），`preStop: sleep 10` 等待 `/ready` 探针失效；应用内部 8 步关闭（`cmd/server/server.go`），超时后强制退出。
- 存活探针：`GET /health`（15s 初始延迟，20s 间隔，失败 3 次重启）。
- 就绪探针：`GET /ready`（5s 初始延迟，10s 间隔，失败 6 次下线），draining 期间返回 503。

## Git 分支规范

- 任何开发任务都必须先在 `.worktrees/` 下创建或切换 git worktree，并在该 worktree 上 checkout 分支进行开发，禁止直接在主工作区开发。开发完成后询问用户是否需要提mr或者直接合并到master，禁止擅自操作
- 分支命名规范：`{feature|bugfix|refactor|chore|docs|test|hotfix}/{5个以内小写英文单词描述功能或修复，使用连字符}-{当前 datetime，例如 2026-05-28}`。
- 分支示例：`feature/session-share-2026-05-28`、`bugfix/token-expiry-2026-05-28`、`refactor/split-endpoint-model-2026-05-28`。

## 周期性技术债清理

- **全仓库过度工程扫描**：使用 `ponytail-audit` 扫描整个代码库，按"能删多少行"排名输出一次性报告。标签：`delete`（死代码）、`stdlib`（标准库已有）、`native`（平台原生已有）、`yagni`（单实现抽象）、`shrink`（同等逻辑更少行）。只列不改。适用于定期清理技术债，非日常开发流程。
- **ponytail 债务台账**：使用 `ponytail-debt` 收集代码库中所有 `// ponytail:` 注释形成债务台账，每条列出简化内容、上限和升级触发条件。标记无升级路径的 shortcut 为 `no-trigger`（容易静默腐烂的项）。适用于定期审查刻意延迟的 shortcut。
