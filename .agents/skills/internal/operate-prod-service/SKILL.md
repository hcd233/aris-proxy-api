---
name: operate-prod-service
description: 操作生产环境 aris-proxy-api 服务（api.lvlvko.top 上的 k3s/Kubernetes 集群）。当用户要求查看线上服务状态、pod 资源与负载、pod 日志、滚动重启、跟踪部署、查看服务/端点、检查集群健康，或笼统地说"看一下线上环境/生产服务怎么了"时使用本 skill。
---

# operate-prod-service

通过 SSH 到生产服务器（k3s 集群）操作 aris-proxy-api 线上服务。**连接入口、凭据读取与统一安全基线一律以 `login-prod-server` skill 为准**，本 skill 只负责 K8s 操作清单与任务路由。

## 前置

1. 加载 `login-prod-server` skill，确认 SSH 方式、环境布局与安全基线；
2. 连接：

```bash
ssh ubuntu@api.lvlvko.top
```

3. 确认集群与命名空间现状（不要假设，先看）：

```bash
kubectl get nodes -o wide
kubectl -n aris-proxy-api get deploy,svc,pods -o wide
```

## 生产拓扑（已实测验证，2026-07 现状）

| 项 | 值 |
|----|----|
| 集群 | k3s v1.35.5，单节点 control-plane `10-7-109-6` |
| namespace | `aris-proxy-api` |
| Deployment | `aris-proxy-api`，副本 2，镜像 `ghcr.io/hcd233/aris-proxy-api:master` |
| 资源 request | cpu 50m / mem 128Mi |
| 资源 limit | cpu 300m / mem 512Mi |
| Service | `aris-proxy-api`（LoadBalancer，端口 18080）、`postgresql`、`redis`（ClusterIP） |
| HPA | 无 |

这些值是快照，操作前以 `kubectl get` 实况为准。

## K8s 操作清单（默认只读）

### 服务与集群状态

```bash
# 集群节点与健康
kubectl get nodes -o wide
kubectl get ns

# 部署与副本状态
kubectl -n aris-proxy-api get deploy -o wide
kubectl -n aris-proxy-api get rs,pods -o wide

# 服务与端点
kubectl -n aris-proxy-api get svc,endpoints -o wide
kubectl -n aris-proxy-api get configmap
kubectl -n aris-proxy-api get secret
```

### pod 资源与实时负载

```bash
# 各 pod 实时 CPU/内存占用（metrics-server 提供）
kubectl -n aris-proxy-api top pods

# 资源请求/限制定义
kubectl -n aris-proxy-api get deploy aris-proxy-api \
  -o jsonpath='{.spec.template.spec.containers[0].resources}{"\n"}'

# 节点资源总量与已用
kubectl top nodes
```

`top pods` 是实时采样值，不代表峰值；需要趋势时结合 CLS 指标主题或部署内监控。

### pod 日志

```bash
# 最近 N 行
kubectl -n aris-proxy-api logs deploy/aris-proxy-api --tail=200

# 指定 pod
kubectl -n aris-proxy-api logs aris-proxy-api-xxxx --tail=200

# 跟随实时输出（交互排查）
kubectl -n aris-proxy-api logs deploy/aris-proxy-api -f --tail=50

# 上一个容器实例（崩溃后排查）
kubectl -n aris-proxy-api logs aris-proxy-api-xxxx --previous
```

生产日志已接入腾讯云 CLS，深度排障（traceId 追踪、错误链路）转 `query-prod-log`，不要只依赖 `kubectl logs`。

### 健康检查

```bash
# 集群外健康检查（无需认证）
curl -s https://api.lvlvko.top/health
curl -sN https://api.lvlvko.top/ssehealth
```

交互式 API 验证转 `call-api` skill。

### 查看详细事件

```bash
# 排查启动失败、拉镜像失败等
kubectl -n aris-proxy-api describe pod <pod-name>
kubectl -n aris-proxy-api get events --sort-by=.lastTimestamp | tail -30
```

## 变更类操作（必须授权）

以下操作属于变更，**必须先向用户完整展示命令、说明影响范围并等待明确授权**，只执行用户授权的原始命令：

- **滚动重启**：`kubectl -n aris-proxy-api rollout restart deployment/aris-proxy-api`（无中断，逐副本替换）
- **跟踪 rollout 状态**：`kubectl -n aris-proxy-api rollout status deployment/aris-proxy-api --timeout=120s`
- **查看 rollout 历史/回滚**：`kubectl -n aris-proxy-api rollout history deployment/aris-proxy-api`；回滚 `kubectl -n aris-proxy-api rollout undo deployment/aris-proxy-api`
- **扩缩容**：`kubectl -n aris-proxy-api scale deployment/aris-proxy-api --replicas=N`
- **更新镜像/打补丁**：`kubectl -n aris-proxy-api set image` / `kubectl -n aris-proxy-api patch deployment` / `kubectl apply`
- **删除资源**：`kubectl -n aris-proxy-api delete pod/deployment/...`

如果用户只说"执行一下""可以""继续"，但没有看到完整命令，先展示命令并再次要求确认。

## 永久禁止

无论是否授权，永久禁止执行：

- `kubectl delete namespace`、`kubectl delete clusterrole/clusterrolebinding`——破坏集群级资源；
- `kubectl drain/cordon`、节点级操作——单节点集群，误操作即整体宕机；
- 通过 `kubectl exec` 在 pod 内执行任意写命令、安装软件、修改容器内文件——绕过镜像不可变原则，变更不持久且难审计；
- `kubectl port-forward` 暴露生产端口到本地——除非用户明确要求且获授权；
- 生产 Secret 的解码输出（`kubectl get secret -o yaml` 可看元数据，`-o jsonpath={.data.*} | base64 -d` 禁止展示）；
- 绕过 kubectl 改用裸 IP、其他连接路径或第三方管理面板（如 portainer）执行同类操作。

## 任务路由

生产相关任务先判断类型，转到对应 skill，不要在本 skill 内自行处理：

| 任务类型 | 转至 skill |
|---------|-----------|
| 生产 PostgreSQL 数据查询/写操作 | `query-prod-database` |
| 生产 Redis 缓存查询/写操作 | `query-prod-cache` |
| 修改 api.env / K8s ConfigMap / 滚动重启生效 | `update-prod-config` |
| CLS 日志排障、traceId/X-Trace-Id 追踪、错误链路 | `query-prod-log` |
| HTTP/API 调用示例、curl、交互式验证 | `call-api` |
| 服务状态、pod 资源/负载/日志、重启、部署跟踪 | 本 skill |

**部署流程**：推送 `master` 或合并 PR 到 `master` 自动触发 `docker-publish.yml` 构建镜像并部署到 K8s，无需手动 SSH 部署。跟踪部署用 `gh run list --workflow=docker-publish.yml`；需要验证部署结果时确认 pod 镜像 tag 与 rollout 状态。

## 输出要求

- 引用精确的 pod/deployment/service 名称与命令；
- 负载数据说明采样时间与口径（实时采样 vs 峰值/趋势）；
- 变更类操作执行后报告结果与影响；
- 不泄露 Secret、Token 等敏感字段。
