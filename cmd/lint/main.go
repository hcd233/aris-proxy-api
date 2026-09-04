// Package main aris-proxy-api lint 入口。
//
// 与 cmd/server、cmd/client 平行的独立入口，仅编译 lint 工具链
// （lintconv/lintstatic 及其轻量依赖），不引入服务端依赖图，保证 lint 秒级执行。
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
