package model

import (
	"path/filepath"

	"strconv"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// filepathJoin 用 "/" 分隔的相对路径拼接到 home（跨平台）
func filepathJoin(home, rel string) string {
	return filepath.Join(home, filepath.FromSlash(rel))
}

// itoa 整数转十进制字符串
func itoa(n int) string {
	return strconv.Itoa(n)
}

// ierrWrapRead 包装配置文件读取错误
func ierrWrapRead(_ string, err error) error {
	return ierr.Wrap(ierr.ErrInternal, err, "read config file")
}

// splitLines 按行拆分（容忍 \r\n）
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, constant.ClientModelCRLF, constant.ClientModelLF)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
