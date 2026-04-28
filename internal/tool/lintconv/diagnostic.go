package lintconv

import (
	"fmt"
	"sort"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// Diagnostic 诊断结果
//
//	@author centonhuang
//	@update 2026-04-28 20:30:24
type Diagnostic struct {
	Rule     string
	Severity enum.Severity
	Path     string
	Line     int
	Message  string
}

// Result 诊断结果
//
//	@author centonhuang
//	@update 2026-04-28 20:30:29
type Result struct {
	Diagnostics []Diagnostic
}

// ErrorCount 错误数量
//
//	@receiver r Result
//	@return int
//	@author centonhuang
//	@update 2026-04-28 20:30:34
func (r Result) ErrorCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == enum.SeverityError {
			count++
		}
	}
	return count
}

// WarningCount 警告数量
//
//	@receiver r Result
//	@return int
//	@author centonhuang
//	@update 2026-04-28 20:30:39
func (r Result) WarningCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == enum.SeverityWarning {
			count++
		}
	}
	return count
}

// Log 使用 zap logger 输出诊断结果，替代 Print 的 fmt 输出。
func (r Result) Log() {
	logger := logger.Logger()
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
		logFunc := logger.Info
		switch diagnostic.Severity {
		case enum.SeverityError:
			logFunc = logger.Error
		case enum.SeverityWarning:
			logFunc = logger.Warn
		default:
		}
		logFunc(constant.ConvCheckLogPrefix+constant.ConvCheckLogViolation,
			zap.String("path", fmt.Sprintf("%s:%d", diagnostic.Path, diagnostic.Line)),
			zap.String("rule", diagnostic.Rule),
			zap.String("message", diagnostic.Message),
		)
	}
	if r.ErrorCount() == 0 && r.WarningCount() == 0 {
		logger.Info(constant.ConvCheckLogPrefix + constant.ConvCheckLogPassed)
		return
	}
	logger.Info(constant.ConvCheckLogPrefix+constant.ConvCheckLogSummary,
		zap.Int("errors", r.ErrorCount()),
		zap.Int("warnings", r.WarningCount()),
	)
}
