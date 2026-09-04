package lintstatic

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

type Result struct {
	Output string
	Err    error
}

// Run 执行 golangci-lint（若已安装）静态分析。
// govet 与 staticcheck 已作为 golangci-lint 内置 linter 启用（见 .golangci.yml），
// 与 CI（golangci-lint-action）覆盖一致，无需再单独跑 go vet / staticcheck 进程。
// 默认扫描 ./...，可通过 args 指定其他路径。
func Run(args []string) Result {
	if len(args) == 0 {
		args = []string{constant.GoAllPackagesPattern}
	}

	glPath := resolveGolangciLint()
	if glPath == "" {
		return Result{
			Output: "[lintstatic] golangci-lint not found in PATH or $(go env GOPATH)/bin, skipping. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest\n",
		}
	}

	glCmd := exec.Command(glPath, append([]string{constant.GolangciLintRunCommand}, args...)...) //nolint:gosec,noctx // args are trusted package paths
	glOut, glErr := glCmd.CombinedOutput()
	res := Result{}
	if len(glOut) > 0 {
		res.Output = string(glOut) + "\n"
	}
	if glErr != nil {
		res.Err = ierr.New(ierr.ErrInternal, constant.StaticChecksFailedMessage)
	}
	return res
}

// Log 使用 zap logger 按行输出静态分析结果，替代直接 fmt.Print。
func (r Result) Log() {
	log := logger.Logger()
	if strings.TrimSpace(r.Output) == "" {
		log.Info("[LintStatic] All static checks passed!")
		return
	}
	for line := range strings.SplitSeq(r.Output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, ":") {
			log.Warn("[LintStatic] Static check issue", zap.String("detail", line))
		} else {
			log.Info("[LintStatic] Static check info", zap.String("detail", line))
		}
	}
}

func resolveGolangciLint() string {
	if p, err := exec.LookPath(constant.GolangciLintCommand); err == nil {
		return p
	}
	if gobin := os.Getenv(constant.GobinEnvKey); gobin != constant.ZeroString {
		p := filepath.Join(gobin, constant.GolangciLintCommand)
		if info, err := os.Stat(p); err == nil && info.Mode()&constant.GopathBinFileMode != 0 { //nolint:gosec // gobin comes from GOBIN env var
			return p
		}
	}
	if out, err := exec.Command(constant.GoCommand, constant.GoEnvCommand, constant.GoEnvKeyGOPATH).Output(); err == nil { //nolint:gosec,noctx // resolving golangci-lint from GOPATH is safe, CLI tool without context
		p := filepath.Join(strings.TrimSpace(string(out)), constant.GopathBinSubDir, constant.GolangciLintCommand)
		if info, err := os.Stat(p); err == nil && info.Mode()&constant.GopathBinFileMode != 0 { //nolint:gosec // path from GOPATH env var
			return p
		}
	}
	return ""
}
