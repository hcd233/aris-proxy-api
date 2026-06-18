# Docker 构建优化实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化 Docker 构建速度（npm cache mount + UPX 降级 + 前端产物缓存）和镜像大小（distroless/static + tzdata 内嵌），同时修复 distroless 带来的 k8s 兼容性问题。

**Architecture:** 三阶段 Dockerfile（frontend-builder/frontend-prebuilt → go-builder → distroless final），CI 新增 frontend job 缓存前端产物，k8s deployment.yaml 加 fsGroup 并改 preStop 为 K8s 原生 sleep action。

**Tech Stack:** Docker buildx, GitHub Actions, distroless/static-debian12:nonroot, Go time/tzdata, K8s lifecycle sleep action

**Spec:** `docs/superpowers/specs/2026-06-18-docker-build-optimize-design.md`

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `.dockerignore` | 修改 | 排除 `web/node_modules/`、`web/.next/`，减少本地 build context |
| `main.go` | 修改 | 引入 `_ "time/tzdata"`，让二进制自带时区数据 |
| `docker/dockerfile` | 修改 | 整合 A（npm cache + UPX -9）+ B（build-arg 切换前端来源）+ E（distroless final） |
| `.github/workflows/docker-publish.yml` | 修改 | 新增 frontend job，build job 下载产物并传 build-arg |
| `k8s/deployment.yaml` | 修改 | 加 fsGroup: 65532，preStop 改为 K8s 原生 sleep action |

---

### Task 1: `.dockerignore` 排除前端大目录

**Files:**
- Modify: `.dockerignore`

- [ ] **Step 1: 读取当前 `.dockerignore`**

Run: `cat .dockerignore`
Expected: 现有内容，末尾有 CodeBuddy 段

- [ ] **Step 2: 在 `.dockerignore` 末尾追加前端排除规则**

在文件末尾（`CLAUDE.md` 行之后）追加：

```
# Frontend build artifacts (CI downloads web/out/ separately, must NOT exclude it)
web/node_modules/
web/.next/
```

完整的文件末尾应该是：

```
# CodeBuddy
.codebuddy/
CODEBUDDY.md
CLAUDE.md

# Frontend build artifacts (CI downloads web/out/ separately, must NOT exclude it)
web/node_modules/
web/.next/
```

- [ ] **Step 3: 验证排除生效**

Run: `docker buildx build --print --target frontend-builder -f docker/dockerfile . 2>&1 | head -5`
Expected: 不报错（验证 Dockerfile 语法正确，context 不含 node_modules）

- [ ] **Step 4: Commit**

```bash
git add .dockerignore
git commit -m "chore(docker): exclude web/node_modules and web/.next from build context"
```

---

### Task 2: `main.go` 引入 `time/tzdata`

**Files:**
- Modify: `main.go:6-8`

- [ ] **Step 1: 修改 `main.go` 的 import 块**

将：
```go
import (
	"github.com/hcd233/aris-proxy-api/cmd"
)
```

改为：
```go
import (
	_ "time/tzdata"

	"github.com/hcd233/aris-proxy-api/cmd"
)
```

`_ "time/tzdata"` 是 blank import，注册内嵌时区数据，使二进制不依赖系统 `/usr/share/zoneinfo/`。

- [ ] **Step 2: 验证编译通过**

Run: `go build -o /dev/null ./main.go`
Expected: 编译成功，无输出

- [ ] **Step 3: 验证时区数据内嵌生效**

Run: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tmp/aris-test ./main.go && /tmp/aris-test --help 2>&1 | head -3; TZ=Asia/Shanghai /tmp/aris-test server start --help 2>&1 | head -3`
Expected: 编译成功且 help 输出正常（证明二进制可独立运行，不依赖系统 tzdata）

- [ ] **Step 4: 验证测试不受影响**

Run: `go test -count=1 -run TestTracker ./test/unit/inflight/`
Expected: PASS（time/tzdata 不影响现有测试）

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: embed time/tzdata for distroless compatibility"
```

---

### Task 3: Dockerfile 整合改造（A + B + E）

**Files:**
- Modify: `docker/dockerfile`（整体重写）

这是核心改动，整合三个方向的变更：
- A2: 前端 npm cache mount
- A3: UPX `--best --lzma` → `-9`
- B3: ARG `FRONTEND_SOURCE` 切换前端来源
- E2: 最终镜像改用 distroless/static-debian12:nonroot

- [ ] **Step 1: 重写 `docker/dockerfile`**

完整内容：

```dockerfile
# 声明在第一个 FROM 之前，支持 CI 通过 build-arg 切换前端来源
ARG FRONTEND_SOURCE=frontend-builder

# 阶段1a: 从源码构建前端（本地默认）
FROM node:22-alpine AS frontend-builder

WORKDIR /app

COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY web/src/ ./src/
COPY web/public/ ./public/
COPY web/next.config.ts web/postcss.config.mjs web/components.json web/tsconfig.json web/eslint.config.mjs ./
RUN npm run build

# 阶段1b: 使用预构建前端（CI 模式，产物路径与 frontend-builder 统一为 /app/out）
FROM scratch AS frontend-prebuilt
COPY web/out/ /app/out/

# 阶段1: 根据 build-arg 选择前端来源
FROM ${FRONTEND_SOURCE} AS frontend-final

# 阶段2: Go 构建
FROM golang:1.25.1-alpine3.22 AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache upx

WORKDIR /app

# 复制依赖声明文件
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 复制后端源码（先复制 internal，确保前端构建产物后覆盖）
COPY cmd ./cmd
COPY internal ./internal
COPY main.go ./

# 复制前端构建产物到 internal/web/dist/（两个前端阶段路径统一为 /app/out）
COPY --from=frontend-final /app/out ./internal/web/dist

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /go/bin/aris-proxy-api \
    && upx -9 /go/bin/aris-proxy-api

# 阶段3: 运行时（distroless，自带 CA 证书，无 shell，nonroot）
FROM gcr.io/distroless/static-debian12:nonroot AS final

WORKDIR /app

COPY --from=builder /go/bin/aris-proxy-api /app/aris-proxy-api

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/app/aris-proxy-api"]

# 本地构建：docker buildx build --platform linux/amd64 -t aris-proxy-api:latest -f docker/dockerfile .
# CI 构建：docker buildx build --build-arg FRONTEND_SOURCE=frontend-prebuilt ...
# 本地运行：docker run -d -p 8080:8080 --env-file api.env --name aris-proxy-api -t aris-proxy-api:latest server start --host 0.0.0.0 --port 8080
```

- [ ] **Step 2: 验证本地构建成功（从源码构建前端）**

Run: `docker buildx build --platform linux/amd64 -t aris-proxy-api:test -f docker/dockerfile .`
Expected: 构建成功，三个阶段都通过

- [ ] **Step 3: 验证镜像大小**

Run: `docker images aris-proxy-api:test --format "{{.Size}}"`
Expected: 约 10-20MB（distroless 基础 ~2MB + UPX 后二进制 ~8-12MB）

- [ ] **Step 4: 验证镜像可运行且 /health 响应**

Run: `docker run -d -p 18080:8080 --name aris-test aris-proxy-api:test server start --host 0.0.0.0 --port 8080 && sleep 3 && curl -s http://localhost:18080/health && docker stop aris-test && docker rm aris-test`
Expected: `{"status":"ok"}` 或类似健康响应

- [ ] **Step 5: 验证时区正确**

Run: `docker run --rm --entrypoint /app/aris-proxy-api aris-proxy-api:test server start --help 2>&1 | head -3; docker run --rm -e TZ=Asia/Shanghai --entrypoint /app/aris-proxy-api aris-proxy-api:test --help 2>&1 | head -3`
Expected: 命令输出正常（证明 distroless 无系统 tzdata 但二进制内嵌的 time/tzdata 生效）

- [ ] **Step 6: 验证 CI 模式构建（模拟预构建前端）**

先准备预构建前端产物：
Run: `cd web && npm ci && npm run build && cd ..`

然后模拟 CI 传 build-arg：
Run: `docker buildx build --platform linux/amd64 --build-arg FRONTEND_SOURCE=frontend-prebuilt -t aris-proxy-api:ci-test -f docker/dockerfile .`
Expected: 构建成功，frontend-prebuilt 阶段从 context 的 `web/out/` COPY，跳过 node 构建

- [ ] **Step 7: Commit**

```bash
git add docker/dockerfile
git commit -m "feat(docker): optimize build speed and reduce image size

- Add npm cache mount for frontend build
- Downgrade UPX from --best --lzma to -9 (faster, minor size tradeoff)
- Support FRONTEND_SOURCE build-arg for CI prebuilt frontend
- Switch final image from alpine to distroless/static-debian12:nonroot
- Remove tzdata/curl packages (tzdata embedded in binary)"
```

---

### Task 4: CI workflow 改造（B — 前端产物缓存）

**Files:**
- Modify: `.github/workflows/docker-publish.yml`

- [ ] **Step 1: 在 `jobs:` 下、`build:` job 之前新增 `frontend` job**

在 `jobs:` 行之后、`build:` 之前插入：

```yaml
  frontend:
    runs-on: ubuntu-latest
    outputs:
      cache-hit: ${{ steps.cache.outputs.cache-hit }}
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Compute web hash
        id: hash
        run: echo "key=frontend-${{ hashFiles('web/**') }}" >> $GITHUB_OUTPUT

      - name: Cache frontend build
        id: cache
        uses: actions/cache@v4
        with:
          path: web/out
          key: ${{ steps.hash.outputs.key }}

      - name: Build frontend (on cache miss)
        if: steps.cache.outputs.cache-hit != 'true'
        run: cd web && npm ci && npm run build

      - name: Upload frontend artifact
        uses: actions/upload-artifact@v4
        with:
          name: frontend-out
          path: web/out
          retention-days: 1
```

- [ ] **Step 2: 在 `build` job 的 `steps` 里，Checkout 之后、Prepare Docker File 之前，加下载前端产物步骤**

在 `- name: Checkout repository` 步骤之后插入：

```yaml
      - name: Download frontend artifact
        uses: actions/download-artifact@v4
        with:
          name: frontend-out
          path: web/out
```

- [ ] **Step 3: 在 `build` job 的 `docker/build-push-action` 步骤加 `build-args`**

在 `docker/build-push-action@v6` 的 `with:` 块里加一行 `build-args`：

```yaml
      - name: Build and push by digest
        id: build
        uses: docker/build-push-action@v6
        with:
          context: .
          platforms: ${{ matrix.platform }}
          push: true
          build-args: FRONTEND_SOURCE=frontend-prebuilt
          labels: ${{ steps.meta.outputs.labels }}
          outputs: type=image,name=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }},push-by-digest=true,name-canonical=true,push=true
          cache-from: type=gha,scope=${{ matrix.platform-pair }}
          cache-to: type=gha,mode=max,scope=${{ matrix.platform-pair }}
```

- [ ] **Step 4: 让 `build` job 依赖 `frontend` job**

在 `build` job 的 `runs-on:` 之前加 `needs:`：

```yaml
  build:
    needs:
      - frontend
    env:
      BRANCH_TAG: ""
      PLATFORM_PAIR: ""
```

- [ ] **Step 5: 验证 YAML 语法**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/docker-publish.yml'))" && echo "YAML OK"`
Expected: `YAML OK`

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/docker-publish.yml
git commit -m "ci: add frontend build cache job and pass prebuilt frontend to docker build"
```

---

### Task 5: k8s/deployment.yaml 改造（distroless 兼容）

**Files:**
- Modify: `k8s/deployment.yaml`

两个必须修复的问题：
1. `preStop: exec: command: ["/bin/sh", "-c", "sleep 10"]` — distroless 无 shell
2. `/app/logs` emptyDir 卷 — nonroot (uid 65532) 无写权限

- [ ] **Step 1: 检查 k3s 版本是否支持 K8s 原生 sleep action**

通过 `update-prod-config` skill 或直接 SSH 到 `api.lvlvko.top` 查询：

Run: `ssh api.lvlvko.top "k3s --version"`
Expected: k3s version >= v1.30.0（K8s `sleep` lifecycle action 在 1.30 进入 beta）

记下版本号，决定 Step 2 走主路径还是 fallback：
- k3s >= 1.30 → Step 2 用 K8s 原生 `sleep` action（主路径）
- k3s < 1.30 → Step 2 保持原 `exec` preStop，但需回到 Task 3 把 Dockerfile final 阶段改为 `gcr.io/distroless/static-debian12:debug-nonroot`（含 busybox shell，约多 2MB），并保留 `USER nonroot:nonroot`

- [ ] **Step 2: 修改 `k8s/deployment.yaml` — preStop 改为 K8s 原生 sleep action**

将：
```yaml
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 10"]
```

改为：
```yaml
        lifecycle:
          preStop:
            sleep:
              seconds: 10
```

K8s 原生 `sleep` action 不依赖容器内有 shell 或 sleep 二进制，由 kubelet 直接实现。

**Fallback（k3s < 1.30）**：保持 `exec: command: ["/bin/sh", "-c", "sleep 10"]` 不变，但 Dockerfile 的 final 阶段改为 `FROM gcr.io/distroless/static-debian12:debug-nonroot AS final`（debug 变体含 busybox shell，约多 2MB）。同时保留 `USER nonroot:nonroot`。

- [ ] **Step 3: 修改 `k8s/deployment.yaml` — 加 fsGroup 解决日志卷写权限**

在 `spec.template.spec` 下（`terminationGracePeriodSeconds` 之前或之后）加 `securityContext`：

```yaml
    spec:
      securityContext:
        fsGroup: 65532
      terminationGracePeriodSeconds: 660
      containers:
```

`fsGroup: 65532` 让 emptyDir 卷的文件属主为 uid 65532（distroless nonroot），app 可写 `/app/logs`。

- [ ] **Step 4: 验证 YAML 语法**

Run: `python3 -c "import yaml; yaml.safe_load(open('k8s/deployment.yaml'))" && echo "YAML OK"`
Expected: `YAML OK`

- [ ] **Step 5: 验证 k8s dry-run（如果有 kubectl）**

Run: `kubectl apply --dry-run=client -f k8s/deployment.yaml 2>&1 || echo "kubectl not available, skip"`
Expected: `deployment.apps/aris-proxy-api configured (dry run)` 或 `kubectl not available, skip`

- [ ] **Step 6: Commit**

```bash
git add k8s/deployment.yaml
git commit -m "fix(k8s): use native sleep preStop and add fsGroup for distroless nonroot"
```

---

### Task 6: 本地端到端验证

**Files:**
- 无修改，纯验证

- [ ] **Step 1: 完整本地构建（从源码）**

Run: `docker buildx build --platform linux/amd64 -t aris-proxy-api:e2e -f docker/dockerfile .`
Expected: 构建成功

- [ ] **Step 2: 运行容器并验证 /health**

Run: `docker run -d -p 18080:8080 -e TZ=Asia/Shanghai --name aris-e2e aris-proxy-api:e2e server start --host 0.0.0.0 --port 8080 && sleep 5 && curl -sf http://localhost:18080/health`
Expected: HTTP 200，`{"status":"ok"}`

- [ ] **Step 3: 验证 /ready 端点**

Run: `curl -sf http://localhost:18080/ready`
Expected: HTTP 200，`{"status":"ok"}`

- [ ] **Step 4: 验证前端静态文件服务**

Run: `curl -sf http://localhost:18080/web/ -o /dev/null -w "%{http_code}"`
Expected: HTTP 200

- [ ] **Step 5: 验证镜像无 shell（distroless 安全性）**

Run: `docker run --rm --entrypoint /bin/sh aris-proxy-api:e2e 2>&1 || true`
Expected: 报错 `exec: "/bin/sh": stat /bin/sh: no such file or directory`（证明无 shell）

- [ ] **Step 6: 验证镜像大小**

Run: `docker images aris-proxy-api:e2e --format "{{.Size}}"`
Expected: 约 10-20MB

- [ ] **Step 7: 清理测试容器**

Run: `docker stop aris-e2e && docker rm aris-e2e`
Expected: 清理成功

- [ ] **Step 8: 运行项目测试确保无回归**

Run: `go test -count=1 ./...`
Expected: 全部 PASS

- [ ] **Step 9: 运行 lint**

Run: `make lint`
Expected: lint 通过

---

## 自检清单

实现完成后逐项确认：

- [ ] `.dockerignore` 排除了 `web/node_modules/` 和 `web/.next/`，但**没有**排除 `web/out/`
- [ ] `main.go` 有 `import _ "time/tzdata"`
- [ ] Dockerfile 的 `ARG FRONTEND_SOURCE` 声明在第一个 FROM 之前
- [ ] Dockerfile 的 `frontend-builder` 和 `frontend-prebuilt` 产物路径统一为 `/app/out`
- [ ] Dockerfile 的 builder 阶段 `COPY --from=frontend-final /app/out`（不是 `/out`）
- [ ] Dockerfile 的 UPX 用 `-9`（不是 `--best --lzma`）
- [ ] Dockerfile 的 final 阶段是 `gcr.io/distroless/static-debian12:nonroot`
- [ ] Dockerfile 有 `USER nonroot:nonroot` 和 `ENTRYPOINT`
- [ ] CI workflow 有 `frontend` job 且 `build` job `needs: [frontend]`
- [ ] CI workflow 的 build job 有 `Download frontend artifact` 步骤
- [ ] CI workflow 的 build-push-action 有 `build-args: FRONTEND_SOURCE=frontend-prebuilt`
- [ ] k8s/deployment.yaml 有 `securityContext.fsGroup: 65532`
- [ ] k8s/deployment.yaml 的 preStop 不依赖 `/bin/sh`（用 K8s native sleep 或 debug-nonroot 镜像）
