# aris-proxy-api → aris-mass 品牌升级设计

- 日期：2026-11-02
- 范围：仓库全量技术改名 + 本次一并切换生产
- 展示品牌：Aris Mass

## 1. 背景与目标

项目对外品牌名为 `aris-proxy-api`，但它已经是「MaaS 模型聚合网关 + 使用数据沉淀平台」，`proxy-api` 无法覆盖当前定位。本次把项目身份统一为 `aris-mass`，并完成对应的生产切换。

成功标准：

1. 仓库内不再以 `aris-proxy-api` 作为项目身份（历史设计文档除外）。
2. Go module、二进制、GHCR 镜像、K8s 资源、客户端更新源、Web 展示品牌全部指向 `aris-mass` / `Aris Mass`。
3. 生产环境在本次发布后运行于 `aris-mass` namespace，`https://api.lvlvko.top` 与 `/web/` 正常。
4. 构建、单测、lint、Web lint/test/build 全部通过。

## 2. 命名映射

| 类型 | 旧值 | 新值 | 是否用户可见 |
| --- | --- | --- | --- |
| Go module | `github.com/hcd233/aris-proxy-api` | `github.com/hcd233/aris-mass` | 否 |
| 服务端二进制 | `aris-proxy-api` | `aris-mass` | 否 |
| CLI 二进制 | `aris` | 不变 | 是 |
| GHCR 镜像 | `ghcr.io/hcd233/aris-proxy-api` | `ghcr.io/hcd233/aris-mass` | 否 |
| GitHub 仓库 | `hcd233/aris-proxy-api` | `hcd233/aris-mass` | 是 |
| K8s namespace / Deployment / Service | `aris-proxy-api` | `aris-mass` | 否 |
| K8s ConfigMap / Secret | `aris-proxy-api-config` / `-secret` | `aris-mass-config` / `-secret` | 否 |
| 生产目录 | `/home/ubuntu/code/aris-proxy-api` | `/home/ubuntu/code/aris-mass`（旧路径保留软链） | 否 |
| 展示品牌 | `Aris Proxy` / `Aris Proxy API` | `Aris Mass` / `Aris Mass API` | 是 |
| 产品短名 | `Aris` | 不变（CLI、`~/.aris`、`ARIS_*`） | 是 |
| JWT issuer / audience | `aris-proxy-api` | `aris-mass` | 间接 |
| Metrics service 名 | `aris-proxy-api` | `aris-mass` | 否 |
| 日志文件名 | `aris-proxy-api*.log` | `aris-mass*.log` | 否 |
| Codex provider 展示名 | `Aris Proxy` | `Aris Mass` | 是 |
| OpenAPI title | `Aris API Tmpl` | `Aris Mass API` | 是 |
| dev compose 数据库名 | `aris-proxy-api` | `aris-mass` | 否 |

明确不改：

- 域名 `api.lvlvko.top`、Web 路径 `/web/`、API 路由前缀（`/api/web/v1`、`/api/cli/v1`、`/api/openai/v1`、`/api/anthropic/v1`）。
- CLI 品牌 `aris`、`~/.aris`、`ARIS_*` 环境变量、`~/.dsh` 中的 `ARIS_API_KEY` 引用。
- `docs/superpowers/` 下 104 篇带日期的历史设计/计划文档（保持当时的决策记录）。

兼容迁移：

- `ClientModelProviderID` 已改为 `aris-mass`，旧值 `aris-proxy` 保留为 `ClientModelLegacyProviderID`：导出时把旧 provider 整体迁移到新键（两侧并存时按 id 合并用户自加模型），Codex 导出时清理旧 `[model_providers."aris-proxy"]` 段。用户需要升级到新的 `aris` release 才会拿到迁移后的写入行为。

## 3. 改动清单

### 3.1 Go 代码（约 431 个 `internal/**` + 298 个 `test/**` 文件）

- 全量替换 module 路径：`github.com/hcd233/aris-proxy-api` → `github.com/hcd233/aris-mass`（import 为主）。
- `internal/common/constant/string.go`：`ProjectName`。
- `internal/common/constant/metrics.go`：`MetricServiceName`。
- `internal/common/constant/log.go`：三个日志文件名。
- `internal/common/constant/convcheck.go`：lintconv 的 module 路径常量。
- `internal/common/constant/clientupdate.go`：`ArisClientReleaseBaseURL`。
- `internal/common/constant/clientmodel.go`：`ClientModelLabelArisProxy` → `ClientModelLabelArisMass`，值改 `Aris Mass`；`ClientModelProviderID` 改 `aris-mass`，新增 `ClientModelLegacyProviderID = "aris-proxy"`。
- `internal/client/model/targets.go` / `text_targets.go`：OpenCode / Pi / Codex 导出时迁移旧 provider 配置并保留用户自加模型。
- `internal/handler/install_aris_client.sh.tmpl`：`gh_base` 与文件头注释。
- `internal/common/constant/string.go`：`APITitle` / `APIDescription`。
- `cmd/server/main.go`、`cmd/lint/main.go`：包注释。
- `test/unit/lintconv/lintconv_test.go`、`test/unit/architecture_refactor`、E2E fixture 中的旧名。

### 3.2 构建与部署

- `Makefile`：`APP_NAME`、注释。
- `docker/dockerfile`：构建产物、`COPY`、`ENTRYPOINT`、注释。
- `docker/docker-compose-single.yml` / `-dev-single.yml` / `-full.yml`：service、container_name、image、command、DB 健康检查。
- `k8s/*.yaml`：namespace、deployment、service、configmap、secret 的名称与二进制路径。
- `script/deploy-k8s.sh`：`NAMESPACE` / `APP_NAME` / 镜像回退 / migration job command / canary 标签；新增旧命名空间退役逻辑（见 §4）。
- `script/deploy.sh`、`script/deploy-dev.sh`：镜像、容器名、日志。
- `.github/workflows/build-and-publish.yml`：二进制输出名；镜像构建增加 OCI source label（见 §4.3）。
- `.vscode/launch.json`：三个启动配置名。
- `.gitignore`、`.dockerignore`：二进制名。
- `.golangci.yml`：depguard 的 module 路径。
- 根 `package-lock.json`：`name`。

### 3.3 Web 前端

- `web/src/app/layout.tsx`：metadata title/description。
- `web/src/app/login/page.tsx`、`web/src/app/share/page.tsx`：展示名 / aria-label。登录页大标题用两个 `SplitText` 拆成 `Aris` / `Proxy` 两行，`Aris Proxy` 不是连续字符串，批量替换会漏掉，必须单独改为 `Aris` / `Mass`。
- `web/src/components/portal/portal-closing.tsx`：品牌名与 GitHub 地址。
- `web/src/lib/portal.ts`、`web/src/lib/portal.test.ts`：`GITHUB_REPO_URL`。
- `web/src/locales/{zh,en,ja}.json`：`auth.login_title`、`layout.aris_proxy` → `layout.aris_mass`，同步 `web/src/app/(dashboard)/layout.tsx` 的 key。

### 3.4 活文档与工程资产

- `README.md` / `README.zh-CN.md`：标题 `Aris Mass`、clone 地址。
- `CONTEXT.md`：标题与项目名引用。
- `docs/agents/*.md`、`AGENTS.md`（如含旧名）、`.agents/skills/**`（含 `internal/**` 与 one 处 `external/shadcn`）：项目名、生产目录、namespace、镜像名。
- `docs/diagrams/aris-proxy-api-architecture.html` → `git mv` 为 `aris-mass-architecture.html`（当前无引用）。
- `docs/diagrams/archify/*.architecture.json`：`meta.repository.url` 与节点标签；重新生成受影响的 HTML（GIF 如工具链可用则一并重生成，否则在交付说明中标注）。
- `docs/loadtest/BENCHMARK.md`、`docs/openai/*`、`docs/anthropic/*`：示例路径与引用。

## 4. 生产切换设计

### 4.1 为什么需要专门的切换逻辑

生产 Service 是 `LoadBalancer:18080`，k3s 的 svclb 以 hostPort 占用节点 18080；旧 Service 不删除，新 namespace 的 Service 无法绑定同一端口。而 `deploy-k8s.sh` 的健康检查打的是 `127.0.0.1:18080/health`——如果先起新 Service 后删旧 Service，健康检查会打到旧服务，产生假成功。

### 4.2 deploy-k8s.sh 调整后的顺序

1. 创建新 namespace（`aris-mass`），同步 ConfigMap / Secret（来源仍是 `env/api.env`）。
2. 运行数据库 migration Job（新镜像，无 schema 变更）。
3. apply 新 Deployment，等待 rollout 完成（readiness 通过；此时尚未接流量）。
4. 旧工作负载退役：`kubectl -n aris-proxy-api scale deployment aris-proxy-api --replicas=0`（保留 Deployment/ConfigMap/Secret 作为回滚目标），再删除旧 Service；轮询 kube-system 中 `svccontroller.k3s.cattle.io/svcname=aris-proxy-api` 的 svclb DaemonSet 消失，确认 18080 已释放。
5. apply 新 Service，轮询 `127.0.0.1:18080/health`（此时唯一监听 18080 的是新服务，不会再假成功）。
6. 旧 namespace 整体保留，不删除（`kubectl delete namespace` 也在生产 skill 的永久禁止项内）；待观察期后人工清理。

失败语义：第 2/3 步失败时旧服务完全不动；第 5 步失败时旧服务已被摘除，需按 §4.5 回滚。

该退役逻辑只在 `LEGACY_NAMESPACE` 存在且仍有运行副本/旧 Service 时执行，幂等；观察期结束后可删除这段一次性迁移代码。

### 4.3 GHCR 镜像可见性

GitHub 仓库改名后，CI 的 `IMAGE_NAME` 自动变为 `ghcr.io/hcd233/aris-mass`，这是一个新包。旧包 `aris-proxy-api` 目前可匿名拉取（已验证 200），新包默认可能是私有。处理：

- 构建命令增加 `--label org.opencontainers.image.source=https://github.com/hcd233/aris-mass`，使包与公开仓库关联、继承可见性。
- 若仍为私有：migration Job 会 ImagePullBackOff 而中止，旧服务不受影响；此时用 `gh auth refresh -s write:packages` 后通过 API 改 public，或由仓库 owner 在包设置页修改，然后重跑 workflow。

### 4.5 回滚

若新服务健康检查失败或上线后异常：

1. 删除新 Service（释放 18080）：`kubectl -n aris-mass delete service aris-mass`；
2. 恢复旧 Service：`git show <merge 前 commit>:k8s/service.yaml | kubectl apply -f -`；
3. 旧 Deployment 扩回 2 副本：`kubectl -n aris-proxy-api scale deployment aris-proxy-api --replicas=2`；
4. 若旧 Deployment 已被删除，用合并前 commit 的 `deployment.yaml` 重新 apply（镜像仍可用 `ghcr.io/hcd233/aris-proxy-api@sha256:c70c3423...`）；
5. 代码侧 revert 合并提交，CI 会重新构建并部署。

### 4.4 GitHub / 服务器 / 客户端

- 用 `gh repo rename aris-mass --repo hcd233/aris-proxy-api --yes` 完成仓库改名；更新本地与服务器 clone 的 `origin`。
- 服务器目录改名为 `/home/ubuntu/code/aris-mass`，保留旧路径软链；同步更新 GitHub secret `PRODUCTION_REPO_PATH`。
- 客户端：`ArisClientReleaseBaseURL` 与 `install_aris_client.sh.tmpl` 指向新仓库；老客户端依赖 GitHub 对旧 URL 的 301 重定向。
- JWT `iss`/`aud` 改名会让存量 access（12h）与 refresh（168h）token 全部失效，所有用户需重新登录一次。这是本次已知的一次性影响。

## 5. 验证方案

| 阶段 | 验证 |
| --- | --- |
| 本地 | `go build ./...`、`go test ./...`、`go vet ./...`；`web` 的 `npm run lint`、`npm test`、`npm run build` |
| 静态 | `git grep -n "aris-proxy-api"` 仅剩 `docs/superpowers/**`；`git grep -n "aris-proxy"` 仅剩 `ClientModelProviderID` 及其测试；`make lint`（如工具可用） |
| CI | PR 的 lint workflow 通过；merge 后 build-and-publish 成功 |
| 生产 | `kubectl -n aris-mass get deploy,pod,svc` 全部 Ready；`GET /health`、`GET /ready` 200；`https://api.lvlvko.top/web/` 登录页显示 `Aris Mass`；`kubectl -n aris-proxy-api` 不再有运行中的 workload |
| 客户端 | `GET /install.sh` 返回的 `gh_base` 指向新仓库；`aris update` 的 HEAD 探测可解析新 release 地址；provider key 迁移进入新的客户端 release 后才对用户生效 |

## 6. 非目标

- 不改域名、OAuth 应用、API 路由与协议契约。
- 不改 CLI 品牌 `aris` 与用户配置目录。
- 不重写历史设计/计划文档。
- 不在本次删除旧 GHCR 包与旧 namespace（保留回滚窗口）。

## 7. 实施顺序

1. 在 `.worktrees/` 创建分支 `refactor/rebrand-aris-mass-2026-11-02`。
2. 批量替换 + 手工修正常量、脚本、CI、Web、skills、活文档。
3. 本地构建/测试/lint 与 `git grep` 残留检查。
4. 提交并推送分支、开 PR，先让 lint 通过。
5. 仓库改名 + 服务器目录准备 + secret 更新。
6. 合并 PR，观察 CI 构建与部署；按 §4.2 完成切换并验证。
7. 补写 Serena 工程经验，清理 worktree。

## 8. 待确认/风险

- GHCR 新包可见性（§4.3）是唯一可能让首次部署失败的点，失败时旧服务不受影响。
- JWT 失效导致全量重新登录（§4.4）。
- 历史文档保留旧名，`docs/superpowers/**` 内的代码片段不会随 module 改名更新。
- 架构图 GIF 若无法重新生成，会短暂保留旧品牌文案。
- dev compose 的 Postgres 数据库名改成 `aris-mass` 后，已有本地 volume 需要重建或手动 rename database；生产库不受影响。
- 设计评审的 builder check 子代理超时未返回，按流程由实施者自审补全了 provider key 兼容、回滚步骤与 svclb 释放校验。
