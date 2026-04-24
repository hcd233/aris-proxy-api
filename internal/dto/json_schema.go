// Package dto 请求/响应数据传输对象定义
package dto

import (
	"github.com/hcd233/aris-proxy-api/internal/domain/common/vo"
)

// JSONSchemaTypeValue 重新导出至 domain/common/vo.JSONSchemaTypeValue
//
// 该类型已迁移到 internal/domain/common/vo 作为领域值对象，此处保留类型别名
// 避免破坏现有调用方 import。新代码应直接使用 vo.JSONSchemaTypeValue。
//
// Deprecated: 请使用 internal/domain/common/vo.JSONSchemaTypeValue
type JSONSchemaTypeValue = vo.JSONSchemaTypeValue

// JSONSchemaProperty 重新导出至 domain/common/vo.JSONSchemaProperty
//
// 该类型已迁移到 internal/domain/common/vo 作为领域值对象，此处保留类型别名
// 避免破坏现有调用方 import。新代码应直接使用 vo.JSONSchemaProperty。
//
// Deprecated: 请使用 internal/domain/common/vo.JSONSchemaProperty
type JSONSchemaProperty = vo.JSONSchemaProperty
