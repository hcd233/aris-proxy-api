package tracecli

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type PendingRecord struct {
	SessionID      string                 `json:"session_id"`
	Model          string                 `json:"model,omitempty"`
	CWD            string                 `json:"cwd,omitempty"`
	SessionSource  string                 `json:"session_source,omitempty"`
	Source         string                 `json:"source"`
	RecordType     string                 `json:"record_type"`
	Event          string                 `json:"hook_event_name,omitempty"`
	TurnID         string                 `json:"turn_id,omitempty"`
	CallID         string                 `json:"call_id,omitempty"`
	TranscriptLine *int64                 `json:"transcript_line,omitempty"`
	ClientSequence int64                  `json:"client_sequence,omitempty"`
	DedupKey       string                 `json:"dedup_key"`
	Payload        sonic.NoCopyRawMessage `json:"payload"`
	CreatedAt      time.Time              `json:"created_at"`
}

type RecordResult struct {
	DedupKey string `json:"dedupKey"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

type Spool struct {
	paths     Paths
	hardLimit int64
}

type pendingFile struct {
	path    string
	name    string
	modTime time.Time
	size    int64
}

func NewSpool(paths Paths, hardLimit int64) *Spool {
	if hardLimit <= 0 {
		hardLimit = constant.TraceClientSpoolLimit
	}
	return &Spool{paths: paths, hardLimit: hardLimit}
}

func (s *Spool) Append(ctx context.Context, record PendingRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if record.SessionID == "" || record.DedupKey == "" || len(record.Payload) == 0 {
		return ierr.New(ierr.ErrValidation, "invalid pending trace record")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	data, err := sonic.Marshal(record)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode pending trace record")
	}
	if int64(len(data)) > constant.TraceClientBatchMaxBytes {
		return ierr.New(ierr.ErrQuotaExceeded, "pending trace record exceeds batch limit")
	}
	return withFileLock(s.lockFile(), func() error {
		path := s.recordPath(record.DedupKey)
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return ierr.Wrap(ierr.ErrInternal, err, "inspect pending trace record")
		}
		size, err := directorySize(s.paths.PendingDir())
		if err != nil {
			return err
		}
		if size+int64(len(data)) > s.hardLimit {
			return ierr.New(ierr.ErrQuotaExceeded, "trace spool hard limit reached")
		}
		return writePrivateFile(path, data)
	})
}

func (s *Spool) Batch(
	ctx context.Context,
	maxRecords int,
	maxBytes int64,
) ([]PendingRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if maxRecords <= 0 {
		maxRecords = constant.TraceClientBatchMaxRecords
	}
	if maxBytes <= 0 {
		maxBytes = constant.TraceClientBatchMaxBytes
	}
	batch := []PendingRecord{}
	err := withFileLock(s.lockFile(), func() error {
		files, err := pendingFiles(s.paths.PendingDir())
		if err != nil {
			return err
		}
		var sessionID string
		var totalBytes int64
		for _, file := range files {
			if len(batch) >= maxRecords {
				break
			}
			data, err := os.ReadFile(file.path)
			if err != nil {
				return ierr.Wrap(ierr.ErrInternal, err, "read pending trace record")
			}
			var record PendingRecord
			if err := sonic.Unmarshal(data, &record); err != nil {
				return ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode pending trace record")
			}
			if sessionID == "" {
				sessionID = record.SessionID
			}
			if record.SessionID != sessionID {
				continue
			}
			if totalBytes+file.size > maxBytes {
				break
			}
			batch = append(batch, record)
			totalBytes += file.size
		}
		return nil
	})
	return batch, err
}

func (s *Spool) Acknowledge(ctx context.Context, results []RecordResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return withFileLock(s.lockFile(), func() error {
		for _, result := range results {
			source := s.recordPath(result.DedupKey)
			switch result.Status {
			case constant.TraceRecordStatusAccepted, constant.TraceRecordStatusDuplicate:
				if err := os.Remove(source); err != nil && !errors.Is(err, os.ErrNotExist) {
					return ierr.Wrap(ierr.ErrInternal, err, "remove acknowledged trace record")
				}
			case constant.TraceRecordStatusRejected:
				if err := os.MkdirAll(s.paths.RejectedDir(), 0o700); err != nil {
					return ierr.Wrap(ierr.ErrInternal, err, "create rejected trace directory")
				}
				target := filepath.Join(s.paths.RejectedDir(), filepath.Base(source))
				if err := os.Rename(source, target); err != nil && !errors.Is(err, os.ErrNotExist) {
					return ierr.Wrap(ierr.ErrInternal, err, "move rejected trace record")
				}
			}
		}
		return nil
	})
}

func (s *Spool) lockFile() string {
	return filepath.Join(s.paths.StateDir(), constant.TraceClientSpoolLockFile)
}

func (s *Spool) recordPath(dedupKey string) string {
	digest := sha256.Sum256([]byte(dedupKey))
	name := hex.EncodeToString(digest[:]) + constant.TraceClientRecordFileSuffix
	return filepath.Join(s.paths.PendingDir(), name)
}

func directorySize(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrInternal, err, "list trace spool")
	}
	var total int64
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return 0, ierr.Wrap(ierr.ErrInternal, err, "inspect trace spool entry")
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	return total, nil
}

func pendingFiles(dir string) ([]pendingFile, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []pendingFile{}, nil
	}
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "list pending trace records")
	}
	files := make([]pendingFile, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, ierr.Wrap(ierr.ErrInternal, err, "inspect pending trace record")
		}
		if info.Mode().IsRegular() && filepath.Ext(entry.Name()) == constant.TraceClientRecordFileSuffix {
			files = append(files, pendingFile{
				path:    filepath.Join(dir, entry.Name()),
				name:    entry.Name(),
				modTime: info.ModTime(),
				size:    info.Size(),
			})
		}
	}
	slices.SortFunc(files, func(a, b pendingFile) int {
		if order := a.modTime.Compare(b.modTime); order != 0 {
			return order
		}
		return cmp.Compare(a.name, b.name)
	})
	return files, nil
}
