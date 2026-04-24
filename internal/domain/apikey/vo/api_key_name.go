// Package vo APIKey 域值对象
package vo

import "strings"

// APIKeyName API Key 的用户可读名称
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type APIKeyName string

// String 返回字符串形态
func (n APIKeyName) String() string { return string(n) }

// IsEmpty 判断名称是否为空
func (n APIKeyName) IsEmpty() bool { return strings.TrimSpace(string(n)) == "" }
