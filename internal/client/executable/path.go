// Package executable 解析当前 aris 可执行文件的真实路径。
//
// 从 trace（hook 安装）与 setup（配置向导）各自的重复实现中抽出，供 hook 安装、
// 状态检查与 `aris update` 自替换共用：自更新不应反向依赖 hook 安装包。
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
