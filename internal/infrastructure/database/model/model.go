package model

// Model 模型关联数据库模型
//
// 记录对外暴露的模型别名与上游端点的关联关系。
// 同一 alias 可通过多条记录关联多个 endpoint，解析时随机选择。
type Model struct {
	BaseModel
	ID              uint               `json:"id" gorm:"column:id;primary_key;auto_increment;comment:模型关联ID"`
	UserID          uint               `json:"user_id" gorm:"column:user_id;not null;default:0;uniqueIndex:idx_model_alias_endpoint_deleted,priority:1;comment:归属用户ID(逻辑外键→users.id)"`
	Alias           string             `json:"alias" gorm:"column:alias;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:2;comment:对外暴露的模型别名"`
	ModelID         string             `json:"model_id" gorm:"column:model_id;not null;default:'';comment:业务模型ID(创建默认=alias,可更新)"`
	UpstreamModel   string             `json:"upstream_model" gorm:"column:upstream_model;not null;default:'';comment:上游实际模型名"`
	EndpointID      uint               `json:"endpoint_id" gorm:"column:endpoint_id;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:3;comment:逻辑外键→endpoint.id"`
	Enabled         bool               `json:"enabled" gorm:"column:enabled;default:true;comment:是否启用"`
	Priority        int                `json:"priority" gorm:"column:priority;not null;default:0;comment:调度优先级,数字小=优先级高"`
	Weight          int                `json:"weight" gorm:"column:weight;not null;default:1;comment:同优先级加权随机权重(>0)"`
	ContextLength   int                `json:"context_length" gorm:"column:context_length;default:0;comment:上下文窗口长度(tokens)"`
	MaxOutputTokens int                `json:"max_output_tokens" gorm:"column:max_output_tokens;default:0;comment:最大输出长度(tokens)"`
	Capabilities    []string           `json:"capabilities" gorm:"column:capabilities;not null;default:'[\"text\"]';comment:模型能力（输入模态集合，如 text/image）;serializer:json"`
	PricingRules    []ModelPricingRule `json:"pricing_rules" gorm:"column:pricing_rules;not null;default:'[]';comment:定价规则(微单位单价,数组顺序=匹配优先级);serializer:json"`
	PricingCurrency string             `json:"pricing_currency" gorm:"column:pricing_currency;not null;default:'';comment:计价币种(''/USD,空=未计价)"`
	DeletedAt       int64              `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_model_alias_endpoint_deleted,priority:4;comment:删除时间"`
}

// ModelTimeWindow 时段窗口 DB 形态：[start,end) 半开区间，end<start 跨午夜；
// days 空=每天（1=周一…7=周日）；timezone 为 IANA 时区名（空按 UTC）。
type ModelTimeWindow struct {
	Days     []int  `json:"days"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// ModelPricingRule 定价规则 DB 形态（四价：微单位/1M tokens）
type ModelPricingRule struct {
	TimeWindows               []ModelTimeWindow `json:"time_windows"`
	ContextMin                int64             `json:"context_min"`
	ContextMax                int64             `json:"context_max"`
	InputPriceMicro           int64             `json:"input_price_micro"`
	OutputPriceMicro          int64             `json:"output_price_micro"`
	CacheCreationPriceMicro   int64             `json:"cache_creation_price_micro"`
	CacheCreation1hPriceMicro int64             `json:"cache_creation_1h_price_micro"`
	CacheReadPriceMicro       int64             `json:"cache_read_price_micro"`
}
