# 仓库与 CI

> **使用场景**：涉及仓库管理、CI workflow、K8s 部署、周期性技术债清理时加载。

## CI 与仓库

- `.github/workflows/lint.yml`：PR 到 `master` 与 push `master` 时触发（无 path 过滤）。包含 `go-lint`（golangci-lint v2.12.2，配置 `.golangci.yml`）与 `web-lint`（ESLint + Prettier check）。这两个 check 是 master 分支保护的 required check，**PR 必须全部通过才能合并**（admin 直推不受限）。
- `.github/workflows/docker-publish.yml`：push `master`（path filter 过滤）、`v*.*.*` tag、定时任务（每日 cron 保温缓存）、workflow_dispatch 触发。结构为 build → publish 两步：build job 在 runner 上增量编译 Go（GOMODCACHE+GOCACHE 单缓存 + web/out 产物缓存），镜像经 dockerfile 的 `binary-prebuilt` 阶段直接打包二进制（amd64 单平台，生产为单节点 amd64 k3s），push 后**后台 SSH 预热拉取镜像到生产节点**（跨境带宽慢，与 publish 启动并行）；publish job 以 **digest 引用**（`DEPLOY_IMAGE`，IfNotPresent）SSH 到生产执行 `script/deploy-k8s.sh` 滚动部署，手动部署回退 master tag + Always。**PR 不构建镜像**。稳态全链路（构建到上线）实测 ~90s。
- 影响构建的 path filter 包含 `internal/**`、`docker/**`、`cmd/**`、`web/**`、`k8s/**`、`script/**`、workflow 自身、`go.mod`、`go.sum`。
- 本地 hook 可通过 `bash .githooks/setup.sh` 安装；除非用户明确要求，不要绕过 hook。
- `AGENTS.md`、`CLAUDE.md`、`CODEBUDDY.md` 是项目级持久规范，修改其中一个时保持同步。
- 编写文档必须使用中文

## K8s 部署

- Deployment：`k8s/deployment.yaml`，副本数 2，`maxUnavailable: 0` 蓝绿更新。
- 优雅关闭：`terminationGracePeriodSeconds: 660`（11 分钟），`preStop: sleep 10` 等待 `/ready` 探针失效；应用内部 8 步关闭（`cmd/server/server.go`），超时后强制退出。
- 存活探针：`GET /health`（15s 初始延迟，20s 间隔，失败 3 次重启）。
- 就绪探针：`GET /ready`（5s 初始延迟，10s 间隔，失败 6 次下线），draining 期间返回 503。

## 周期性技术债清理

- **全仓库过度工程扫描**：使用 `ponytail-audit` 扫描整个代码库，按"能删多少行"排名输出一次性报告。标签：`delete`（死代码）、`stdlib`（标准库已有）、`native`（平台原生已有）、`yagni`（单实现抽象）、`shrink`（同等逻辑更少行）。只列不改。适用于定期清理技术债，非日常开发流程。
- **ponytail 债务台账**：使用 `ponytail-debt` 收集代码库中所有 `// ponytail:` 注释形成债务台账，每条列出简化内容、上限和升级触发条件。标记无升级路径的 shortcut 为 `no-trigger`（容易静默腐烂的项）。适用于定期审查刻意延迟的 shortcut。
