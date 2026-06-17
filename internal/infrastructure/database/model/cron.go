package model

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// CronJob 定时任务元数据
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJob struct {
	Name        string    `gorm:"column:name;primary_key;comment:任务名"`
	Spec        string    `gorm:"column:spec;not null;comment:cron 表达式"`
	Description string    `gorm:"column:description;comment:任务描述"`
	Enabled     bool      `gorm:"column:enabled;default:true;comment:是否启用"`
	CreatedAt   time.Time `gorm:"comment:创建时间"`
	UpdatedAt   time.Time `gorm:"comment:更新时间"`
}

// TableName 返回表名
//
//	@receiver CronJob
//	@return string
func (CronJob) TableName() string {
	return constant.FieldTableCronJob
}

// CronCallAudit 定时任务执行审计
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronCallAudit struct {
	BaseModel
	CronName   string    `json:"cron_name" gorm:"column:cron_name;not null;comment:任务名;index"`
	TraceID    string    `json:"trace_id" gorm:"column:trace_id;comment:Trace ID"`
	StartedAt  time.Time `json:"started_at" gorm:"column:started_at;not null;comment:开始时间"`
	EndedAt    time.Time `json:"ended_at" gorm:"column:ended_at;comment:结束时间"`
	DurationMs int64     `json:"duration_ms" gorm:"column:duration_ms;comment:执行耗时(ms)"`
	Status     string    `json:"status" gorm:"column:status;not null;comment:执行状态"`
	Message    string    `json:"message" gorm:"column:message;comment:附加信息"`
}

// TableName 返回表名
//
//	@receiver CronCallAudit
//	@return string
func (CronCallAudit) TableName() string {
	return constant.FieldTableCronCallAudit
}
