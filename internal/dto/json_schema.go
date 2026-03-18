package dto

// JSONSchemaProperty 递归 JSON Schema 属性定义，覆盖标准 JSON Schema 字段
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type JSONSchemaProperty struct {
	Type                 string                         `json:"type,omitempty" doc:"数据类型(string/number/integer/boolean/array/object/null)"`
	Description          string                         `json:"description,omitempty" doc:"属性描述"`
	Properties           map[string]*JSONSchemaProperty `json:"properties,omitempty" doc:"对象属性定义(递归)"`
	Items                *JSONSchemaProperty            `json:"items,omitempty" doc:"数组元素定义(递归)"`
	Required             []string                       `json:"required,omitempty" doc:"必填字段列表"`
	Enum                 []any                          `json:"enum,omitempty" doc:"枚举值列表"`
	Const                any                            `json:"const,omitempty" doc:"常量值"`
	Default              any                            `json:"default,omitempty" doc:"默认值"`
	AnyOf                []*JSONSchemaProperty          `json:"anyOf,omitempty" doc:"任意匹配"`
	OneOf                []*JSONSchemaProperty          `json:"oneOf,omitempty" doc:"唯一匹配"`
	AllOf                []*JSONSchemaProperty          `json:"allOf,omitempty" doc:"全部匹配"`
	Not                  *JSONSchemaProperty            `json:"not,omitempty" doc:"取反"`
	AdditionalProperties any                            `json:"additionalProperties,omitempty" doc:"额外属性(bool或JSONSchemaProperty)"`
	Strict               *bool                          `json:"strict,omitempty" doc:"是否启用严格模式"`
}
