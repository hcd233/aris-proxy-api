#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
K8S_DIR="${REPO_DIR}/k8s"
NAMESPACE="aris-proxy-api"
APP_NAME="aris-proxy-api"
ENV_FILE="${REPO_DIR}/env/api.env"
SERVICE_PORT="18080"

BRANCH_NAME=$(git -C "${REPO_DIR}" rev-parse --abbrev-ref HEAD)
DEPLOY_ID=$(date +%Y%m%d%H%M%S)
MIGRATION_JOB="${APP_NAME}-db-migrate-${DEPLOY_ID}"

CONFIG_ENV=$(mktemp)
SECRET_ENV=$(mktemp)
cleanup() {
  rm -f "${CONFIG_ENV}" "${SECRET_ENV}"
}
trap cleanup EXIT

log() {
  echo -e "\033[1;36m$*\033[0m"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

wait_for_service_health() {
  local url="http://127.0.0.1:${SERVICE_PORT}/health"

  for i in $(seq 1 60); do
    if curl -fsS --max-time 3 "${url}"; then
      echo
      return 0
    fi

    echo "waiting for ${url} (${i}/60)" >&2
    sleep 1
  done

  kubectl get deployment,pod,service,endpoints -n "${NAMESPACE}" -o wide >&2 || true
  return 1
}

mask_sensitive_keys_to_secret() {
  : > "${CONFIG_ENV}"
  : > "${SECRET_ENV}"

  while IFS= read -r line || [[ -n "${line}" ]]; do
    case "${line}" in
      ""|\#*) continue ;;
    esac

    key="${line%%=*}"
    case "${key}" in
      POSTGRES_HOST|REDIS_HOST|PORT|LOG_DIR)
        continue
        ;;
    esac

    case "${key}" in
      *SECRET*|*PASSWORD*|*KEY*|*TOKEN*)
        printf '%s\n' "${line}" >> "${SECRET_ENV}"
        ;;
      *)
        printf '%s\n' "${line}" >> "${CONFIG_ENV}"
        ;;
    esac
  done < "${ENV_FILE}"

  cat >> "${CONFIG_ENV}" <<EOF
PORT=8080
POSTGRES_HOST=postgresql
REDIS_HOST=redis
LOG_DIR=./logs
EOF
}

node_internal_ip() {
  if [[ -n "${K8S_BRIDGE_HOST_IP:-}" ]]; then
    echo "${K8S_BRIDGE_HOST_IP}"
    return
  fi

  kubectl get node -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}'
}

ensure_socat_unit() {
  local name="$1"
  local port="$2"
  local bridge_host_ip="$3"

  sudo tee "/etc/systemd/system/${name}.service" >/dev/null <<EOF
[Unit]
Description=Socat TCP proxy: ${bridge_host_ip}:${port} -> 127.0.0.1:${port}
Wants=network-online.target docker.service
After=network-online.target docker.service

[Service]
Type=simple
ExecStart=/usr/bin/socat TCP-LISTEN:${port},bind=${bridge_host_ip},reuseaddr,fork TCP:127.0.0.1:${port}
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

  sudo systemctl daemon-reload
  sudo systemctl enable "${name}.service" >/dev/null
  sudo systemctl restart "${name}.service"
  sudo systemctl is-active --quiet "${name}.service"
}

apply_external_service() {
  local name="$1"
  local port="$2"
  local bridge_host_ip="$3"

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: ${name}
  namespace: ${NAMESPACE}
spec:
  ports:
  - name: tcp
    port: ${port}
    targetPort: ${port}
---
apiVersion: v1
kind: Endpoints
metadata:
  name: ${name}
  namespace: ${NAMESPACE}
subsets:
- addresses:
  - ip: ${bridge_host_ip}
  ports:
  - name: tcp
    port: ${port}
EOF
}

run_migration() {
  cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${MIGRATION_JOB}
  namespace: ${NAMESPACE}
  labels:
    app: ${APP_NAME}
    job: db-migrate
spec:
  backoffLimit: 1
  ttlSecondsAfterFinished: 600
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: db-migrate
        image: ${IMAGE}
        imagePullPolicy: Always
        command: ["/app/aris-proxy-api", "database", "migrate"]
        envFrom:
        - configMapRef:
            name: ${APP_NAME}-config
        - secretRef:
            name: ${APP_NAME}-secret
        resources:
          requests:
            cpu: 50m
            memory: 128Mi
          limits:
            cpu: 300m
            memory: 512Mi
EOF

  if ! kubectl wait --for=condition=complete "job/${MIGRATION_JOB}" -n "${NAMESPACE}" --timeout=120s; then
    kubectl logs "job/${MIGRATION_JOB}" -n "${NAMESPACE}" --tail=100 || true
    exit 1
  fi
}

require_cmd git
require_cmd kubectl
require_cmd socat
require_cmd curl

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing env file: ${ENV_FILE}" >&2
  exit 1
fi

log "Preparing k3s deployment from branch ${BRANCH_NAME}"

git -C "${REPO_DIR}" fetch --prune origin
git -C "${REPO_DIR}" pull --ff-only origin "${BRANCH_NAME}"

COMMIT_SHA=$(git -C "${REPO_DIR}" rev-parse --short=7 HEAD)
IMAGE_TAG="${DEPLOY_IMAGE_TAG:-master}"
IMAGE="ghcr.io/hcd233/aris-proxy-api:${IMAGE_TAG}"
log "Deploying to k3s (branch: ${BRANCH_NAME}, commit: ${COMMIT_SHA}, image: ${IMAGE})"

kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1 || kubectl create namespace "${NAMESPACE}"

BRIDGE_HOST_IP=$(node_internal_ip)
if [[ -z "${BRIDGE_HOST_IP}" ]]; then
  echo "failed to detect Kubernetes node InternalIP" >&2
  exit 1
fi

log "Ensuring Docker-to-k3s bridge through host ${BRIDGE_HOST_IP}"
ensure_socat_unit socat-postgresql 5432 "${BRIDGE_HOST_IP}"
ensure_socat_unit socat-redis 6379 "${BRIDGE_HOST_IP}"
apply_external_service postgresql 5432 "${BRIDGE_HOST_IP}"
apply_external_service redis 6379 "${BRIDGE_HOST_IP}"

log "Applying config and secret from env/api.env"
mask_sensitive_keys_to_secret
kubectl create configmap "${APP_NAME}-config" -n "${NAMESPACE}" --from-env-file="${CONFIG_ENV}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic "${APP_NAME}-secret" -n "${NAMESPACE}" --from-env-file="${SECRET_ENV}" --dry-run=client -o yaml | kubectl apply -f -

log "Running database migration job ${MIGRATION_JOB}"
run_migration

log "Applying deployment and service"
sed "s|ghcr.io/hcd233/aris-proxy-api:PLACEHOLDER|${IMAGE}|g" "${K8S_DIR}/deployment.yaml" | kubectl apply -f -
kubectl apply -f "${K8S_DIR}/service.yaml"

log "Cleaning up stuck pods before rollout (if any)"
kubectl delete pod -n "${NAMESPACE}" -l app=${APP_NAME} --field-selector=status.phase!=Running --ignore-not-found=true || true

log "Restarting deployment to pick up new image (tag: ${IMAGE_TAG})"
kubectl rollout restart "deployment/${APP_NAME}" -n "${NAMESPACE}"

log "Waiting for rollout"
kubectl rollout status "deployment/${APP_NAME}" -n "${NAMESPACE}" --timeout=600s

log "Verifying k3s service health"
wait_for_service_health

log "Removing legacy canary resources after formal deployment is healthy"
kubectl delete deployment,service,configmap,secret -n "${NAMESPACE}" -l app=aris-proxy-api-k3s-canary --ignore-not-found=true >/dev/null 2>&1 || true

if command -v docker >/dev/null 2>&1 && docker ps -a --format '{{.Names}}' | grep -qx "${APP_NAME}"; then
  log "Stopping legacy Docker app container to avoid duplicate cron jobs"
  docker update --restart=no "${APP_NAME}" >/dev/null 2>&1 || true
  docker stop "${APP_NAME}" >/dev/null 2>&1 || true
fi

log "Current k3s resources"
kubectl get deployment,pod,service -n "${NAMESPACE}" -o wide

log "Deployment complete"
