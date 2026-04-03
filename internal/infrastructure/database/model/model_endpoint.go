// Package model defines the database schema for the model.
//
//	update 2026-04-04 10:00:00
package model

// ModelEndpoint 模型端点配置数据库模型
//
// 对应原 config.yaml 中 model_list 下每个模型每个 provider 的端点配置。
// 一个模型别名（Alias）可以有多个端点（按 Provider 区分）。
//
//	@author centonhuang
//	@update 2026-04-04 10:00:00
type ModelEndpoint struct {
	BaseModel
	ID       uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:端点ID"`
	Alias    string `json:"alias" gorm:"column:alias;not null;index:idx_alias_provider,unique;comment:模型别名（对外暴露的名称）"`
	Model    string `json:"model" gorm:"column:model;not null;comment:上游实际模型名称"`
	Provider string `json:"provider" gorm:"column:provider;not null;index:idx_alias_provider,unique;comment:协议提供方(openai/anthropic)"`
	APIKey   string `json:"api_key" gorm:"column:api_key;not null;comment:上游API密钥"`
	BaseURL  string `json:"base_url" gorm:"column:base_url;not null;comment:上游基础URL"`
}
