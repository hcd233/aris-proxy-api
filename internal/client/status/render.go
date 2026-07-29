package status

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Render 将检查结果渲染为五节面板
func Render(w io.Writer, r *Report) error {
	indent := constant.ClientUIRowIndent
	lines := []string{constant.ClientUIStatusTitle, ""}

	lines = append(lines, ui.SectionTitle(constant.ClientUISectionServer))
	switch {
	case !r.ConfigFound:
		lines = append(lines, indent+ui.CheckRowWarn(constant.ClientUIStatusNotInitialized, constant.ClientUIStatusRunInitHint))
	case r.ServerOK:
		lines = append(lines, indent+ui.CheckRowOK(r.Host, fmt.Sprintf(constant.TraceClientReachableFormat, r.ServerLatency.Round(time.Millisecond))))
	default:
		lines = append(lines, indent+ui.CheckRowFail(r.Host, r.ServerErr))
	}

	lines = append(lines, ui.SectionTitle(constant.ClientUISectionAuth))
	switch {
	case !r.ConfigFound:
		lines = append(lines, indent+ui.CheckRowWarn(constant.ClientUIStatusNotConfigured, ""))
	case r.AuthOK:
		lines = append(lines, indent+ui.CheckRowOK(constant.ClientUIStatusKeyValid, r.AuthMaskedKey))
	default:
		lines = append(lines, indent+ui.CheckRowFail(constant.ClientUIStatusKeyInvalid, r.AuthErr))
	}

	agent := r.Agent
	if agent == "" {
		agent = constant.TraceClientAgentCodex
	}
	hooksDetail := fmt.Sprintf(constant.ClientUIStatusHooksFormat, r.HooksFound, r.HooksTotal)
	lines = append(lines, ui.SectionTitle(constant.ClientUISectionAgent))
	if r.HooksFound == r.HooksTotal {
		lines = append(lines, indent+ui.CheckRowOK(agent, hooksDetail))
	} else {
		lines = append(lines, indent+ui.CheckRowWarn(agent, hooksDetail+constant.ClientUIStatusHooksMissingSuffix+strings.Join(r.HooksMissing, constant.ClientUISeparatorComma)))
	}

	lines = append(lines, ui.SectionTitle(constant.ClientUISectionQueue))
	if r.PendingCount == 0 && r.RejectedCount == 0 {
		lines = append(lines, indent+ui.CheckRowOK(constant.ClientUIStatusQueueClear, ""))
	} else {
		lines = append(lines, indent+ui.CheckRowWarn(fmt.Sprintf(constant.ClientUIStatusQueueFormat, r.PendingCount, humanizeBytes(r.PendingBytes), r.RejectedCount), ""))
	}

	lines = append(lines, ui.SectionTitle(constant.ClientUISectionDiagnostics))
	if r.RecentErrors == 0 {
		lines = append(lines, indent+ui.CheckRowOK(constant.ClientUIStatusNoErrors, ""))
	} else {
		lines = append(lines, indent+ui.CheckRowWarn(fmt.Sprintf(constant.ClientUIStatusRecentErrorsFormat, r.RecentErrors), fmt.Sprintf(constant.ClientUIStatusLogsHintFormat, r.LogDir)))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n")) //nolint:errcheck // best-effort stdout
	return err
}

// RenderJSON 将检查结果渲染为机器可读 JSON（--json）
func RenderJSON(w io.Writer, r *Report) error {
	data, err := sonic.MarshalIndent(newJSONReport(r), "", constant.TraceClientJSONIndent)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode status report")
	}
	_, err = fmt.Fprintln(w, string(data)) //nolint:errcheck // best-effort stdout
	return err
}

// humanizeBytes 人类可读字节数（B/KB/MB）
func humanizeBytes(size int64) string {
	switch {
	case size >= 1<<20:
		return fmt.Sprintf(constant.ClientUIBytesMBFormat, float64(size)/float64(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf(constant.ClientUIBytesKBFormat, float64(size)/float64(1<<10))
	default:
		return fmt.Sprintf(constant.ClientUIBytesBFormat, size)
	}
}

type jsonReport struct {
	ConfigFound     bool     `json:"configFound"`
	Host            string   `json:"host,omitempty"`
	Agent           string   `json:"agent,omitempty"`
	ServerOK        bool     `json:"serverOk"`
	ServerLatencyMs int64    `json:"serverLatencyMs,omitempty"`
	ServerErr       string   `json:"serverErr,omitempty"`
	AuthOK          bool     `json:"authOk"`
	AuthMaskedKey   string   `json:"authMaskedKey,omitempty"`
	AuthErr         string   `json:"authErr,omitempty"`
	HooksFound      int      `json:"hooksFound"`
	HooksTotal      int      `json:"hooksTotal"`
	HooksMissing    []string `json:"hooksMissing,omitempty"`
	PendingCount    int      `json:"pendingCount"`
	PendingBytes    int64    `json:"pendingBytes"`
	RejectedCount   int      `json:"rejectedCount"`
	RecentErrors    int      `json:"recentErrors"`
}

func newJSONReport(r *Report) *jsonReport {
	return &jsonReport{
		ConfigFound:     r.ConfigFound,
		Host:            r.Host,
		Agent:           r.Agent,
		ServerOK:        r.ServerOK,
		ServerLatencyMs: r.ServerLatency.Milliseconds(),
		ServerErr:       r.ServerErr,
		AuthOK:          r.AuthOK,
		AuthMaskedKey:   r.AuthMaskedKey,
		AuthErr:         r.AuthErr,
		HooksFound:      r.HooksFound,
		HooksTotal:      r.HooksTotal,
		HooksMissing:    r.HooksMissing,
		PendingCount:    r.PendingCount,
		PendingBytes:    r.PendingBytes,
		RejectedCount:   r.RejectedCount,
		RecentErrors:    r.RecentErrors,
	}
}
