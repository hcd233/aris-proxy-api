# 常用命令

> **使用场景**：需要构建、测试、lint、清理、缓存预热时作为命令参考。

## 硬性约束

- **非用户明确要求，不允许新增 cobra 命令**（`cmd/server/`、`cmd/client/` 下的 `cobra.Command`）。
  需要新的运维动作时，优先考虑复用现有命令、cron 任务或环境变量/配置开关，并向用户说明方案。

- 构建：`make build`（含前端 + 服务端 + 四平台客户端）；单独构建服务端：`make build-server`；单独构建客户端：`make build-client`；四平台交叉编译：`make build-client-all`
- 直接构建服务端：`go build ./cmd/server`；直接构建客户端：`go build ./cmd/client`
- 规范扫描：`make lint`（conv + static 并发执行，底层使用轻量入口 `go run ./cmd/lint`，不编译服务端二进制）；单独跑规范扫描：`make lint-conv`，静态检查：`make lint-static`；旧入口 `go run ./cmd/server lint ...` 仍可用但会全量编译服务端，耗时高
- 全量测试：`make test`（等价于 `go test -count=1 ./cmd/... ./internal/... ./test/...`，显式排除 `web/node_modules` 中的嵌套 Go 目录）
- 聚焦测试：`go test -v -count=1 -run TestFunctionName ./test/unit/<topic>/` 或 `./test/e2e/<topic>/`
- 前端 lint：`cd web && npm run lint`（或 `make web-lint`）；自动修复：`npm run lint:fix`
- 前端单测：`cd web && npm run test`（vitest）
- 前端格式化：`cd web && npm run format`（Prettier 写入，或 `make web-format`）；格式检查：`npm run format:check`（CI 用，或 `make web-format-check`）
- 前端构建（同时同步到 `internal/web/dist/`）：`make web-build`；清理产物：`make web-clean`
- 前端运行时验证：`cd web && npx next dev --port <port>`，再按 `next-dev-loop` skill 交叉校验 `/_next/mcp` 与 `agent-browser`（入口 `http://localhost:<port>/web/`，注意 `basePath`）。详见 [web-frontend.md](web-frontend.md) 的「运行时验证」
- 探测 `/_next/mcp`：`curl -s -X POST http://localhost:<port>/_next/mcp -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | sed -n 's/^data: //p'`（回包是 SSE）
- 停掉前端 dev server：`lsof -nP -ti tcp:<port> -sTCP:LISTEN | xargs kill`（本机 `pkill -f` 会匹配到调用者自身而卡死）
- 生产构建会自动包含前端：`make build` 在编译 Go 之前先跑 `web-build`
- UPX 极致压缩：`make build-upx`（需安装 upx）
- 编译缓存预热：`make warm-cache`（CI 加速）
- 全量清理：`make clean-all`（含 `go clean -cache`）
