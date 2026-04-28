package lintstatic

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"go.uber.org/zap"
)

type Result struct {
	Output string
	Err    error
}

// Run 执行 go vet 和 staticcheck（若已安装）静态分析。
// 默认扫描 ./...，可通过 args 指定其他路径。
func Run(args []string) Result {
	if len(args) == 0 {
		args = []string{constant.GoAllPackagesPattern}
	}

	var out strings.Builder
	var hasErr bool

	// go vet
	vetCmd := exec.Command(constant.GoCommand, append([]string{constant.GoVetCommand}, args...)...)
	vetOut, vetErr := vetCmd.CombinedOutput()
	if len(vetOut) > 0 {
		out.Write(vetOut)
		out.WriteByte('\n')
	}
	if vetErr != nil {
		hasErr = true
	}

	// staticcheck
	if staticcheckPath, lookErr := exec.LookPath(constant.StaticcheckCommand); lookErr == nil {
		scCmd := exec.Command(staticcheckPath, args...)
		scOut, scErr := scCmd.CombinedOutput()
		if len(scOut) > 0 {
			out.Write(scOut)
			out.WriteByte('\n')
		}
		if scErr != nil {
			hasErr = true
		}
	} else {
		out.WriteString("[lintstatic] staticcheck not found in PATH, skipping. Install with: go install honnef.co/go/tools/cmd/staticcheck@latest\n")
	}

	res := Result{Output: out.String()}
	if hasErr {
		res.Err = fmt.Errorf(constant.StaticChecksFailedMessage)
	}
	return res
}

// Log 使用 zap logger 按行输出静态分析结果，替代直接 fmt.Print。
func (r Result) Log(logger *zap.Logger) {
	if strings.TrimSpace(r.Output) == "" {
		logger.Info("[LintStatic] All static checks passed!")
		return
	}
	for _, line := range strings.Split(r.Output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, ":") {
			logger.Warn("[LintStatic] Static check issue", zap.String("detail", line))
		} else {
			logger.Info("[LintStatic] Static check info", zap.String("detail", line))
		}
	}
}
