package tracecli

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type Config struct {
	Host   string `json:"host"`
	Agent  string `json:"agent"`
	APIKey string `json:"apiKey"`
}

type ConfigStore interface {
	Load(ctx context.Context) (Config, error)
	Save(ctx context.Context, config Config) error
}

type configStore struct {
	paths Paths
}

func NewConfigStore(paths Paths) ConfigStore {
	return &configStore{paths: paths}
}

func (s *configStore) Load(ctx context.Context) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(s.paths.ConfigFile())
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, ierr.Wrap(ierr.ErrInternal, err, "read trace client config")
	}
	var config Config
	if err := sonic.Unmarshal(data, &config); err != nil {
		return Config{}, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode trace client config")
	}
	return config, nil
}

func (s *configStore) Save(ctx context.Context, config Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := sonic.MarshalIndent(config, "", constant.TraceClientJSONIndent)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode trace client config")
	}
	return writePrivateFile(s.paths.ConfigFile(), data)
}

func writePrivateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "create private directory")
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // directory needs 0700 for execute
		return ierr.Wrap(ierr.ErrInternal, err, "secure private directory")
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "create private temporary file")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() //nolint:errcheck // best-effort cleanup

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return ierr.Wrap(ierr.ErrInternal, err, "secure private temporary file")
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return ierr.Wrap(ierr.ErrInternal, err, "write private temporary file")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return ierr.Wrap(ierr.ErrInternal, err, "sync private temporary file")
	}
	if err := tmp.Close(); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "close private temporary file")
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "replace private file")
	}
	return nil
}
