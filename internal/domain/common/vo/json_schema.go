// Package vo 提供领域通用的值对象。
//
// 注意1：本包中的 JSONSchemaProperty 使用了 sonic.NoCopyRawMessage（等价于
// json.RawMessage）。这是 JSON Schema 表示固有需要的——字段如 default、const、
// enum、additionalProperties 可以是任意 JSON 值，没有类型安全的替代方案。
// 本包属于"禁止 json.RawMessage"规则的故意豁免。
//
// 注意2：由于 Huma v2 OpenAPI 规范生成需要，部分 VO 实现了 huma.SchemaProvider
// 接口，引入了对 huma v2 的依赖。这是一个有意的权衡——保持 Schema 定义与类型定义
// 在一起，避免在 application/infrastructure 层维护重复的 OpenAPI 注解。
package vo

import (
	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
)

// JSONSchemaTypeValue JSON Schema `type` 字段联合类型（string 或 string[]）
//
// 部分客户端（如 Codex Desktop）会在工具参数 schema 中传递
// `"type": ["string", "null"]`。为兼容该合法写法，这里支持
// 字符串和字符串数组两种形态。
//
//	@author centonhuang
//	@update 2026-04-22 14:00:00
type JSONSchemaTypeValue struct {
	Single string   `json:"-"`
	Multi  []string `json:"-"`
}

// UnmarshalJSON 解析 string 或 string[]
//
//	@receiver t *JSONSchemaTypeValue
//	@param data []byte
//	@return error
//	@author centonhuang
//	@update 2026-04-22 14:00:00
func (t *JSONSchemaTypeValue) UnmarshalJSON(data []byte) error {
	var single string
	if err := sonic.Unmarshal(data, &single); err == nil {
		t.Single = single
		t.Multi = nil
		return nil
	}

	var multi []string
	if err := sonic.Unmarshal(data, &multi); err == nil {
		t.Single = ""
		t.Multi = multi
		return nil
	}

	return sonic.Unmarshal(data, &single)
}

// MarshalJSON 按原始分支输出
//
//	@receiver t JSONSchemaTypeValue
//	@return []byte
//	@return error
//	@author centonhuang
//	@update 2026-04-22 14:00:00
func (t JSONSchemaTypeValue) MarshalJSON() ([]byte, error) {
	if len(t.Multi) > 0 {
		return sonic.Marshal(t.Multi)
	}
	return sonic.Marshal(t.Single)
}

// HasType 判断是否包含目标 type 字面量
//
//	@receiver t *JSONSchemaTypeValue
//	@param typeName string
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 14:00:00
func (t *JSONSchemaTypeValue) HasType(typeName string) bool {
	if t == nil {
		return false
	}
	if t.Single == typeName {
		return true
	}
	for _, v := range t.Multi {
		if v == typeName {
			return true
		}
	}
	return false
}

// JSONSchemaProperty 递归 JSON Schema 属性定义，覆盖标准 JSON Schema 字段
//
//	@author centonhuang
//	@update 2026-04-22 14:00:00
type JSONSchemaProperty struct {
	Type                 *JSONSchemaTypeValue           `json:"type,omitempty" doc:"数据类型(string/number/integer/boolean/array/object/null，支持 string 或 string[])"`
	Description          string                         `json:"description,omitempty" doc:"属性描述"`
	Properties           map[string]*JSONSchemaProperty `json:"properties,omitempty" doc:"对象属性定义(递归)"`
	Items                *JSONSchemaProperty            `json:"items,omitempty" doc:"数组元素定义(递归)"`
	Required             []string                       `json:"required,omitempty" doc:"必填字段列表"`
	Enum                 []sonic.NoCopyRawMessage       `json:"enum,omitempty" doc:"枚举值列表"`
	Const                sonic.NoCopyRawMessage         `json:"const,omitempty" doc:"常量值"`
	Default              sonic.NoCopyRawMessage         `json:"default,omitempty" doc:"默认值"`
	AnyOf                []*JSONSchemaProperty          `json:"anyOf,omitempty" doc:"任意匹配"`
	OneOf                []*JSONSchemaProperty          `json:"oneOf,omitempty" doc:"唯一匹配"`
	AllOf                []*JSONSchemaProperty          `json:"allOf,omitempty" doc:"全部匹配"`
	Not                  *JSONSchemaProperty            `json:"not,omitempty" doc:"取反"`
	AdditionalProperties sonic.NoCopyRawMessage         `json:"additionalProperties,omitempty" doc:"额外属性(bool或JSONSchemaProperty)"`
	Strict               *bool                          `json:"strict,omitempty" doc:"是否启用严格模式"`

	// 数值验证
	Minimum          *float64 `json:"minimum,omitempty" doc:"最小值"`
	Maximum          *float64 `json:"maximum,omitempty" doc:"最大值"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty" doc:"排他最小值"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty" doc:"排他最大值"`
	MultipleOf       *float64 `json:"multipleOf,omitempty" doc:"倍数"`

	// 字符串验证
	MinLength *int   `json:"minLength,omitempty" doc:"最小长度"`
	MaxLength *int   `json:"maxLength,omitempty" doc:"最大长度"`
	Pattern   string `json:"pattern,omitempty" doc:"正则表达式模式"`
	Format    string `json:"format,omitempty" doc:"格式(如date-time, email等)"`

	// 数组验证
	MinItems    *int  `json:"minItems,omitempty" doc:"最小元素数"`
	MaxItems    *int  `json:"maxItems,omitempty" doc:"最大元素数"`
	UniqueItems *bool `json:"uniqueItems,omitempty" doc:"元素是否唯一"`

	// 对象验证
	MinProperties     *int                           `json:"minProperties,omitempty" doc:"最小属性数"`
	MaxProperties     *int                           `json:"maxProperties,omitempty" doc:"最大属性数"`
	PatternProperties map[string]*JSONSchemaProperty `json:"patternProperties,omitempty" doc:"正则匹配的属性定义"`
	PropertyNames     *JSONSchemaProperty            `json:"propertyNames,omitempty" doc:"属性名称约束"`

	// 其他标准字段
	Title     string                         `json:"title,omitempty" doc:"标题"`
	Ref       string                         `json:"$ref,omitempty" doc:"JSON Schema引用"`
	SchemaURI string                         `json:"$schema,omitempty" doc:"JSON Schema版本"`
	Defs      map[string]*JSONSchemaProperty `json:"$defs,omitempty" doc:"JSON Schema定义"`
}

// Schema 实现 huma.SchemaProvider 接口，告诉 Huma 此类型是自由格式的 JSON Schema 对象
//
// JSON Schema 本身是递归多态结构，字段如 additionalProperties 可以是 bool 或嵌套 schema、
// default 可以是任意 JSON 值，无法用固定的 OpenAPI schema 描述。
// 因此直接声明为 object 类型，跳过 Huma 的严格类型校验。
//
//	@receiver JSONSchemaProperty
//	@param r huma.Registry
//	@return *huma.Schema
//	@author centonhuang
//	@update 2026-04-22 14:00:00
func (JSONSchemaProperty) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{Type: "object"}
}

// HasType 判断 JSON Schema 的 type 是否包含给定类型
//
//	@receiver p *JSONSchemaProperty
//	@param typeName string
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 14:00:00
func (p *JSONSchemaProperty) HasType(typeName string) bool {
	if p == nil {
		return false
	}
	return p.Type.HasType(typeName)
}
