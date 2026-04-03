// Package model defines the database schema for the model.
//
//	update 2026-04-04 10:00:00
package model

// ProxyAPIKey 代理API密钥数据库模型
//
// 对应原 config.yaml 中的 api_keys 配置。
// 存储代理自身对外暴露的 API Key，用于客户端认证。
//
//	@author centonhuang
//	@update 2026-04-04 10:00:00
type ProxyAPIKey struct {
	BaseModel
	ID   uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:密钥ID"`
	Name string `json:"name" gorm:"column:name;uniqueIndex;not null;comment:密钥名称（对应用户标识）"`
	Key  string `json:"key" gorm:"column:key;uniqueIndex;not null;comment:API密钥值"`
}
