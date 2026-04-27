package vo

// APIKeyOwner Session 所属的 API Key 名称值对象（来自鉴权中间件注入的 ctx）
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type APIKeyOwner string

// String 返回字符串形态
func (o APIKeyOwner) String() string { return string(o) }

// IsEmpty 判断是否为空
func (o APIKeyOwner) IsEmpty() bool { return string(o) == "" }
