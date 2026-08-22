package model

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// DemoAccessAudit Demo 访问审计记录
//
// 记录 demo 账户的登录事件与模块级 API 访问（含被白名单拒绝的探测尝试），
// 供 admin 在 Audit 子页排查「谁在什么时候用 demo 干了什么」。
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAudit struct {
	BaseModel
	Action    string `json:"action" gorm:"column:action;not null;index;comment:动作:login/login_denied/module_access/module_denied"`
	Module    string `json:"module" gorm:"column:module;not null;default:'';index;comment:demo 模块名，login 类为空串"`
	Path      string `json:"path" gorm:"column:path;not null;default:'';comment:请求路径"`
	IP        string `json:"ip" gorm:"column:ip;not null;default:'';comment:客户端 IP"`
	UserAgent string `json:"user_agent" gorm:"column:user_agent;not null;default:'';comment:User-Agent"`
	Reason    string `json:"reason" gorm:"column:reason;not null;default:'';comment:拒绝原因:login_disabled/no_demo_user/module_closed"`
}

// TableName 返回表名
//
//	@receiver DemoAccessAudit
//	@return string
func (DemoAccessAudit) TableName() string {
	return constant.DemoAccessAuditTableName
}
