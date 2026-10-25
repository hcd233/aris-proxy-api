// Package config 管理 aris 客户端本地配置与路径。
package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Paths aris 客户端本地路径。
type Paths struct {
	Root string
}

// DefaultPaths 返回客户端默认根目录 ~/.aris。
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, ierr.Wrap(ierr.ErrInternal, err, "resolve user home directory")
	}
	return Paths{Root: filepath.Join(home, constant.ArisClientRootDirName)}, nil
}

// ConfigFile 返回当前配置文件路径 ~/.aris/config.json。
func (p Paths) ConfigFile() string {
	return filepath.Join(p.Root, constant.ArisClientConfigFileName)
}

// legacyConfigFile 返回旧版配置文件路径 ~/.aris/trace/config.json。
func (p Paths) legacyConfigFile() string {
	return filepath.Join(p.Root, constant.ArisClientLegacyConfigDirName, constant.ArisClientConfigFileName)
}

// Config 客户端配置。
type Config struct {
	Host   string `json:"host"`
	APIKey string `json:"apiKey"`
}

// Store 配置读写接口。
type Store interface {
	Load(ctx context.Context) (Config, error)
	Save(ctx context.Context, config Config) error
}

type store struct {
	paths Paths
}

// NewStore 构造配置存储。
func NewStore(paths Paths) Store {
	return &store{paths: paths}
}

// Load 读取配置；新路径不存在时回退读取旧版 trace 目录，避免升级后要求重新 init。
func (s *store) Load(ctx context.Context) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(s.paths.ConfigFile())
	if errors.Is(err, os.ErrNotExist) {
		data, err = os.ReadFile(s.paths.legacyConfigFile())
	}
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, ierr.Wrap(ierr.ErrInternal, err, "read aris client config")
	}
	var config Config
	if err := sonic.Unmarshal(data, &config); err != nil {
		return Config{}, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode aris client config")
	}
	return config, nil
}

// Save 写入配置；父目录与文件分别收紧到 0700/0600。
func (s *store) Save(ctx context.Context, config Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := sonic.Marshal(config)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode aris client config")
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
