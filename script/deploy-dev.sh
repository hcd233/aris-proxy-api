#!/usr/bin/env bash
#
# deploy-dev.sh - 部署开发环境
#
# 严格模式：任一命令失败立即退出，避免带旧代码或脏状态部署。
#
#	@author centonhuang
#	@update 2026-04-20 11:00:00
set -euo pipefail

echo -e "\033[1;36mDeploying development environment...\033[0m"

cd "$(dirname "$0")/.."

echo -e "\033[1;36mPulling the latest code from GitHub...\033[0m"
git fetch --prune origin
git checkout master
git pull --ff-only origin master

echo -e "\033[1;32mPulling the latest Docker image...\033[0m"
docker pull ghcr.io/hcd233/aris-proxy-api:master

echo -e "\033[1;34mStarting up services with docker-compose...\033[0m"
docker compose -f docker/docker-compose-dev-single.yml up -d

echo -e "\033[1;31mPruning unused Docker images...\033[0m"
docker image prune -a -f

echo -e "\033[1;33mDisplaying Docker logs for aris-proxy-api-dev...\033[0m"
docker logs -f aris-proxy-api-dev --details
