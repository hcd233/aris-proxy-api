package model

// Model 模型关联数据库模型
//
// 记录对外暴露的模型别名与上游端点的关联关系。
// 同一 alias 可通过多条记录关联多个 endpoint，解析时随机选择。
type Model struct {
	BaseModel
	ID              uint     `json:"id" gorm:"column:id;primary_key;auto_increment;comment:模型关联ID"`
	Alias           string   `json:"alias" gorm:"column:alias;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:1;comment:对外暴露的模型别名"`
	ModelID         string   `json:"model_id" gorm:"column:model_id;not null;default:'';comment:业务模型ID(创建默认=alias,可更新)"`
	ModelName       string   `json:"model_name" gorm:"column:model;not null;comment:上游实际模型名"`
	EndpointID      uint     `json:"endpoint_id" gorm:"column:endpoint_id;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:2;comment:逻辑外键→endpoint.id"`
	Enabled         bool     `json:"enabled" gorm:"column:enabled;default:true;comment:是否启用"`
	ContextLength   int      `json:"context_length" gorm:"column:context_length;default:0;comment:上下文窗口长度(tokens)"`
	MaxOutputTokens int      `json:"max_output_tokens" gorm:"column:max_output_tokens;default:0;comment:最大输出长度(tokens)"`
	Capabilities    []string `json:"capabilities" gorm:"column:capabilities;not null;default:'[\"text\"]';comment:模型能力（输入模态集合，如 text/image）;serializer:json"`
	DeletedAt       int64    `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_model_alias_endpoint_deleted,priority:3;comment:删除时间"`
}
