# fgprof Web 可视化工具设计

## 目标

封装 `make fgprof` 命令，从远程 API 拉取 fgprof profile 数据并打开 Web 可视化界面。

## 交互流程

1. 用户执行 `make fgprof`
2. 脚本提示输入 fgprof 端点 URL（交互式）
3. 使用 `go tool pprof` 拉取 30 秒 profile
4. 自动打开浏览器，显示火焰图和调用图

## 实现方案

### 命令设计

```make
## fgprof: 从远程服务拉取 fgprof profile 并打开 Web 可视化（火焰图+调用图）
fgprof:
	@read -p "Enter fgprof endpoint URL (e.g., http://localhost:8080): " URL; \
	if [ -z "$$URL" ]; then \
		echo "URL is required"; \
		exit 1; \
	fi; \
	echo "Fetching fgprof profile from $$URL/debug/fgprof?seconds=30..."; \
	go tool pprof "$$URL/debug/fgprof?seconds=30"
```

### 使用方式

```bash
# 交互式（会提示输入 URL）
make fgprof
```

### 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| URL | 交互式输入 | fgprof 服务端点，不含 `/debug/fgprof?seconds=XX` |
| seconds | `30` | profiling 时长（秒） |

## 效果

运行后自动打开浏览器，支持：
- **火焰图（flame graph）**：函数调用栈热力图
- **调用图（call graph）**：函数调用关系图
- **top**：CPU 占用排名
- **tree**：调用树

## 依赖

- `go tool pprof`（Go 内置，已安装）
- fgprof 端点路径：`/debug/fgprof?seconds=30`

## 文件变更

- `Makefile`：新增 `fgprof` 目标
