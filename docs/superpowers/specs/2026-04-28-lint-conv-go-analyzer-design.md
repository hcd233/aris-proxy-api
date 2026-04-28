# lint-conv Go 静态检查器设计

## 背景

当前 `make lint-conv` 通过 `script/lint-conventions.sh` 执行项目自定义规范检查。脚本大量依赖 `grep`、`find`、`awk` 对源码文本做正则匹配，容易出现误报、漏报，也难以识别 Go 语法结构、导入别名、类型信息和注释上下文。

项目已经在 `.golangci.yml` 中迁移了一部分通用规则。剩余规则主要是项目私有约束，适合迁移为仓库内 Go 静态检查器。

## 目标

- 保留 `make lint-conv` 作为开发者和 pre-commit 使用的入口。
- 将现有 shell 中的项目特有规则迁移到基于 Go AST 和类型信息的检查器。
- 避免继续用正则扫描 Go 源码判断语义。
- 输出清晰的文件、行号、规则说明，并保持有错误时非零退出。
- `script/lint-conventions.sh` 保留为兼容包装，但实际执行 Go 检查器。

## 非目标

- 不把项目私有规则改造成 golangci-lint 插件。本地安装和版本耦合成本较高。
- 不重做 `.golangci.yml` 已覆盖的通用规则。
- 不在本次改造中放宽或删除现有项目规范，除非 AST 化后证明原规则是明显误报。

## 方案

新增仓库内命令 `cmd/lintconv`，使用 Go 标准库 `go/ast`、`go/parser`、`go/token` 和必要时的 `golang.org/x/tools/go/packages` 加载源码。检查器按规则分组输出：错误处理、常量定义、日志、测试规范、死代码、命名、架构、魔法值、匿名 struct。

`make lint-conv` 继续调用 `script/lint-conventions.sh`。该脚本改为轻量包装，执行：

```bash
go run ./cmd/lintconv ./...
```

检查器内部区分 error 和 warning。只存在 warning 时退出码为 0；存在 error 时退出码为 1，保持现有行为。

## 规则迁移

- 错误处理：识别 `constant.ErrXxx` selector，排除定义文件。
- 常量定义：识别 constant/enum 包内 `const X = pkg.Y` 形式的转发常量。
- 日志：识别 `logger.Info/Error/Warn/Debug` 调用，检查第一个字符串参数是否使用 `[Module]` 前缀；识别 `zap.String` 中疑似敏感字段未使用 `util.MaskSecret`。
- 测试规范：用路径和 AST/import 检查测试文件位置、testify 导入、`time.Sleep` 调用。
- 死代码：基于注释文本做保守检查，仅对明显注释掉的 Go 语句告警。
- 命名：识别局部变量声明和短变量声明中 `List/Map/Slice/Array` 后缀，保留现有白名单。
- 架构：基于导入路径和调用表达式检查 handler 直连 DAO/DB、接口层创建根 context、domain 依赖 infrastructure/dto/internal/util、application 引用废弃包。
- 透传封装：基于函数体 AST 识别单行 `return receiver.Other(...)` 的 wrapper 方法。
- 魔法数字、魔法 duration、魔法字符串：基于 literal 节点和路径白名单判断，排除 import、const、日志参数和允许目录。
- 匿名 struct：识别非测试目录中的匿名 struct 类型字面量，排除命名 type 声明。

## 测试

新增 `test/unit/lintconv`，使用 fixture 文件覆盖典型违规与允许场景。测试直接调用检查器包的 API，不通过 shell。至少覆盖：日志前缀、匿名 struct、魔法值、架构导入、测试目录规则、透传封装。

完成后运行：

- `go test -count=1 ./test/unit/lintconv/`
- `make lint-conv`
- `go test -count=1 ./...`

## 风险

- AST 化后可能发现当前代码存在正则漏掉的违规，需要按规则修复或增加明确白名单。
- 部分规则原本依赖文本位置，迁移时要保持保守，避免一次引入大量主观告警。
- `go run ./cmd/lintconv` 首次执行会比纯 shell 慢，但能换来更准确的语义检查。
