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
#	@update 2026-04-24 15:00:00
set -euo pipefail

cd "$(dirname "$0")/.."

BRANCH_NAME=$(git rev-parse --abbrev-ref HEAD)
IMAGE_TAG=$(echo "${BRANCH_NAME}" | tr '/' '-')

echo -e "\033[1;36mDeploying production environment (branch: ${BRANCH_NAME}, image: ${IMAGE_TAG})...\033[0m"

echo -e "\033[1;36mPulling the latest code from GitHub...\033[0m"
git fetch --prune origin
git pull --ff-only origin "${BRANCH_NAME}"

echo -e "\033[1;32mPulling the latest Docker image...\033[0m"
docker pull "ghcr.io/hcd233/aris-proxy-api:${IMAGE_TAG}"

echo -e "\033[1;34mStarting up services with docker-compose...\033[0m"
export IMAGE_TAG
docker compose -f docker/docker-compose-single.yml up -d

echo -e "\033[1;31mPruning unused Docker images...\033[0m"
docker image prune -a -f

echo -e "\033[1;33mDisplaying Docker logs for aris-proxy-api...\033[0m"
docker logs -f aris-proxy-api --details
