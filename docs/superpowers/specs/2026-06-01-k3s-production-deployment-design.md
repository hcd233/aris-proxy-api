# k3s 生产部署方案设计

## 背景

`api.lvlvko.top` 原生产链路为：

```text
浏览器 / API 客户端
  -> api.lvlvko.top:443
  -> 1Panel OpenResty
  -> http://172.18.0.1:7070
  -> Docker 容器 aris-proxy-api:8080
```

PostgreSQL 和 Redis 仍运行在 Docker 中，并且仅通过宿主机本地端口暴露：

- PostgreSQL：`127.0.0.1:5432 -> docker postgresql:5432`
- Redis：`127.0.0.1:6379 -> docker redis:6379`

k3s 与 Docker bridge 网络不天然互通。因此应用迁入 k3s 时，不直接访问 Docker 容器 IP，而是通过宿主机私网 IP 上的 `socat` 高可用桥接访问 Docker 中的数据库和缓存。

## 目标

1. 正式流量切到 k3s 上的 `aris-proxy-api`。
2. Docker 中的 PostgreSQL 和 Redis 暂不迁移，作为外部依赖继续复用。
3. `socat` 桥接必须由 systemd 托管，支持开机自启和失败自动恢复。
4. Pod 资源配置必须适配当前 2C / 4GiB 小规格服务器，避免滚动升级或异常重启导致 OOM。
5. GitHub Actions 在 `master` 推送后自动构建多架构镜像，并通过 SSH 发布到 k3s。

## 生产流量链路

切换后的链路为：

```text
浏览器 / API 客户端
  -> api.lvlvko.top:443
  -> 1Panel OpenResty
  -> http://127.0.0.1:18080
  -> k3s Service aris-proxy-api:18080
  -> Pod aris-proxy-api:8080
```

Docker 应用容器不再承接 `api.lvlvko.top` 流量。部署脚本在 k3s 健康后会对旧 Docker 应用执行 `docker update --restart=no aris-proxy-api` 和 `docker stop aris-proxy-api`，避免双实例重复执行 cron；PostgreSQL 和 Redis 容器必须继续运行。

## Docker 到 k3s 的数据依赖桥接

k3s 内部应用使用固定服务名：

- `POSTGRES_HOST=postgresql`
- `REDIS_HOST=redis`

对应的 Kubernetes `Service + Endpoints` 指向节点 InternalIP：

```text
k3s Pod
  -> Service postgresql / redis
  -> Endpoints <node-internal-ip>:5432 / <node-internal-ip>:6379
  -> systemd socat
  -> 127.0.0.1:5432 / 127.0.0.1:6379
  -> Docker PostgreSQL / Redis
```

`socat` systemd 单元由 `script/deploy-k8s.sh` 幂等创建和重启：

- `socat-postgresql.service`：`<node-internal-ip>:5432 -> 127.0.0.1:5432`
- `socat-redis.service`：`<node-internal-ip>:6379 -> 127.0.0.1:6379`

单元要求：

- `WantedBy=multi-user.target`
- `Restart=always`
- `RestartSec=5s`
- `After=network-online.target docker.service`

这样可以覆盖进程崩溃、服务器重启和 Docker 重启后的自动恢复场景。

## k3s 工作负载设计

正式 Deployment：`aris-proxy-api`

关键策略：

- `replicas: 1`：当前服务器只有 2C / 4GiB，不做双副本常驻。
- `strategy.type: Recreate`：升级时先停旧 Pod 再起新 Pod，避免短时间双 Pod 叠加导致内存压力。
- `requests.cpu: 50m`，`requests.memory: 128Mi`：给调度器保守基线。
- `limits.cpu: 300m`，`limits.memory: 512Mi`：防止异常流量或泄漏拖垮整机。
- `emptyDir.sizeLimit: 512Mi` 挂载 `/app/logs`：限制文件日志占用。
- `readinessProbe` 和 `livenessProbe` 均访问 `/health`。

数据库迁移不放在 `initContainer`，因为 `initContainer` 会随每个 Pod 执行。部署脚本改为每次发布前创建一次 Kubernetes Job：

```text
Job aris-proxy-api-db-migrate-<timestamp>
  -> /app/aris-proxy-api database migrate
  -> 成功后再更新 Deployment
```

这样能避免多副本或重启时重复迁移。

## 配置与密钥

服务器仍以 `env/api.env` 作为生产事实源。部署脚本在服务器侧读取该文件并生成：

- `ConfigMap aris-proxy-api-config`：非敏感配置。
- `Secret aris-proxy-api-secret`：包含 `*SECRET*`、`*PASSWORD*`、`*KEY*`、`*TOKEN*` 这类敏感字段。

部署脚本会覆盖以下 k3s 专用连接配置：

```text
PORT=8080
POSTGRES_HOST=postgresql
REDIS_HOST=redis
LOG_DIR=./logs
```

因此 `env/api.env` 中原本面向 Docker 网络的数据库和缓存主机名不会泄漏到 Pod 中。

## CI/CD 流程

`.github/workflows/docker-publish.yml` 的流程：

1. `build`：按 `linux/amd64` 和 `linux/arm64` 分别构建并推送 digest。
2. `merge`：合并多架构 manifest，并推送 `master`、semver、sha 和分支标签。
3. `deploy-k8s`：仅在 `master` push 时运行，通过 SSH 执行服务器上的 `script/deploy-k8s.sh`。

触发路径包含：

- `internal/**`
- `docker/**`
- `cmd/**`
- `web/**`
- `k8s/**`
- `script/**`
- `.github/workflows/docker-publish.yml`
- `main.go`
- `go.mod`
- `go.sum`

`deploy-k8s` 必须配置以下 GitHub Secrets：

- `PRODUCTION_HOST`
- `PRODUCTION_USERNAME`
- `PRODUCTION_SSH_KEY`
- `PRODUCTION_REPO_PATH`

部署任务设置：

- 远端脚本使用 `set -euo pipefail`：任一远端命令失败即失败。
- `command_timeout: 10m`：防止 SSH 长时间挂死。
- 部署后访问 `https://api.lvlvko.top/health` 做线上健康验证。

## 手工切流与回滚

切流方式：修改 1Panel OpenResty 的 `api.lvlvko.top` root proxy：

```nginx
proxy_pass http://127.0.0.1:18080;
```

修改后必须执行：

```bash
docker exec 1Panel-openresty-4VqU nginx -t
docker exec 1Panel-openresty-4VqU nginx -s reload
curl -kfsS --max-time 10 https://api.lvlvko.top/health
```

回滚方式：先恢复旧 Docker 应用，再恢复切流前备份，将 upstream 改回 Docker：

```bash
docker update --restart=always aris-proxy-api
docker start aris-proxy-api
```

```nginx
proxy_pass http://172.18.0.1:7070;
```

然后同样执行 `nginx -t`、reload 和 `/health` 验证。

## 验证清单

每次发布至少确认：

1. `systemctl is-active socat-postgresql socat-redis` 均为 `active`。
2. `kubectl get deploy,pod,svc -n aris-proxy-api` 中正式 Pod 为 `1/1 Running`。
3. `curl http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。
4. `curl -kfsS https://api.lvlvko.top/health` 返回 `{"status":"ok"}`。
5. `kubectl top pod -n aris-proxy-api` 中 Pod 内存低于 `512Mi` limit。
6. OpenResty access log 对 `/health`、`/docs` 等请求能在 k3s Pod 日志中看到对应记录。

## 后续演进

当前方案只迁移应用层，不迁移有状态组件。后续如需完全 Kubernetes 化，应单独规划 PostgreSQL 和 Redis：

1. 先为数据库做备份和恢复演练。
2. 再设计 `StatefulSet + PVC` 或使用托管数据库。
3. 最后逐步去掉 `socat` 和 Docker 数据依赖桥接。

在完成有状态组件迁移前，不能删除 Docker PostgreSQL 和 Redis。