package trace

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type transcriptState struct {
	Identity string `json:"identity"`
	Offset   int64  `json:"offset"`
	Line     int64  `json:"line"`
}

// RolloutReader 是通用 transcript 增量读取器（codex rollout / claude session 均适用）：
// 按 transcript 文件路径哈希分派 offset 状态，逐行读取新内容并交给 adapter 分类。
type RolloutReader struct {
	paths   Paths
	spool   *Spool
	adapter AgentAdapter
}

func NewRolloutReader(paths Paths, spool *Spool, adapter AgentAdapter) *RolloutReader {
	return &RolloutReader{paths: paths, spool: spool, adapter: adapter}
}

func (r *RolloutReader) ReadNew(
	ctx context.Context,
	sessionID string,
	transcriptPath string,
) ([]PendingRecord, error) {
	if transcriptPath == "" {
		return []PendingRecord{}, nil
	}
	statePath, lockPath := r.transcriptPaths(transcriptPath)
	records := []PendingRecord{}
	err := withFileLock(lockPath, func() error {
		appended, err := r.readIncremental(ctx, sessionID, transcriptPath, statePath, PendingRecord{})
		records = append(records, appended...)
		return err
	})
	return records, err
}

// ReadNewForSubagent 与 ReadNew 相同，但生成的每条记录携带父会话与 agent 元数据
// （SubagentStop 上报子代理 transcript 时使用，保证任意路径发出的批次不丢失父关联）。
func (r *RolloutReader) ReadNewForSubagent(
	ctx context.Context,
	sessionID string,
	transcriptPath string,
	parentSessionID, agentID, agentType string,
) ([]PendingRecord, error) {
	if transcriptPath == "" {
		return []PendingRecord{}, nil
	}
	meta := PendingRecord{
		ParentSessionID: parentSessionID,
		AgentID:         agentID,
		AgentType:       agentType,
	}
	statePath, lockPath := r.transcriptPaths(transcriptPath)
	records := []PendingRecord{}
	err := withFileLock(lockPath, func() error {
		appended, err := r.readIncremental(ctx, sessionID, transcriptPath, statePath, meta)
		records = append(records, appended...)
		return err
	})
	return records, err
}

func (r *RolloutReader) readIncremental(
	ctx context.Context,
	sessionID, transcriptPath, statePath string,
	meta PendingRecord,
) ([]PendingRecord, error) {
	state, err := loadTranscriptState(statePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDataNotExists, err, "inspect transcript")
	}
	identity := fileIdentity(info)
	if state.Identity != identity || info.Size() < state.Offset {
		state = transcriptState{Identity: identity}
	}
	file, err := os.Open(transcriptPath)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDataNotExists, err, "open transcript")
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // best-effort close
	if _, err := file.Seek(state.Offset, io.SeekStart); err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "seek transcript")
	}
	records, newState, err := r.parseRolloutLines(ctx, sessionID, file, state, meta)
	if err != nil {
		return nil, err
	}
	data, err := sonic.Marshal(newState)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOMarshal, err, "encode rollout state")
	}
	return records, writePrivateFile(statePath, data)
}

func (r *RolloutReader) parseRolloutLines(
	ctx context.Context,
	sessionID string,
	reader io.Reader,
	state transcriptState,
	meta PendingRecord,
) ([]PendingRecord, transcriptState, error) {
	records := []PendingRecord{}
	bufReader := bufio.NewReader(reader)
	for {
		line, readErr := bufReader.ReadBytes('\n')
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, state, ierr.Wrap(ierr.ErrInternal, readErr, "read transcript")
		}
		state.Offset += int64(len(line))
		state.Line++
		raw := bytes.TrimSuffix(line, []byte{'\n'})
		raw = bytes.TrimSuffix(raw, []byte{'\r'})
		if !sonic.Valid(raw) {
			continue
		}
		classified := r.adapter.ClassifyTranscriptLine(raw)
		if r.adapter.IgnoreTranscriptLine(classified) {
			continue
		}
		record := r.rolloutRecord(sessionID, state.Line, raw, meta, classified)
		record.CreatedAt = time.Now().UTC()
		if err := r.spool.Append(ctx, record); err != nil {
			return nil, state, err
		}
		records = append(records, record)
	}
	return records, state, nil
}

func (r *RolloutReader) rolloutRecord(
	sessionID string,
	line int64,
	raw []byte,
	meta PendingRecord,
	classified TranscriptMeta,
) PendingRecord {
	lineCopy := line
	record := PendingRecord{
		SessionID:       sessionID,
		Agent:           r.adapter.Name(),
		Source:          constant.TraceRecordSourceRollout,
		RecordType:      classified.RecordType,
		Event:           classified.Event,
		TurnID:          classified.TurnID,
		CallID:          classified.CallID,
		TranscriptLine:  &lineCopy,
		DedupKey:        RolloutDedupKey(sessionID, classified, line, raw),
		Payload:         append(sonic.NoCopyRawMessage{}, raw...),
		ParentSessionID: meta.ParentSessionID,
		AgentID:         meta.AgentID,
		AgentType:       meta.AgentType,
	}
	return record
}

// RolloutDedupKey 生成 rollout 记录 dedup key：session_meta 用稳定语义键
// （payload.id，压缩重写后行号变化不产生重复），其余记录保持 line:hash。
func RolloutDedupKey(sessionID string, meta TranscriptMeta, line int64, raw []byte) string {
	if meta.RecordType == constant.TraceRolloutTypeSessionMeta && meta.SessionID != "" {
		return fmt.Sprintf(constant.TraceClientSessionMetaDedupFormat, sessionID, meta.SessionID)
	}
	if meta.RecordType == constant.TraceRolloutTypeEventMsg && meta.Event == constant.TraceEventTokenCount {
		// token_count 固定 key：同一会话多条 token_count 共用同一 key，服务端
		// ON CONFLICT DO UPDATE 后库里只保留最后一条（会话累计 token 汇总）。
		return fmt.Sprintf(constant.TraceClientTokenCountDedupFormat, sessionID)
	}
	digest := sha256.Sum256(raw)
	return fmt.Sprintf(constant.TraceClientRolloutDedupFormat, sessionID, line, hex.EncodeToString(digest[:]))
}

func (r *RolloutReader) transcriptPaths(transcriptPath string) (statePath, lockPath string) {
	digest := sha256.Sum256([]byte(transcriptPath))
	name := hex.EncodeToString(digest[:])
	dir := filepath.Join(r.paths.StateDir(), constant.TraceClientTranscriptStateDir)
	return filepath.Join(dir, name+constant.TraceClientRecordFileSuffix),
		filepath.Join(dir, name+constant.TraceClientTranscriptLockSuffix)
}

func loadTranscriptState(path string) (transcriptState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return transcriptState{}, nil
	}
	if err != nil {
		return transcriptState{}, ierr.Wrap(ierr.ErrInternal, err, "read rollout state")
	}
	var state transcriptState
	if err := sonic.Unmarshal(data, &state); err != nil {
		return transcriptState{}, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode rollout state")
	}
	return state, nil
}
