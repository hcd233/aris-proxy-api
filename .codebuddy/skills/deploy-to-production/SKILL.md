---
name: deploy-to-production
description: 构建并部署 aris-proxy-api 到生产环境。当用户想要部署、发布、发布到生产环境时使用此技能。覆盖完整流程：git commit + push → 等待 GitHub Actions Docker 构建 → SSH 到服务器执行部署脚本。
compatibility: 需要已认证的 gh CLI、生产服务器的 SSH 密钥，以及服务器上已安装 docker compose。
---

# 部署

将 aris-proxy-api 项目部署到 `api.lvlvko.top` 的生产服务器。

## 工作流程

本技能覆盖完整流程 — 提交并推送、等待 CI 构建、然后在服务器上部署。

### 第 1 步：提交并推送

如果有未提交的更改，先暂存并提交。如果未提供提交信息，请向用户询问。使用约定式提交格式：`type(scope): description`。

```bash
git add -A
git commit -m "<信息>"
git push origin <当前分支>
```

推送后，记下当前分支名称 — 稍后用于镜像标签。

### 第 2 步：等待 Docker 构建

GitHub Actions 工作流 `docker-publish.yml` 在推送到 `master` 或版本标签（`v*.*.*`）时触发。如果推送到非 master 分支，工作流可能不会自动触发 — 相应调整标签策略。

查找并监控工作流运行：

```bash
# 列出 docker-publish 工作流的最近运行记录
gh run list --workflow docker-publish.yml --repo hcd233/aris-proxy-api --limit 5

# 监控最新运行（将 <run-id> 替换为实际 ID）
gh run watch <run-id> --repo hcd233/aris-proxy-api --exit-status
```

轮询直到构建完成。构建通常需要不到 3 分钟。如果 `gh run watch` 以非零状态退出，说明构建失败 — 请向用户报告。

### 第 3 步：在服务器上部署

Docker 镜像构建并推送到 GHCR 后，从域名解析生产服务器 IP 并通过 SSH 连接。

**所有 SSH 调用都必须设置总超时和连接超时**，防止部署命令在网络抖动、日志 tail、pty 卡住等场景下把会话挂死：

```bash
# 从生产域名解析服务器 IP
PROD_IP=$(dig +short api.lvlvko.top | head -1)

# SSH 到服务器并执行部署脚本（Linux 默认有 timeout 命令）
timeout 60 ssh -o ConnectTimeout=10 ubuntu@${PROD_IP} 'cd code/aris-proxy-api/ && bash script/deploy.sh'
```

**macOS 教训（必看）**：macOS 系统默认 **没有** `timeout` 命令（只有安装了 `coreutils` 才有 `gtimeout`）。此时直接执行 `timeout 60 ssh ...` 会报 `command not found: timeout`，而整条管道仍以 exit 0 结束，看起来成功但实际部署**根本没发生**。正确的跨平台写法是使用 `perl -e 'alarm'`：

```bash
# 跨平台 SSH 超时写法（macOS 优先用这个）
perl -e 'alarm 60; exec @ARGV' \
  ssh -o ConnectTimeout=10 -o ServerAliveInterval=5 -o ServerAliveCountMax=3 \
      ubuntu@${PROD_IP} 'cd code/aris-proxy-api/ && bash script/deploy.sh'
```

- `alarm 60` 对进程组强制 60s 总超时
- `ConnectTimeout=10` 约束 TCP/SSH 握手阶段
- `ServerAliveInterval=5` + `ServerAliveCountMax=3` 在会话期间每 5s 发心跳，连续 3 次无响应即断开，避免 tail 日志 / broken pipe 卡死

**严禁**在 SSH 中启动 `docker logs -f`、`tail -f`、交互式 shell 等**长驻前台命令**。部署脚本（`script/deploy.sh`）末尾使用的是 `docker logs --tail 25` 这种短输出模式，不会卡住；如果将来有人改成 `-f`，必须在 SSH 外层套 `timeout`/`alarm`。

### 第 4 步：验证镜像是否真的被换掉

部署脚本有时会打印 `Container aris-proxy-api Running` —— 这行**不保证**容器用了新镜像，只是 compose 看到它还在跑。需要额外比对镜像创建时间和容器启动时间：

```bash
perl -e 'alarm 30; exec @ARGV' ssh -o ConnectTimeout=10 ubuntu@${PROD_IP} \
  'docker inspect aris-proxy-api --format "Container started: {{.State.StartedAt}}
Image: {{.Image}}" && docker inspect ghcr.io/hcd233/aris-proxy-api:master --format "Image created: {{.Created}}"'
```

判定规则：**容器 `StartedAt` ≥ 镜像 `Created`** → 跑的是新代码；否则说明 compose 复用了旧容器，需要手动 `docker compose up -d --force-recreate`。

### 第 5 步：E2E 回归验证

**不要**只跑 `curl` 就声明闭环。必须跑仓库内沉淀的 E2E 用例：

```bash
BASE_URL=https://api.lvlvko.top API_KEY=$ANTHROPIC_AUTH_TOKEN \
  perl -e 'alarm 180; exec @ARGV' \
  go test -v -count=1 ./test/e2e/<topic>/
```

新 bug 修复时，这条命令应同时覆盖：1) 针对当前 bug 的回归用例；2) 同 topic 的已有用例（防止 bugfix 破坏其他场景）。

如果 E2E 失败，从响应头拿 `X-Trace-Id`，回到 `cls-log-bugfix` 流程排障。

### 第 6 步：报告

部署完成后，总结以下信息：
- 部署的分支和提交
- 构建状态和耗时
- 容器启动时间 vs 镜像创建时间（证明跑的是新代码）
- E2E 回归的 `go test` 输出（证明线上功能通路）
- 日志中的任何警告或错误

## 服务器详情

| 项目 | 值 |
|------|-------|
| 域名 | `api.lvlvko.top` |
| 解析命令 | `dig +short api.lvlvko.top` |
| 用户 | `ubuntu` |
| 项目路径 | `code/aris-proxy-api/` |
| 部署脚本 | `script/deploy.sh` |
| Docker 镜像 | `ghcr.io/hcd233/aris-proxy-api` |
| 服务名称 | `aris-proxy-api` |

## 故障排查

- **构建未触发**：工作流仅在推送 `master` 和 `v*.*.*` 标签时触发。对于其他分支，请推送到 master 或创建版本标签。
- **构建失败**：使用 `gh run view <run-id> --log --repo hcd233/aris-proxy-api` 查看日志。
- **SSH 失败**：检查 SSH 密钥是否已加载（`ssh-add -l`）。
- **SSH 卡死**：你忘了超时包装。立刻 `Ctrl+C`，改用 `perl -e 'alarm N; exec @ARGV' ssh -o ConnectTimeout=10 ...`。
- **docker pull 失败**：确认镜像标签与分支名称匹配（`/` 替换为 `-`）。
- **容器崩溃**：部署后，通过 `perl -e 'alarm 30; exec @ARGV' ssh -o ConnectTimeout=10 ubuntu@$(dig +short api.lvlvko.top | head -1) 'docker logs aris-proxy-api --tail 50'` 检查日志。
- **容器在跑但代码没变**：对比 `StartedAt` 与镜像 `Created`；若旧，执行 `docker compose -f docker/docker-compose-single.yml up -d --force-recreate`。
