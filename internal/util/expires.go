// Package util expires 解析工具
package util

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// ParseExpiresIn 解析分享过期选项为 TTL。
//
//	@param expiresIn string 过期选项: 1d | 7d | 30d | never | custom
//	@param customAt *int64 自定义过期时间戳（秒），expiresIn=custom 时使用
//	@return time.Duration
//	@return error
//	@author centonhuang
//	@update 2026-06-02 10:00:00
func ParseExpiresIn(expiresIn string, customAt *int64) (time.Duration, error) {
	switch expiresIn {
	case constant.ShareExpireOption1Day, "":
		return constant.ShareTTL1Day, nil
	case constant.ShareExpireOption1Week, constant.ShareExpireOption1WeekAlt:
		return constant.ShareTTL1Week, nil
	case constant.ShareExpireOption1Month, constant.ShareExpireOption1MonthAlt:
		return constant.ShareTTL1Month, nil
	case constant.ShareExpireOptionNever:
		return constant.ShareTTLNeverExpire, nil
	case constant.ShareExpireOptionCustom:
		if customAt == nil {
			return 0, ierr.New(ierr.ErrValidation, "expiresAt is required when expiresIn is custom")
		}
		t := time.Unix(*customAt, 0)
		remaining := time.Until(t)
		if remaining <= 0 {
			return 0, ierr.New(ierr.ErrValidation, "expiresAt must be in the future")
		}
		return remaining, nil
	default:
		return constant.ShareTTLDefault, nil
	}
}
