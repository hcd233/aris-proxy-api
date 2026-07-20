package tracecli

import (
	"context"
	"fmt"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type CodexInstaller interface {
	Install(ctx context.Context, commandPath string) (backupPath string, err error)
}

type InitOptions struct {
	Host        string
	CommandPath string
}

type InitRunner struct {
	Terminal Terminal
	Config   ConfigStore
	Codex    CodexInstaller
	HTTP     *HTTPClient
	Paths    Paths
}

func (r InitRunner) Run(ctx context.Context, opts InitOptions) error {
	if r.Terminal == nil || !r.Terminal.Interactive() {
		return ierr.New(ierr.ErrValidation, constant.TraceClientInitNonInteractiveMessage)
	}
	host, err := normalizeHost(opts.Host)
	if err != nil {
		return err
	}

	r.Terminal.WriteLine(constant.TraceClientInitStepConnect)
	if err := r.connect(ctx, host); err != nil {
		return err
	}
	r.Terminal.WriteLine(constant.TraceClientInitConnected)

	r.Terminal.WriteLine(constant.TraceClientInitStepAgent)
	if err := r.selectAgent(); err != nil {
		return err
	}

	existing, err := r.Config.Load(ctx)
	if err != nil {
		return err
	}
	r.Terminal.WriteLine(constant.TraceClientInitStepAPIKey)
	apiKey, err := r.readAPIKey(ctx, host, existing.APIKey)
	if err != nil {
		return err
	}

	r.Terminal.WriteLine(constant.TraceClientInitStepHook)
	if _, err := r.Codex.Install(ctx, opts.CommandPath); err != nil {
		return err
	}
	config := Config{Host: host, Agent: constant.TraceClientAgentCodex, APIKey: apiKey}
	if err := r.Config.Save(ctx, config); err != nil {
		return err
	}

	r.Terminal.WriteLine(constant.TraceClientInitDone)
	r.Terminal.WriteLine(fmt.Sprintf(constant.TraceClientInitConfigFormat, r.Paths.ConfigFile()))
	r.Terminal.WriteLine(constant.TraceClientInitApprovalHint)
	return nil
}

func (r InitRunner) connect(ctx context.Context, host string) error {
	for {
		err := r.HTTP.CheckHealth(ctx, host)
		if err == nil {
			return nil
		}
		retry, readErr := r.Terminal.ReadLine(constant.TraceClientInitRetryPrompt)
		if readErr != nil {
			return readErr
		}
		if declined(retry) {
			return err
		}
	}
}

func (r InitRunner) selectAgent() error {
	for {
		agent, err := r.Terminal.ReadLine(constant.TraceClientInitAgentPrompt)
		if err != nil {
			return err
		}
		if agent == "" || strings.EqualFold(agent, constant.TraceClientAgentCodex) {
			return nil
		}
		r.Terminal.WriteLine(constant.TraceClientInitInvalidAgentMessage)
	}
}

func declined(answer string) bool {
	return strings.EqualFold(answer, constant.TraceClientNegativeShort) ||
		strings.EqualFold(answer, constant.TraceClientNegative)
}

func (r InitRunner) readAPIKey(ctx context.Context, host, current string) (string, error) {
	for {
		prompt := constant.TraceClientInitAPIKeyPrompt
		if current != "" {
			prompt = constant.TraceClientInitKeepAPIKeyPrompt
		}
		apiKey, err := r.Terminal.ReadSecret(prompt)
		if err != nil {
			return "", err
		}
		if apiKey == "" {
			apiKey = current
		}
		if apiKey == "" {
			r.Terminal.WriteLine(constant.TraceClientInitMissingAPIKeyMessage)
			continue
		}
		if err := r.HTTP.CheckAPIKey(ctx, host, apiKey); err == nil {
			return apiKey, nil
		}
		r.Terminal.WriteLine(constant.TraceClientInitAPIKeyFailed)
		retry, readErr := r.Terminal.ReadLine(constant.TraceClientInitAPIKeyRetryPrompt)
		if readErr != nil {
			return "", readErr
		}
		if declined(retry) {
			return "", err
		}
	}
}
