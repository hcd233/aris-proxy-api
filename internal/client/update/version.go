// Package update 实现 aris 客户端的使用中更新检查与 aris update 自更新。
package update

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// parseVersion 解析 vX.Y.Z / X.Y.Z 版本号；缺省段补 0，含非数字段（如 dev、rc）即不可解析
func parseVersion(version string) ([3]int, bool) {
	var parts [3]int
	trimmed := strings.TrimPrefix(version, constant.ArisClientVersionPrefixV)
	if trimmed == "" {
		return parts, false
	}
	fields := strings.Split(trimmed, ".")
	if len(fields) > len(parts) {
		return parts, false
	}
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 0 {
			return parts, false
		}
		parts[index] = value
	}
	return parts, true
}

// IsNewer 判断 latest 是否比 current 更新；任一版本不可解析时返回 false（避免误判降级与预发布）
func IsNewer(latest, current string) bool {
	latestParts, latestOK := parseVersion(latest)
	currentParts, currentOK := parseVersion(current)
	if !latestOK || !currentOK {
		return false
	}
	return slices.Compare(latestParts[:], currentParts[:]) > 0
}

// IsUpToDate 判断 current 是否已不低于 latest；任一版本不可解析时返回 false
func IsUpToDate(latest, current string) bool {
	latestParts, latestOK := parseVersion(latest)
	currentParts, currentOK := parseVersion(current)
	if !latestOK || !currentOK {
		return false
	}
	return slices.Compare(currentParts[:], latestParts[:]) >= 0
}

// ShouldCheck 判断当前构建与终端环境是否参与使用中更新检查
func ShouldCheck(current string, interactive bool) bool {
	if !interactive || current == "" || current == constant.ArisClientDevVersion {
		return false
	}
	if _, ok := parseVersion(current); !ok {
		return false
	}
	return !envTrue(constant.ArisClientNoUpdateCheckEnv)
}

// envTrue 判断布尔语义环境变量是否为真值（大小写不敏感）
func envTrue(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return false
	}
	values := strings.Split(constant.ArisClientEnvValueTrueList, constant.ArisClientEnvValueSeparator)
	return slices.Contains(values, value)
}
