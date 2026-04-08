package model

// ProxyAPIKey 代理API密钥数据库模型
//
// 对应原 config.yaml 中的 api_keys 配置。
// 存储代理自身对外暴露的 API Key，用于客户端认证。
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type ProxyAPIKey struct {
	BaseModel
	ID        uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:密钥ID"`
	UserID    uint   `json:"userId" gorm:"column:user_id;not null;uniqueIndex:idx_user_id_name_deleted,priority:1;comment:所属用户ID"`
	Name      string `json:"name" gorm:"column:name;not null;uniqueIndex:idx_user_id_name_deleted,priority:2;comment:密钥名称"`
	Key       string `json:"key" gorm:"column:key;not null;uniqueIndex:idx_key_deleted,priority:1;comment:API密钥值"`
	DeletedAt int64  `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_user_id_name_deleted,priority:3;uniqueIndex:idx_key_deleted,priority:2;comment:删除时间，默认为0"`
}
