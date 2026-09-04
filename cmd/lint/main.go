// Package main aris-proxy-api lint 专用轻量入口。
//
// 仅编译 lint 工具链（lintconv/lintstatic 及其轻量依赖），不引入服务端依赖图，
// 避免 go run ./cmd/server 每次链接全量二进制（fiber/sonic/fx + embed dist）导致的高耗时。
// 用法：go run ./cmd/lint [all|conv|static] [paths...]，缺省 all + ./...。
package main

import (
	"os"
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintstatic"
)

// lint 运行模式
const (
	lintModeAll    = "all"
	lintModeConv   = "conv"
	lintModeStatic = "static"
)

func main() {
	mode, paths := parseArgs(os.Args[1:])
	if run(mode, paths) {
		os.Exit(1)
	}
}

// parseArgs 解析命令行参数：首参数若为模式关键字（all/conv/static）则作为模式，其余均为扫描路径。
func parseArgs(argv []string) (mode string, paths []string) {
	if len(argv) > 0 {
		switch argv[0] {
		case lintModeAll, lintModeConv, lintModeStatic:
			return argv[0], argv[1:]
		}
	}
	return lintModeAll, argv
}

// run 执行指定模式的 lint，返回是否存在检查失败。
func run(mode string, paths []string) bool {
	if len(paths) == 0 {
		paths = []string{constant.GoAllPackagesPattern}
	}
	switch mode {
	case lintModeConv:
		convResult := lintconv.Run(paths)
		convResult.Log()
		return convResult.ErrorCount() > 0
	case lintModeStatic:
		staticResult := lintstatic.Run(paths)
		staticResult.Log()
		return staticResult.Err != nil
	default:
		return runAll(paths)
	}
}

// runAll 并发执行 conv 与 static 两阶段，按 conv → static 顺序输出结果。
func runAll(paths []string) bool {
	var (
		wg           sync.WaitGroup
		convResult   lintconv.Result
		staticResult lintstatic.Result
	)
	wg.Go(func() { convResult = lintconv.Run(paths) })
	wg.Go(func() { staticResult = lintstatic.Run(paths) })
	wg.Wait()

	convResult.Log()
	staticResult.Log()
	return convResult.ErrorCount() > 0 || staticResult.Err != nil
}
