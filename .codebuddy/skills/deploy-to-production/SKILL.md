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
gh run watch <run-id> --repo hcd233/aris-proxy-api
```

轮询直到构建完成。构建通常需要不到 3 分钟。如果 `gh run watch` 以非零状态退出，说明构建失败 — 请向用户报告。

### 第 3 步：在服务器上部署

Docker 镜像构建并推送到 GHCR 后，从域名解析生产服务器 IP 并通过 SSH 连接：

```bash
# 从生产域名解析服务器 IP
PROD_IP=$(dig +short api.lvlvko.top | head -1)

# SSH 到服务器并执行部署脚本（设置 1 分钟超时，覆盖连接和执行全过程）
timeout 60 ssh -o ConnectTimeout=10 ubuntu@${PROD_IP} 'cd code/aris-proxy-api/ && bash script/deploy.sh'
```

部署脚本执行以下操作：
1. `git fetch` + `git pull --ff-only` — 拉取最新代码（用于 docker-compose 配置）
2. `docker pull ghcr.io/hcd233/aris-proxy-api:<分支名>` — 拉取新镜像
3. `docker compose up -d` — 使用新镜像重启服务
4. `docker image prune -a -f` — 清理旧镜像
5. `docker logs -f aris-proxy-api --details` — 持续输出日志，验证服务是否正常启动

**重要**：最后一步（`docker logs -f`）会持续跟踪日志。看到几行健康日志后，部署即成功。按 Ctrl+C 停止跟踪。

### 第 4 步：报告

部署完成后，总结以下信息：
- 部署的分支和提交
- 构建状态和耗时
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
- **docker pull 失败**：确认镜像标签与分支名称匹配（`/` 替换为 `-`）。
- **容器崩溃**：部署后，通过 `ssh ubuntu@$(dig +short api.lvlvko.top | head -1) 'docker logs aris-proxy-api --tail 50'` 检查日志。
