package model

import "github.com/hcd233/aris-proxy-api/internal/common/constant"

// Trigger 触发词数据库模型。
//
// 索引说明（fix/trigger-word-recreate-2026-08-12）：
//   - DeletedAt 在此重声明以覆盖 BaseModel.DeletedAt，仅为给本表挂 (word, deleted_at)
//     复合唯一索引 tag，避免污染其他继承 BaseModel 的表（仿 model_call_audit.go 覆盖写法）。
//   - 索引名由 idx_word_deleted_at 改为 idx_trigger_word_deleted：旧索引已作为 word 单列唯一
//     索引存在（BaseModel.DeletedAt 未挂 priority:2，导致唯一约束不区分软删除状态），
//     GORM AutoMigrate 检测到同名索引会跳过创建，无法升级为复合索引；换新名后 AutoMigrate
//     会创建 (word, deleted_at) 复合唯一索引，软删后同词可重新添加。
//   - 生产库需手动 DROP 旧单列唯一索引 idx_word_deleted_at（否则唯一约束仍生效），见部署说明。
type Trigger struct {
	BaseModel
	DeletedAt int64  `json:"deleted_at" gorm:"column:deleted_at;default:0;comment:删除时间，默认为0;uniqueIndex:idx_trigger_word_deleted,priority:2"`
	Word      string `json:"word" gorm:"column:word;type:varchar(512);not null;uniqueIndex:idx_trigger_word_deleted,priority:1;comment:触发词"`
	HitCount  uint   `json:"hit_count" gorm:"column:hit_count;not null;default:0;comment:命中次数"`
	Action    string `json:"action" gorm:"column:action;type:varchar(16);not null;default:deny;comment:命中处理动作"`
}

func (Trigger) TableName() string {
	return constant.TriggerTableName
}
