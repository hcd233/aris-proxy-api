// Package executable 解析当前 aris 可执行文件的真实路径。
//
// 供 `aris update` 自替换使用；独立成包，避免自更新反向依赖其他客户端包。
package executable

import (
	"os"
	"path/filepath"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Path 返回当前可执行文件的绝对路径（解析符号链接）
func Path() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "resolve executable path")
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "resolve executable symlinks")
	}
	return resolved, nil
}
