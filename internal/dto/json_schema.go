// Package dto 请求/响应数据传输对象定义
package dto

import (
	"github.com/hcd233/aris-proxy-api/internal/domain/common/vo"
)

// JSONSchemaTypeValue 是 domain/common/vo.JSONSchemaTypeValue 的类型别名
//
// 该类型表示 JSON Schema 的 type 字段联合类型，支持 string 或 string[] 两种形态。
// 部分客户端（如 Codex Desktop）会在工具参数 schema 中传递 "type": ["string", "null"]。
//
// 注意：此类型别名仅用于 DTO 层内部兼容。domain 层已定义原始类型，
// 两者等价，可以相互赋值使用。
//
//	@author centonhuang
//	@update 2026-04-26 00:55:00
type JSONSchemaTypeValue = vo.JSONSchemaTypeValue

// JSONSchemaProperty 是 domain/common/vo.JSONSchemaProperty 的类型别名
//
// 该类型表示递归 JSON Schema 属性定义，覆盖标准 JSON Schema 字段。
//
// 注意：此类型别名仅用于 DTO 层内部兼容。domain 层已定义原始类型，
// 两者等价，可以相互赋值使用。
//
//	@author centonhuang
//	@update 2026-04-26 00:55:00
type JSONSchemaProperty = vo.JSONSchemaProperty
