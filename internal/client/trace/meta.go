package trace

import (
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// sessionMeta codex hook 触发时记录的会话级元数据，flush 时随批次上报。
type sessionMeta struct {
	Model string `json:"model,omitempty"`
	CWD   string `json:"cwd,omitempty"`
}

func sessionMetaPath(paths Paths, sessionID string) string {
	return filepath.Join(paths.StateDir(), constant.ArisClientSessionMetaDir, sessionID+constant.ArisClientSessionMetaSuffix)
}

// writeSessionMeta 覆盖写入 per-session 元数据（codex SessionStart/Stop hook 触发时调用）。
func writeSessionMeta(paths Paths, sessionID string, meta sessionMeta) error {
	data, err := sonic.Marshal(meta)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode session meta")
	}
	path := sessionMetaPath(paths, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "mkdir session meta dir")
	}
	return withFileLock(path+constant.ArisClientSessionMetaLockSuffix, func() error {
		return writePrivateFile(path, data)
	})
}

// loadSessionMeta 读取 per-session 元数据；不存在或损坏时返回零值（元数据缺失不阻塞上报）。
func loadSessionMeta(paths Paths, sessionID string) sessionMeta {
	var meta sessionMeta
	data, err := os.ReadFile(sessionMetaPath(paths, sessionID))
	if err != nil {
		return meta
	}
	_ = sonic.Unmarshal(data, &meta) //nolint:errcheck // best-effort read
	return meta
}
