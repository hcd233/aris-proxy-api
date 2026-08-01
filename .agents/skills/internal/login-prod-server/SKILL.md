---
name: login-prod-server
description: 生产服务器登录入口与安全基线。任何需要 SSH 到生产服务器 api.lvlvko.top 执行操作的任务（查询生产数据库/Redis 缓存、更新生产配置、K8s 运维）都必须先加载本 skill 获取登录方式、环境布局、凭据读取规则与统一安全基线。触发词：SSH 到生产、登录生产服务器、连接线上服务器、api.lvlvko.top、生产环境操作。
---

# login-prod-server

生产服务器 `api.lvlvko.top` 的统一登录入口与安全基线。所有生产操作 skill（`operate-prod-service`、`query-prod-database`、`query-prod-cache`、`update-prod-config` 等）都复用本 skill 的连接与安全规则，**不要在各自的 skill 里重复维护连接细节**。

## 连接入口

始终使用域名，**禁止使用裸 IP**：

```bash
ssh ubuntu@api.lvlvko.top
```

SSH 失败时报告连接错误，不要尝试使用裸 IP 绕过。

## 环境布局（已实测验证）

生产服务器上同时运行 Docker 容器与 k3s（Kubernetes）集群：

| 组件 | 说明 |
|------|------|
| **k3s 集群** | v1.35.5 单节点（control-plane `10-7-109-6`），`kubectl` 直接可用；业务部署在 namespace `aris-proxy-api` |
| **Docker 容器** | `redis`（redis:8.4.0，由 1Panel 管理）、`postgresql`，供 k3s 集群外部访问 |
| **应用目录** | `/home/ubuntu/code/aris-proxy-api/`（含 `env/api.env`） |
| **1Panel** | 管理 Redis 等容器，数据/配置在 `/opt/1panel/apps/` |

**不要假设容器名/部署名永远不变**；执行前先确认：

```bash
docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}'
kubectl -n aris-proxy-api get deploy,svc
```

找不到明确目标时停止并向用户报告，不要猜测。

## 获取连接配置

应用自身的连接凭据统一存放在：

```text
/home/ubuntu/code/aris-proxy-api/env/api.env
```

常见键包括：

```text
REDIS_HOST / REDIS_PORT / REDIS_PASSWORD / REDIS_DB
POSTGRES_USER / POSTGRES_PASSWORD / POSTGRES_DATABASE / POSTGRES_HOST / POSTGRES_PORT / POSTGRES_SSLMODE
```

注意：生产上**没有**独立的 `redis.env`/`postgres.env`（只有 `.template`），容器由 1Panel 部署，Redis 密码在容器启动参数 `--requirepass` 与 `/opt/1panel/apps/redis/redis/conf/redis.conf` 中。若部署链路由 Kubernetes 管理，可检查对应 ConfigMap/Secret；读取 Secret 时**不得输出解码后的值**。

**不要把密码写入 skill、脚本、Git、聊天消息或命令行参数中。** 只允许输出配置键名或 `<redacted>`，例如：

```bash
awk -F= 'tolower($1) ~ /(redis|cache|postgres|database|db|dsn)/ {print $1"=<redacted>"}' \
  /home/ubuntu/code/aris-proxy-api/env/api.env
```

## 统一安全基线

适用于所有通过 SSH 对生产执行的操作：

1. **默认只读**：查询/查看类操作默认放行；任何写操作（改数据、改配置、删资源、重启、执行删除命令）必须先向用户完整展示待执行命令、说明影响范围，等待用户对这条完整命令的明确授权，只执行用户授权的原始命令。
2. **不泄露凭据**：不输出密码、连接串、Secret、API Key、JWT 或完整凭证；查询结果含敏感字段先脱敏再展示；不为方便查询把生产数据导出到本地。
3. **不降低安全规则**：不要为了"完成任务"而放宽只读默认或跳过授权确认。
4. **结果标注口径**：区分精确值与估算值（如 `kubectl top` 为实时采样、`n_live_tup` 为估算行数）。

## 失败处理

- SSH 失败：报告连接错误，不尝试裸 IP；
- 容器/资源不存在或不健康：停止并报告；
- 配置缺失或认证失败：停止，不猜测密码；
- 命令不明确、混入写操作或涉及禁止命令：停止并要求澄清/授权；
- 命令超时或失败：返回错误摘要，不自动重试可能产生副作用的命令。
