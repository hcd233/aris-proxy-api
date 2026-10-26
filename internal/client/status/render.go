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

// Render 将检查结果渲染为三节面板
func Render(w io.Writer, r *Report) error {
	indent := constant.ClientUIRowIndent
	lines := []string{constant.ClientUIStatusTitle, ""}

	lines = append(lines, ui.SectionTitle(constant.ClientUISectionServer))
	switch {
	case !r.ConfigFound:
		lines = append(lines, indent+ui.CheckRowWarn(constant.ClientUIStatusNotInitialized, constant.ClientUIStatusRunInitHint))
	case r.ServerOK:
		lines = append(lines, indent+ui.CheckRowOK(r.Host, fmt.Sprintf(constant.ArisClientReachableFormat, r.ServerLatency.Round(time.Millisecond))))
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

	lines = append(lines, ui.SectionTitle(constant.ClientUISectionProviders))
	if len(r.ProvidersFound) > 0 {
		detail := strings.Join(r.ProvidersFound, constant.ClientUISeparatorComma)
		if len(r.ProvidersMissing) > 0 {
			lines = append(lines, indent+ui.CheckRowWarn(constant.ClientUIStatusProvidersLabel, detail+constant.ClientUIStatusProvidersMissingPrefix+strings.Join(r.ProvidersMissing, constant.ClientUISeparatorComma)))
		} else {
			lines = append(lines, indent+ui.CheckRowOK(constant.ClientUIStatusProvidersLabel, detail))
		}
	} else {
		lines = append(lines, indent+ui.CheckRowWarn(constant.ClientUIStatusProvidersLabel, constant.ClientUIStatusProvidersNoneHint))
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n")) //nolint:errcheck // best-effort stdout
	return err
}

// RenderJSON 将检查结果渲染为机器可读 JSON（--json）
func RenderJSON(w io.Writer, r *Report) error {
	data, err := sonic.MarshalIndent(newJSONReport(r), "", constant.ArisClientJSONIndent)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode status report")
	}
	_, err = fmt.Fprintln(w, string(data)) //nolint:errcheck // best-effort stdout
	return err
}

type jsonReport struct {
	ConfigFound      bool     `json:"configFound"`
	Host             string   `json:"host,omitempty"`
	ServerOK         bool     `json:"serverOk"`
	ServerLatencyMs  int64    `json:"serverLatencyMs,omitempty"`
	ServerErr        string   `json:"serverErr,omitempty"`
	AuthOK           bool     `json:"authOk"`
	AuthMaskedKey    string   `json:"authMaskedKey,omitempty"`
	AuthErr          string   `json:"authErr,omitempty"`
	ProvidersFound   []string `json:"providersFound,omitempty"`
	ProvidersMissing []string `json:"providersMissing,omitempty"`
}

func newJSONReport(r *Report) *jsonReport {
	return &jsonReport{
		ConfigFound:      r.ConfigFound,
		Host:             r.Host,
		ServerOK:         r.ServerOK,
		ServerLatencyMs:  r.ServerLatency.Milliseconds(),
		ServerErr:        r.ServerErr,
		AuthOK:           r.AuthOK,
		AuthMaskedKey:    r.AuthMaskedKey,
		AuthErr:          r.AuthErr,
		ProvidersFound:   r.ProvidersFound,
		ProvidersMissing: r.ProvidersMissing,
	}
}
