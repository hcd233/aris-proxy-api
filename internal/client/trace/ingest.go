package trace

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
	SessionID       string         `json:"session_id"`
	ParentSessionID string         `json:"parent_session_id,omitempty"`
	Agent           string         `json:"agent,omitempty"`
	AgentID         string         `json:"agent_id,omitempty"`
	AgentType       string         `json:"agent_type,omitempty"`
	Model           string         `json:"model,omitempty"`
	CWD             string         `json:"cwd,omitempty"`
	Records         []ingestRecord `json:"records"`
}

// IngestBatchJSON 导出视图，供外部测试断言上报请求体。
type IngestBatchJSON struct {
	SessionID       string `json:"session_id"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
	Agent           string `json:"agent,omitempty"`
	AgentID         string `json:"agent_id,omitempty"`
	AgentType       string `json:"agent_type,omitempty"`
	Records         []struct {
		Source     string `json:"source"`
		RecordType string `json:"record_type"`
		Event      string `json:"event"`
		SessionID  string `json:"session_id"`
	} `json:"records"`
}

type ingestResultEnvelope struct {
	Results []RecordResult `json:"results"`
}

type Ingestor struct {
	adapter AgentAdapter
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
	AgentName  string
}

func NewIngestor(paths Paths, client *http.Client, adapter AgentAdapter) *Ingestor {
	if client == nil {
		client = &http.Client{Timeout: constant.TraceClientHTTPTimeout}
	} else if client.Timeout == 0 {
		clone := *client
		clone.Timeout = constant.TraceClientHTTPTimeout
		client = &clone
	}
	spool := NewSpool(paths, constant.TraceClientSpoolLimit)
	return &Ingestor{
		adapter: adapter,
		paths:   paths,
		config:  NewConfigStore(paths),
		spool:   spool,
		rollout: NewRolloutReader(paths, spool, adapter),
		client:  client,
	}
}

func (i *Ingestor) Ingest(ctx context.Context, raw []byte) error {
	info, err := i.adapter.ParseHook(raw)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode hook input")
	}
	if info.SessionID == "" || info.EventName == "" {
		return ierr.New(ierr.ErrValidation, "hook input missing identity")
	}
	if info.EventName == constant.TraceEventSubagentStop && i.adapter.Name() == constant.TraceAgentCodex {
		// codex 子代理是独立 session（独立 transcript），SubagentStop 只读子代理 transcript；
		// claude 的 SubagentStop 无子代理 transcript 概念，保持原 hook 记录路径。
		return i.ingestSubagentStop(ctx, info)
	}
	if i.adapter.Name() == constant.TraceAgentCodex {
		// codex hook 纯触发：不生成 hook 记录，仅写会话元数据 + 触发 transcript 增量读取
		return i.ingestCodexHookTrigger(ctx, info)
	}
	// ── claude（保持现状：生成 hook 记录）──
	spoolID, sequence, err := nextSequence(ctx, i.paths)
	if err != nil {
		return err
	}
	payload := raw
	if info.EventName == constant.TraceEventStop {
		payload = TrimStopHookPayload(raw)
	}
	record := PendingRecord{
		SessionID:      info.SessionID,
		Agent:          i.adapter.Name(),
		Model:          info.Model,
		CWD:            info.CWD,
		Source:         constant.TraceRecordSourceHook,
		RecordType:     constant.TraceRecordTypeHookEvent,
		Event:          info.EventName,
		TurnID:         info.TurnID,
		CallID:         info.CallID,
		ClientSequence: sequence,
		DedupKey:       fmt.Sprintf(constant.TraceClientHookDedupFormat, spoolID, sequence),
		Payload:        append(sonic.NoCopyRawMessage{}, payload...),
	}
	if err := i.spool.Append(ctx, record); err != nil {
		return err
	}
	if info.TranscriptPath != "" {
		if _, err := i.rollout.ReadNew(ctx, info.SessionID, info.TranscriptPath); err != nil {
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

// ingestCodexHookTrigger codex hook 纯触发：写 per-session 元数据并读取 transcript 增量，
// 不生成任何 hook 记录。
func (i *Ingestor) ingestCodexHookTrigger(ctx context.Context, info HookInfo) error {
	if err := writeSessionMeta(i.paths, info.SessionID, sessionMeta{
		Model: info.Model,
		CWD:   info.CWD,
	}); err != nil {
		return err
	}
	if info.TranscriptPath != "" {
		if _, err := i.rollout.ReadNew(ctx, info.SessionID, info.TranscriptPath); err != nil {
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
	meta := loadSessionMeta(i.paths, batch[0].SessionID)
	request := ingestBatch{
		SessionID:       batch[0].SessionID,
		ParentSessionID: batch[0].ParentSessionID,
		Agent:           batch[0].Agent,
		AgentID:         batch[0].AgentID,
		AgentType:       batch[0].AgentType,
		Model:           meta.Model,
		CWD:             meta.CWD,
		Records:         make([]ingestRecord, 0, len(batch)),
	}
	for _, record := range batch {
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
	return i.postBatch(ctx, config, body)
}

// ingestSubagentStop 处理 SubagentStop hook：不生成 hook 记录，读取子代理
// transcript 增量（SessionID=子代理 id），每条记录携带父 session_id。
func (i *Ingestor) ingestSubagentStop(ctx context.Context, info HookInfo) error {
	if info.AgentTranscriptPath == "" {
		return nil // 无子代理 transcript，无数据可上报
	}
	childID := SubagentSessionIDFromPath(info.AgentTranscriptPath)
	if childID == "" {
		return nil
	}
	if _, err := i.rollout.ReadNewForSubagent(
		ctx, childID, info.AgentTranscriptPath, info.SessionID, info.AgentID, info.AgentType,
	); err != nil {
		writeLocalError(i.paths, constant.TraceClientLogCategoryRollout)
		return nil //nolint:nilerr // fail-open: never block agent CLI
	}
	config, err := i.config.Load(ctx)
	if err != nil {
		return err
	}
	if config.Host == "" || config.APIKey == "" {
		return ierr.New(ierr.ErrValidation, "trace client is not initialized")
	}
	return i.flushSubagent(ctx, config, childID, info)
}

// flushSubagent 上报子代理批次：从 spool 精确取 childID 对应记录，batch 携带父 session 与 agent 元数据。
// 若 spool 中最旧记录属于其他会话（父会话积压），本次不 POST，子代理记录留待后续批次。
func (i *Ingestor) flushSubagent(ctx context.Context, config Config, childID string, info HookInfo) error {
	batch, err := i.spool.BatchForSession(
		ctx,
		childID,
		constant.TraceClientBatchMaxRecords,
		constant.TraceClientBatchMaxBytes,
	)
	if err != nil || len(batch) == 0 {
		return err
	}
	meta := loadSessionMeta(i.paths, childID)
	request := ingestBatch{
		SessionID:       childID,
		ParentSessionID: info.SessionID,
		Agent:           i.adapter.Name(),
		AgentID:         info.AgentID,
		AgentType:       info.AgentType,
		Model:           meta.Model,
		CWD:             meta.CWD,
		Records:         make([]ingestRecord, 0, len(batch)),
	}
	for _, record := range batch {
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
	if len(request.Records) == 0 {
		return nil // 无子代理记录可发，避免空批次 400
	}
	body, err := sonic.Marshal(request)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode trace ingest request")
	}
	return i.postBatch(ctx, config, body)
}

// postBatch 发送 ingest 请求并确认 spool（flush 与 flushSubagent 的公共尾部）。
func (i *Ingestor) postBatch(ctx context.Context, config Config, body []byte) error {
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
			return nil //nolint:nilerr // fail-open: never block agent CLI
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
	adapter, err := LookupAdapter(opts.AgentName)
	if err != nil {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
		return nil //nolint:nilerr // fail-open: never block agent CLI
	}
	raw, err := io.ReadAll(io.LimitReader(in, constant.TraceClientHookInputLimit+1))
	if err != nil || len(raw) > constant.TraceClientHookInputLimit {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
		return nil //nolint:nilerr // fail-open: never block agent CLI
	}
	if info, parseErr := adapter.ParseHook(raw); parseErr == nil {
		if ack := adapter.StdoutAck(info); ack != "" {
			_, _ = io.WriteString(out, ack) //nolint:errcheck // best-effort stdout
		}
	}
	if err := NewIngestor(paths, opts.HTTPClient, adapter).Ingest(ctx, raw); err != nil {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
	} //nolint:nilerr // fail-open: never block agent CLI
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
