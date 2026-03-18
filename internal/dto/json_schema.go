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

	// 其他标准字段
	Title  string                         `json:"title,omitempty" doc:"标题"`
	Ref    string                         `json:"$ref,omitempty" doc:"JSON Schema引用"`
	Schema string                         `json:"$schema,omitempty" doc:"JSON Schema版本"`
	Defs   map[string]*JSONSchemaProperty `json:"$defs,omitempty" doc:"JSON Schema定义"`
}
