#!/usr/bin/env bash
#
# deploy.sh - 部署生产环境
#
# 严格模式说明：
#   - errexit（-e）：任一命令失败立即退出，防止带着旧镜像继续启动 / 误删其他镜像
#   - nounset（-u）：使用未定义变量即报错
#   - pipefail（-o pipefail）：管道中任一命令失败都会使整条管道失败
#
#	@author centonhuang
#	@update 2026-04-20 11:00:00
set -euo pipefail

echo -e "\033[1;36mDeploying production environment...\033[0m"

# 切到仓库根目录，确保 docker/ 等相对路径可用
cd "$(dirname "$0")/.."

echo -e "\033[1;36mPulling the latest code from GitHub...\033[0m"
# 若本地有未提交改动，git checkout / pull 会失败并终止整个脚本，避免带脏状态部署
git fetch --prune origin
git checkout master
git pull --ff-only origin master

echo -e "\033[1;32mPulling the latest Docker image...\033[0m"
docker pull ghcr.io/hcd233/aris-proxy-api:master

echo -e "\033[1;34mStarting up services with docker-compose...\033[0m"
docker compose -f docker/docker-compose-single.yml up -d

echo -e "\033[1;31mPruning unused Docker images...\033[0m"
docker image prune -a -f

echo -e "\033[1;33mDisplaying Docker logs for aris-proxy-api...\033[0m"
docker logs -f aris-proxy-api --details
