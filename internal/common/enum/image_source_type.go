// Package enum provides common enums for the application.
package enum

// ImageSourceType Anthropic 图片来源类型
//
//	@author centonhuang
//	@update 2026-04-09 15:00:00
type ImageSourceType = string

const (
	// ImageSourceTypeBase64 base64 编码图片
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	ImageSourceTypeBase64 ImageSourceType = "base64"

	// ImageSourceTypeURL URL 引用图片
	//
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	ImageSourceTypeURL ImageSourceType = "url"
)

// JSONSchemaObjectType JSON Schema object 类型字面量
//
//	@author centonhuang
//	@update 2026-04-09 15:00:00
const JSONSchemaObjectType = "object"
