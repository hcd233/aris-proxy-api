package tracecli

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

type rolloutEnvelope struct {
	Type    string                 `json:"type"`
	Payload sonic.NoCopyRawMessage `json:"payload"`
}

type rolloutPayload struct {
	Type   string `json:"type"`
	TurnID string `json:"turn_id,omitempty"`
	CallID string `json:"call_id,omitempty"`
}

type RolloutReader struct {
	paths Paths
	spool *Spool
}

func NewRolloutReader(paths Paths, spool *Spool) *RolloutReader {
	return &RolloutReader{paths: paths, spool: spool}
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
		appended, err := r.readIncremental(ctx, sessionID, transcriptPath, statePath)
		records = append(records, appended...)
		return err
	})
	return records, err
}

func (r *RolloutReader) readIncremental(
	ctx context.Context,
	sessionID, transcriptPath, statePath string,
) ([]PendingRecord, error) {
	state, err := loadTranscriptState(statePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDataNotExists, err, "inspect Codex rollout")
	}
	identity := fileIdentity(info)
	if state.Identity != identity || info.Size() < state.Offset {
		state = transcriptState{Identity: identity}
	}
	file, err := os.Open(transcriptPath)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDataNotExists, err, "open Codex rollout")
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // best-effort close
	if _, err := file.Seek(state.Offset, io.SeekStart); err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "seek Codex rollout")
	}
	records, newState, err := r.parseRolloutLines(ctx, sessionID, file, state)
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
) ([]PendingRecord, transcriptState, error) {
	records := []PendingRecord{}
	bufReader := bufio.NewReader(reader)
	for {
		line, readErr := bufReader.ReadBytes('\n')
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, state, ierr.Wrap(ierr.ErrInternal, readErr, "read Codex rollout")
		}
		state.Offset += int64(len(line))
		state.Line++
		raw := bytes.TrimSuffix(line, []byte{'\n'})
		raw = bytes.TrimSuffix(raw, []byte{'\r'})
		if !sonic.Valid(raw) {
			continue
		}
		record, err := rolloutRecord(sessionID, state.Line, raw)
		if err != nil {
			return nil, state, err
		}
		record.CreatedAt = time.Now().UTC()
		if err := r.spool.Append(ctx, record); err != nil {
			return nil, state, err
		}
		records = append(records, record)
	}
	return records, state, nil
}

func rolloutRecord(sessionID string, line int64, raw []byte) (PendingRecord, error) {
	var envelope rolloutEnvelope
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return PendingRecord{}, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode rollout envelope")
	}
	var payload rolloutPayload
	if len(envelope.Payload) > 0 {
		_ = sonic.Unmarshal(envelope.Payload, &payload) //nolint:errcheck // best-effort field extraction
	}
	digest := sha256.Sum256(raw)
	lineCopy := line
	return PendingRecord{
		SessionID:      sessionID,
		Source:         constant.TraceRecordSourceRollout,
		RecordType:     rolloutRecordType(envelope.Type),
		Event:          payload.Type,
		TurnID:         payload.TurnID,
		CallID:         payload.CallID,
		TranscriptLine: &lineCopy,
		DedupKey: fmt.Sprintf(
			constant.TraceClientRolloutDedupFormat,
			sessionID,
			line,
			hex.EncodeToString(digest[:]),
		),
		Payload: append(sonic.NoCopyRawMessage{}, raw...),
	}, nil
}

func rolloutRecordType(recordType string) string {
	switch recordType {
	case constant.TraceRolloutTypeSessionMeta,
		constant.TraceRolloutTypeTurnContext,
		constant.TraceRolloutTypeResponseItem,
		constant.TraceRolloutTypeEventMsg:
		return recordType
	default:
		return constant.TraceRolloutTypeUnknown
	}
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
