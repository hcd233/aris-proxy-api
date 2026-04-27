package main

import (
	"os"

	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
)

func main() {
	result := lintconv.Run(os.Args[1:])
	result.Print(os.Stdout)
	if result.ErrorCount() > 0 {
		os.Exit(1)
	}
}
