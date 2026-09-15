// Package main aris 客户端入口。
package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	// 根 context 只在入口创建：cobra 之前 Context() 为 nil，下游不得自行 context.Background()
	if err := execute(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
