# Docker 构建优化设计

## 背景

当前 `docker/dockerfile` 采用三阶段多阶段构建（frontend-builder → builder → final），已具备多阶段、Go cache mount、UPX `--best --lzma`、`-trimpath -s -w`、`CGO_ENABLED=0`、原生 ARM runner、GHA 缓存等优化。CI workflow 用原生 amd64/arm64 runner 并行构建，digest 合并 manifest。

但仍存在以下可优化点：

1. **`.dockerignore` 未排除 `web/node_modules`（1.1G）/`web/.next`（51M）/`web/out`（8.8M）**：本地构建会把 1G+ 文件发给 daemon。CI 因 fresh checkout 不受影响。
2. **前端 `npm ci` 无 npm cache mount**：每次构建重新下载全部依赖。
3. **UPX `--best --lzma` 是构建速度主要损耗**：对大体积 Go 二进制耗时可达 30s~2min，LZMA 增益很小但慢。
4. **纯 Go 改动也重建前端**：两个平台各自重建前端，无跨平台共享。
5. **最终镜像用 `alpine:3.22` + `tzdata` + `curl`**：curl 仅手动 debug 用；tzdata 可内嵌进二进制；基础镜像可更小更安全。

## 目标

两个独立方向，分别交付：

- **方向一（提速）**：A 增量优化 + B 前端产物缓存
- **方向二（瘦身）**：E distroless/static + tzdata 内嵌

## 当前构建瓶颈分析

| 阶段 | 现状 | 问题 |
|------|------|------|
| Build context | `.dockerignore` 未排除 `web/node_modules`/`web/.next`/`web/out` | 本地构建发 1G+ context |
| 前端 `npm ci` | 无 npm cache mount | 每次重下依赖 |
| Go build | 已有 cache mount ✓ | 已最优 |
| UPX `--best --lzma` | 最强档 | 速度主要损耗 |
| 最终镜像 | `alpine:3.22` + `tzdata` + `curl` | curl debug 用、tzdata 可内嵌、基础镜像可更小 |

## 方向一：提速方案

### A — Dockerfile + .dockerignore 增量优化

#### A1. `.dockerignore` 新增排除

```
web/node_modules/
web/.next/
```

注意：**不排除 `web/out/`**。原因：CI 模式下 frontend job 下载产物到 `web/out/`，docker build 需要它在 context 里供 `frontend-prebuilt` 阶段 COPY。如果排除 `web/out/`，CI 下载的产物不会进入 build context，`frontend-prebuilt` 阶段会失败。本地构建时 `web/out/` 虽然在 context 里（8.8M，可接受），但 `frontend-builder` 从源码构建会忽略它。

排除 `web/node_modules`（1.1G）和 `web/.next`（51M）是本地构建提速的主要来源。

#### A2. 前端阶段加 npm cache mount

```dockerfile
RUN --mount=type=cache,target=/root/.npm \
    npm ci
```

利用 buildx cache mount 复用 npm 下载缓存。

#### A3. UPX 降级 `--best --lzma` → `-9`

```dockerfile
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /go/bin/aris-proxy-api \
    && upx -9 /go/bin/aris-proxy-api
```

`-9` 是 UPX 最高纯数字档，去掉 `--lzma`。预期体积损失 5-10%，速度提升 3-5x。

### B — CI 前端产物缓存

#### B1. 新增 `frontend` job

```yaml
frontend:
  runs-on: ubuntu-latest
  outputs:
    cache-hit: ${{ steps.cache.outputs.cache-hit }}
  steps:
    - uses: actions/checkout@v4
    - name: Compute web hash
      id: hash
      run: echo "key=frontend-${{ hashFiles('web/**') }}" >> $GITHUB_OUTPUT
    - name: Cache frontend build
      id: cache
      uses: actions/cache@v4
      with:
        path: web/out
        key: ${{ steps.hash.outputs.key }}
    - name: Build frontend (on miss)
      if: steps.cache.outputs.cache-hit != 'true'
      run: cd web && npm ci && npm run build
    - name: Upload frontend artifact
      uses: actions/upload-artifact@v4
      with:
        name: frontend-out
        path: web/out
        retention-days: 1
```

#### B2. `build` 矩阵 job 下载前端产物并传 build-arg

```yaml
- name: Download frontend artifact
  uses: actions/download-artifact@v4
  with:
    name: frontend-out
    path: web/out

- name: Build and push
  uses: docker/build-push-action@v6
  with:
    build-args: FRONTEND_SOURCE=frontend-prebuilt
    # 其他参数不变
```

#### B3. Dockerfile 支持 build-arg 切换前端来源

```dockerfile
# 必须声明在第一个 FROM 之前
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

# 阶段1b: 使用预构建前端（CI 模式）
FROM scratch AS frontend-prebuilt
COPY web/out/ /app/out/

# 阶段1: 根据 build-arg 选择来源
FROM ${FRONTEND_SOURCE} AS frontend-final
```

本地构建不传 build-arg，走 `frontend-builder`；CI 传 `FRONTEND_SOURCE=frontend-prebuilt`，直接 COPY context 里的 `web/out/`。

注意：两个前端阶段的产物路径必须统一为 `/app/out`，builder 阶段统一从 `/app/out` 复制。

#### B4. 效果

- 纯 Go 改动 → frontend job cache 命中，build job 秒下产物，跳过 node 阶段
- 前端改动 → frontend job 重建一次，两个平台共享产物（原来每个平台各建一次）
- 本地构建 → 不传 build-arg，走完整 frontend-builder

## 方向二：瘦身方案

### E — distroless/static + tzdata 内嵌

#### E1. `main.go` 引入 `time/tzdata`

```go
import (
    _ "time/tzdata"
    // ... 其他 import
)
```

二进制 +450KB，UPX 后更少。让二进制自带时区数据，不再依赖系统 tzdata 包。

#### E2. 最终镜像改用 distroless

```dockerfile
FROM gcr.io/distroless/static-debian12:nonroot AS final

WORKDIR /app
COPY --from=builder /go/bin/aris-proxy-api /app/aris-proxy-api

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/aris-proxy-api"]
```

#### E3. distroless/static-debian12:nonroot 特性

- 自带 CA 证书：LLM 代理出站 HTTPS 到 OpenAI/Anthropic 不会 TLS 失败
- 无 shell、无 curl、无包管理器：攻击面最小
- 以 nonroot (uid 65532) 运行：更安全
- 专为静态二进制设计：`CGO_ENABLED=0` 匹配

#### E4. 移除的内容

- `alpine:3.22` 基础镜像
- `tzdata` 包（已内嵌进二进制）
- `curl` 包（debug 用 `kubectl debug` 临时容器替代）

#### E5. 预期镜像大小

约 10-15MB（distroless 基础 ~2MB + UPX 后二进制 ~8-12MB），相比当前约 25MB 减半。

## 最终 Dockerfile 全貌

```dockerfile
# 必须声明在第一个 FROM 之前
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

# 阶段1b: 使用预构建前端（CI 模式）
FROM scratch AS frontend-prebuilt
COPY web/out/ /app/out/

# 阶段1: 根据 build-arg 选择来源
FROM ${FRONTEND_SOURCE} AS frontend-final

# 阶段2: Go 构建
FROM golang:1.25.1-alpine3.22 AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache upx

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY main.go ./

# 复制前端构建产物（两个前端阶段路径统一为 /app/out）
COPY --from=frontend-final /app/out ./internal/web/dist

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /go/bin/aris-proxy-api \
    && upx -9 /go/bin/aris-proxy-api

# 阶段3: 运行时（distroless）
FROM gcr.io/distroless/static-debian12:nonroot AS final

WORKDIR /app
COPY --from=builder /go/bin/aris-proxy-api /app/aris-proxy-api

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/aris-proxy-api"]
```

## 需要验证的风险点与阻断问题

### 阻断问题（必须在实现时解决）

1. **preStop `/bin/sh` 与 distroless 不兼容**（k8s/deployment.yaml:44）：
   - 现状：`preStop: exec: command: ["/bin/sh", "-c", "sleep 10"]`
   - distroless/static 无 shell，此 hook 会失败，导致优雅关闭链路断裂。
   - 解决方案（需改 k8s/deployment.yaml）：
     - 方案 a：改用 HTTP preStop hook 调用 app 的 draining 端点（如果 app 有 `/drain` 或类似端点）
     - 方案 b：app 增加 `/drain` 端点，preStop 改为 `httpGet: path: /drain, port: 8080`
     - 方案 c：移除 preStop，仅依赖 `terminationGracePeriodSeconds: 660` + app 内部 draining 逻辑（需确认 readiness 探针在 SIGTERM 前能及时失败摘流）

2. **`/app/logs` 卷 + nonroot 写权限**（k8s/deployment.yaml:38-40, 68-70）：
   - 现状：app 写日志文件到 `config.LogDirPath`（默认 `./logs`），k8s 挂载 emptyDir 到 `/app/logs`
   - distroless `:nonroot` 以 uid 65532 运行，emptyDir 默认 root 拥有，nonroot 无写权限 → 日志写入失败
   - 解决方案（需改 k8s/deployment.yaml）：在 pod spec 加 `securityContext: fsGroup: 65532`，让 emptyDir 卷对 nonroot 可写

### 待验证项

3. **`time/tzdata` 兼容性**：确认 app 没有依赖读取 `/usr/share/zoneinfo/` 路径（`time/tzdata` 包会自动注册内嵌时区数据，通常透明兼容）。
4. **`FROM ${FRONTEND_SOURCE}` 语法**：Docker buildx 稳定语法支持 ARG 在 FROM 中使用，但需实际测试验证。
5. **CI cache key 稳定性**：`hashFiles('web/**')` 需确认 hash 稳定性，避免误命中或误 miss。

### k8s/deployment.yaml 必改动清单

本设计需要同步修改 `k8s/deployment.yaml`（用户已确认 CI 范围可改，k8s 部署配置作为部署配套改动）：

- pod spec 加 `securityContext: fsGroup: 65532`（解决日志卷写权限）
- preStop hook 改为不依赖 shell 的方案（解决 distroless 无 shell 问题）

## 不做的事（YAGNI）

- 不预构建 deps 基础镜像推到 GHCR（收益递减、维护成本高）
- 不用 Chainguard wolfi（distroless 已够小）
- 不拆分 amd64/arm64 的前端 job（一个 frontend job 足够）
- 不引入 `docker compose build` 的改动（只优化 CI 和 Dockerfile）
