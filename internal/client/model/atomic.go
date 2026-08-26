// Package model 实现 aris model export：将服务端模型配置写入本地 agent harness。
package model

import (
	"os"
	"path/filepath"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// TargetModel 写入目标的统一模型描述
type TargetModel struct {
	Alias           string
	UpstreamModel   string
	ContextLength   int
	MaxOutputTokens int
	Capabilities    []string
}

// Target 单个 agent harness 配置写入器
type Target interface {
	// Key 机器标识：opencode|pi|codex|claude-code
	Key() string
	// Label 展示名
	Label() string
	// ConfigPath 默认配置路径（home 注入便于测试）
	ConfigPath(home string) string
	// Write 将模型配置写入 path；已存在时先备份 .bak 再原子替换
	Write(path string, host, apiKey string, models []TargetModel) error
}

// Targets 返回全部支持的写入器，顺序即交互展示顺序
func Targets() []Target {
	return []Target{
		OpenCodeTarget{},
		PiTarget{},
	}
}

// backupAndWrite 备份已有文件为 .bak 后原子写入 data（0600 私有权限）
func backupAndWrite(path string, data []byte) error {
	if original, err := os.ReadFile(path); err == nil { //nolint:gosec // path 由调用方传入固定配置路径
		if bakErr := os.WriteFile(path+".bak", original, 0o600); bakErr != nil { //nolint:gosec // path is a fixed config path from caller
			return ierr.Wrap(ierr.ErrInternal, bakErr, "backup config file")
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "create config directory")
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "create temporary file")
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() //nolint:errcheck // best-effort cleanup

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return ierr.Wrap(ierr.ErrInternal, err, "secure temporary file")
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() //nolint:errcheck // already in error path
		return ierr.Wrap(ierr.ErrInternal, err, "write temporary file")
	}
	if err := tmp.Close(); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "close temporary file")
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "replace config file")
	}
	return nil
}

// readJSONFile 读取 JSON 文件到 dst；文件不存在时不报错（dst 保持零值）
func readJSONFile(path string, dst any) error {
	data, err := os.ReadFile(path) //nolint:gosec // path 由调用方传入固定配置路径
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return ierr.Wrap(ierr.ErrInternal, err, "read config file")
	}
	if len(data) == 0 {
		return nil
	}
	if err := sonicUnmarshal(data, dst); err != nil {
		return err
	}
	return nil
}
