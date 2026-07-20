package tracecli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type hookEnvelope struct {
	HookEventName  string `json:"hook_event_name"`
	SessionID      string `json:"session_id"`
	Model          string `json:"model,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	Source         string `json:"source,omitempty"`
	TurnID         string `json:"turn_id,omitempty"`
	ToolUseID      string `json:"tool_use_id,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`
}

type ingestRecord struct {
	Source         string                 `json:"source"`
	RecordType     string                 `json:"record_type"`
	HookEventName  string                 `json:"hook_event_name,omitempty"`
	TurnID         string                 `json:"turn_id,omitempty"`
	CallID         string                 `json:"call_id,omitempty"`
	TranscriptLine *int64                 `json:"transcript_line,omitempty"`
	ClientSequence int64                  `json:"client_sequence,omitempty"`
	DedupKey       string                 `json:"dedup_key"`
	Payload        sonic.NoCopyRawMessage `json:"payload"`
}

type ingestBatch struct {
	SessionID string         `json:"session_id"`
	Model     string         `json:"model,omitempty"`
	CWD       string         `json:"cwd,omitempty"`
	Source    string         `json:"source,omitempty"`
	Records   []ingestRecord `json:"records"`
}

type ingestResultEnvelope struct {
	Results []RecordResult `json:"results"`
}

type Ingestor struct {
	paths   Paths
	config  ConfigStore
	spool   *Spool
	rollout *RolloutReader
	client  *http.Client
}

type IngestCommandOptions struct {
	Paths      Paths
	In         io.Reader
	Out        io.Writer
	HTTPClient *http.Client
}

func NewIngestor(paths Paths, client *http.Client) *Ingestor {
	if client == nil {
		client = &http.Client{Timeout: constant.TraceClientHTTPTimeout}
	} else if client.Timeout == 0 {
		clone := *client
		clone.Timeout = constant.TraceClientHTTPTimeout
		client = &clone
	}
	spool := NewSpool(paths, constant.TraceClientSpoolLimit)
	return &Ingestor{
		paths:   paths,
		config:  NewConfigStore(paths),
		spool:   spool,
		rollout: NewRolloutReader(paths, spool),
		client:  client,
	}
}

func (i *Ingestor) Ingest(ctx context.Context, raw []byte) error {
	var hook hookEnvelope
	if err := sonic.Unmarshal(raw, &hook); err != nil {
		return ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode hook input")
	}
	if hook.SessionID == "" || hook.HookEventName == "" {
		return ierr.New(ierr.ErrValidation, "hook input missing identity")
	}
	spoolID, sequence, err := nextSequence(ctx, i.paths)
	if err != nil {
		return err
	}
	record := PendingRecord{
		SessionID:      hook.SessionID,
		Model:          hook.Model,
		CWD:            hook.CWD,
		SessionSource:  hook.Source,
		Source:         constant.TraceRecordSourceHook,
		RecordType:     constant.TraceRecordTypeHookEvent,
		Event:          hook.HookEventName,
		TurnID:         hook.TurnID,
		CallID:         hook.ToolUseID,
		ClientSequence: sequence,
		DedupKey:       fmt.Sprintf(constant.TraceClientHookDedupFormat, spoolID, sequence),
		Payload:        append(sonic.NoCopyRawMessage{}, raw...),
	}
	if err := i.spool.Append(ctx, record); err != nil {
		return err
	}
	if hook.TranscriptPath != "" {
		if _, err := i.rollout.ReadNew(ctx, hook.SessionID, hook.TranscriptPath); err != nil {
			writeLocalError(i.paths, constant.TraceClientLogCategoryRollout)
		}
	}
	config, err := i.config.Load(ctx)
	if err != nil {
		return err
	}
	if config.Host == "" || config.APIKey == "" {
		return ierr.New(ierr.ErrValidation, "trace client is not initialized")
	}
	return i.flush(ctx, config)
}

func (i *Ingestor) flush(ctx context.Context, config Config) error {
	batch, err := i.spool.Batch(
		ctx,
		constant.TraceClientBatchMaxRecords,
		constant.TraceClientBatchMaxBytes,
	)
	if err != nil || len(batch) == 0 {
		return err
	}
	request := ingestBatch{
		SessionID: batch[0].SessionID,
		Records:   make([]ingestRecord, 0, len(batch)),
	}
	for _, record := range batch {
		if request.Model == "" && record.Model != "" {
			request.Model = record.Model
		}
		if request.CWD == "" && record.CWD != "" {
			request.CWD = record.CWD
		}
		if request.Source == "" && record.SessionSource != "" {
			request.Source = record.SessionSource
		}
		request.Records = append(request.Records, ingestRecord{
			Source:         record.Source,
			RecordType:     record.RecordType,
			HookEventName:  record.Event,
			TurnID:         record.TurnID,
			CallID:         record.CallID,
			TranscriptLine: record.TranscriptLine,
			ClientSequence: record.ClientSequence,
			DedupKey:       record.DedupKey,
			Payload:        record.Payload,
		})
	}
	body, err := sonic.Marshal(request)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode trace ingest request")
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		config.Host+constant.TraceClientIngestPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return ierr.Wrap(ierr.ErrBadRequest, err, "create trace ingest request")
	}
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+config.APIKey)
	resp, err := i.client.Do(req)
	if err != nil {
		return ierr.Wrap(ierr.ErrProxySend, err, "send trace ingest request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ierr.New(ierr.ErrBadRequest, "trace ingest request rejected")
	}
	var response ingestResultEnvelope
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode trace ingest response")
	}
	if len(response.Results) == 0 {
		return ierr.New(ierr.ErrBadRequest, "trace ingest response has no results")
	}
	return i.spool.Acknowledge(ctx, response.Results)
}

func RunIngestCommand(ctx context.Context, opts IngestCommandOptions) error {
	paths := opts.Paths
	if paths.Root == "" {
		resolved, err := DefaultPaths()
		if err != nil {
			return nil //nolint:nilerr // fail-open: never block Codex
		}
		paths = resolved
	}
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	raw, err := io.ReadAll(io.LimitReader(in, constant.TraceClientHookInputLimit+1))
	if err != nil || len(raw) > constant.TraceClientHookInputLimit {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
		return nil //nolint:nilerr // fail-open: never block Codex
	}
	var hook hookEnvelope
	if sonic.Unmarshal(raw, &hook) == nil && hook.HookEventName == constant.TraceEventStop {
		_, _ = io.WriteString(out, constant.EmptyJSONObject) //nolint:errcheck // best-effort stdout
	}
	if err := NewIngestor(paths, opts.HTTPClient).Ingest(ctx, raw); err != nil {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
	} //nolint:nilerr // fail-open: never block Codex
	return nil
}

func writeLocalError(paths Paths, category string) {
	if err := os.MkdirAll(paths.LogDir(), 0o700); err != nil {
		return
	}
	_ = os.Chmod(paths.LogDir(), 0o700) //nolint:errcheck,gosec // directory needs 0700
	now := time.Now().UTC()
	name := constant.TraceClientLogPrefix + now.Format(constant.TraceClientLogDateFormat) +
		constant.TraceClientLogSuffix
	file, err := os.OpenFile(
		filepath.Join(paths.LogDir(), name),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()                                                             //nolint:errcheck // best-effort close
	_ = file.Chmod(0o600)                                                                           //nolint:errcheck // best-effort permission
	_, _ = fmt.Fprintf(file, constant.TraceClientLogLineFormat, now.Format(time.RFC3339), category) //nolint:errcheck // best-effort write
	cleanupOldFiles(paths.LogDir(), now.Add(-constant.TraceClientRejectedRetention))
}

func cleanupOldFiles(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, entry.Name())) //nolint:errcheck // best-effort cleanup
		}
	}
}
