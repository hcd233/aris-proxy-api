package lintconv

import (
	"fmt"
	"io"
	"sort"

	"go.uber.org/zap"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Diagnostic struct {
	Rule     string
	Severity Severity
	Path     string
	Line     int
	Message  string
}

type Result struct {
	Diagnostics []Diagnostic
}

func (r Result) ErrorCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			count++
		}
	}
	return count
}

func (r Result) WarningCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

func (r Result) Print(w io.Writer) {
	diagnostics := append([]Diagnostic(nil), r.Diagnostics...)
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Rule < diagnostics[j].Rule
	})
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(w, "%s:%d: [%s] %s: %s\n", diagnostic.Path, diagnostic.Line, diagnostic.Severity, diagnostic.Rule, diagnostic.Message)
	}
	if r.ErrorCount() == 0 && r.WarningCount() == 0 {
		fmt.Fprintln(w, "All convention checks passed!")
		return
	}
	fmt.Fprintf(w, "%d error(s), %d warning(s)\n", r.ErrorCount(), r.WarningCount())
}

// Log 使用 zap logger 输出诊断结果，替代 Print 的 fmt 输出。
func (r Result) Log(logger *zap.Logger) {
	diagnostics := append([]Diagnostic(nil), r.Diagnostics...)
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Rule < diagnostics[j].Rule
	})
	for _, diagnostic := range diagnostics {
		logger.Warn("[LintConv] convention violation",
			zap.String("path", fmt.Sprintf("%s:%d", diagnostic.Path, diagnostic.Line)),
			zap.String("severity", string(diagnostic.Severity)),
			zap.String("rule", diagnostic.Rule),
			zap.String("message", diagnostic.Message),
		)
	}
	if r.ErrorCount() == 0 && r.WarningCount() == 0 {
		logger.Info("[LintConv] All convention checks passed!")
		return
	}
	logger.Info("[LintConv] convention check summary",
		zap.Int("errors", r.ErrorCount()),
		zap.Int("warnings", r.WarningCount()),
	)
}
