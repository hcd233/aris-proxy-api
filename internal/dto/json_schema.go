// Package dto 请求/响应数据传输对象定义
package dto

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/vo"
)

// JSONSchemaTypeValue 是 domain/common/vo.JSONSchemaTypeValue 的内嵌类型
//
// 该类型表示 JSON Schema 的 type 字段联合类型，支持 string 或 string[] 两种形态。
// 部分客户端（如 Codex Desktop）会在工具参数 schema 中传递 "type": ["string", "null"]。
//
//	@author centonhuang
//	@update 2026-04-26 00:55:00
type JSONSchemaTypeValue struct {
	vo.JSONSchemaTypeValue
}

// JSONSchemaProperty 是 domain/common/vo.JSONSchemaProperty 的内嵌类型
//
// 该类型表示递归 JSON Schema 属性定义，覆盖标准 JSON Schema 字段。
//
//	@author centonhuang
//	@update 2026-04-26 00:55:00
type JSONSchemaProperty struct {
	vo.JSONSchemaProperty
}

// Schema 实现 huma.SchemaProvider 接口
func (JSONSchemaProperty) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{Type: constant.JSONSchemaTypeObject}
}

// HasType 判断 JSON Schema 的 type 是否包含给定类型
func (p *JSONSchemaProperty) HasType(typeName string) bool {
	if p == nil {
		return false
	}
	return p.JSONSchemaProperty.HasType(typeName)
}
