# Makefile for aris-proxy-api

APP_NAME   := aris-proxy-api
MAIN       := main.go
OUTPUT     := $(APP_NAME)

# 并行编译：默认使用全部 CPU 核心
GOMAXPROCS ?= $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)

# 编译优化参数
# -s: 去除符号表  -w: 去除 DWARF 调试信息
LDFLAGS    := -s -w
# -trimpath: 去除编译路径信息（减小体积 + 安全）
BUILD_FLAGS := -trimpath -p $(GOMAXPROCS)

# golangci-lint release tag (https://github.com/golangci/golangci-lint/releases)
GOLANGCI_LINT_VERSION ?= v2.11.4

.PHONY: build build-upx build-dev build-debug clean test test-cover lint lint-conv lint-go fgprof help

## build: 生产构建（strip 符号）
build:
	CGO_ENABLED=0 go build \
		$(BUILD_FLAGS) \
		-ldflags="$(LDFLAGS)" \
		-o $(OUTPUT) $(MAIN)
	@echo "Built $(OUTPUT) ($$(du -h $(OUTPUT) | cut -f1))"

## build-upx: 极致压缩构建（strip + UPX，体积最小，需安装 upx）
build-upx: build
	upx --best --lzma $(OUTPUT)
	@echo "Compressed $(OUTPUT) ($$(du -h $(OUTPUT) | cut -f1))"

## build-dev: 开发构建（保留调试信息，最快编译速度）
build-dev:
	go build -p $(GOMAXPROCS) \
		-o $(OUTPUT) $(MAIN)
	@echo "Built $(OUTPUT) ($$(du -h $(OUTPUT) | cut -f1))"

## build-debug: 带完整调试信息的构建（用于 dlv 调试）
build-debug:
	go build -p $(GOMAXPROCS) \
		-gcflags="all=-N -l" \
		-o $(OUTPUT) $(MAIN)
	@echo "Built $(OUTPUT) ($$(du -h $(OUTPUT) | cut -f1))"

## warm-cache: 预热编译缓存（CI 首次运行后可加速后续编译）
warm-cache:
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags="$(LDFLAGS)" -o /dev/null $(MAIN)
	@echo "Build cache warmed"

## clean: 清理构建产物
clean:
	rm -f $(OUTPUT)

## clean-all: 清理构建产物和编译缓存
clean-all: clean
	go clean -cache

## test: 运行全量测试
test:
	go test -count=1 ./...

## test-cover: 带覆盖率的测试
test-cover:
	go test -count=1 -coverprofile=coverage.out ./test/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint-conv: 扫描项目编码规范
lint-conv:
	@bash script/lint-conventions.sh



## lint: 编码规范脚本 + golangci-lint
lint: lint-conv 

## fgprof: 从远程服务拉取 fgprof profile 并打开 Web 可视化（火焰图+调用图）
fgprof:
	@read -p "Enter fgprof endpoint URL (e.g., http://localhost:8080): " URL; \
	if [ -z "$$URL" ]; then \
		echo "URL is required"; \
		exit 1; \
	fi; \
	echo "Fetching fgprof profile from $$URL/debug/fgprof?seconds=30..."; \
	go tool pprof -http=:8081 "$$URL/debug/fgprof?seconds=30"

## help: 显示帮助
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
