# 常用命令

> **使用场景**：需要构建、测试、lint、清理、缓存预热时作为命令参考。

- 构建：`make build`（含前端 + 服务端 + 四平台客户端）；单独构建服务端：`make build-server`；单独构建客户端：`make build-client`；四平台交叉编译：`make build-client-all`
- 直接构建服务端：`go build ./cmd/server`；直接构建客户端：`go build ./cmd/client`
- 规范扫描：`make lint`（执行 `lint-conv` + `lint-static` 两阶段，底层使用 `go run ./cmd/server lint ...`）
- 全量测试：`make test` 或 `go test -count=1 ./...`
- 聚焦测试：`go test -v -count=1 -run TestFunctionName ./test/unit/<topic>/` 或 `./test/e2e/<topic>/`
- 前端 lint：`cd web && npm run lint`
- 前端构建（同时同步到 `internal/web/dist/`）：`make web-build`；清理产物：`make web-clean`
- 生产构建会自动包含前端：`make build` 在编译 Go 之前先跑 `web-build`
- UPX 极致压缩：`make build-upx`（需安装 upx）
- 编译缓存预热：`make warm-cache`（CI 加速）
- 全量清理：`make clean-all`（含 `go clean -cache`）
