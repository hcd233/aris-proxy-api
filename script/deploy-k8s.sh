#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
K8S_DIR="${REPO_DIR}/k8s"
DEPLOYMENT_FILE="${K8S_DIR}/deployment.yaml"

BRANCH_NAME=$(git -C "${REPO_DIR}" rev-parse --abbrev-ref HEAD)
IMAGE_TAG=$(echo "${BRANCH_NAME}" | tr '/' '-')

echo -e "\033[1;36mDeploying to K8s (branch: ${BRANCH_NAME}, image: ${IMAGE_TAG})...\033[0m"

echo -e "\033[1;36mPulling the latest code from GitHub...\033[0m"
git -C "${REPO_DIR}" fetch --prune origin
git -C "${REPO_DIR}" pull --ff-only origin "${BRANCH_NAME}"

echo -e "\033[1;32mUpdating deployment image tag...\033[0m"
if [[ "$(uname)" == "Darwin" ]]; then
  sed -i '' "s|ghcr.io/hcd233/aris-proxy-api:PLACEHOLDER|ghcr.io/hcd233/aris-proxy-api:${IMAGE_TAG}|g" "${DEPLOYMENT_FILE}"
else
  sed -i "s|ghcr.io/hcd233/aris-proxy-api:PLACEHOLDER|ghcr.io/hcd233/aris-proxy-api:${IMAGE_TAG}|g" "${DEPLOYMENT_FILE}"
fi

echo -e "\033[1;34mApplying K8s resources...\033[0m"
kubectl apply -f "${K8S_DIR}/namespace.yaml"
kubectl apply -f "${K8S_DIR}/configmap.yaml"
kubectl apply -f "${K8S_DIR}/secret.yaml"
kubectl apply -f "${K8S_DIR}/deployment.yaml"
kubectl apply -f "${K8S_DIR}/service.yaml"

echo -e "\033[1;33mWaiting for rollout to complete...\033[0m"
kubectl rollout status deployment/aris-proxy-api -n aris-proxy-api --timeout=120s

echo -e "\033[1;36mRestoring PLACEHOLDER in deployment.yaml...\033[0m"
if [[ "$(uname)" == "Darwin" ]]; then
  sed -i '' "s|ghcr.io/hcd233/aris-proxy-api:${IMAGE_TAG}|ghcr.io/hcd233/aris-proxy-api:PLACEHOLDER|g" "${DEPLOYMENT_FILE}"
else
  sed -i "s|ghcr.io/hcd233/aris-proxy-api:${IMAGE_TAG}|ghcr.io/hcd233/aris-proxy-api:PLACEHOLDER|g" "${DEPLOYMENT_FILE}"
fi

echo -e "\033[1;32mDeployment complete!\033[0m"
