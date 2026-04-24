// Package vo llmproxy 域值对象
package vo

// EndpointAlias 模型端点别名值对象
//
// 表示客户端请求中暴露的模型名（与上游实际模型名区分），
// 是 Endpoint 聚合的业务标识。
//
//	@author centonhuang
//	@update 2026-04-22 16:30:00
type EndpointAlias string

// String 返回字符串形态
//
//	@receiver a EndpointAlias
//	@return string
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (a EndpointAlias) String() string { return string(a) }

// IsEmpty 判断别名是否为空
//
//	@receiver a EndpointAlias
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (a EndpointAlias) IsEmpty() bool { return string(a) == "" }
