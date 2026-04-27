// Package vo Identity 域值对象
package vo

import "strings"

// UserName 用户名值对象
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type UserName string

// String 返回字符串形态
func (n UserName) String() string { return string(n) }

// IsEmpty 判断是否为空
func (n UserName) IsEmpty() bool { return strings.TrimSpace(string(n)) == "" }

// Email 邮箱值对象（不做严格校验，遵循原有宽松策略）
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type Email string

// String 返回字符串形态
func (e Email) String() string { return string(e) }

// Avatar 头像 URL 值对象
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type Avatar string

// String 返回字符串形态
func (a Avatar) String() string { return string(a) }
