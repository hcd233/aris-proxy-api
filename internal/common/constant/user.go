package constant

import "time"

const (
	// PeriodManageUser 用户管理接口限流窗口
	PeriodManageUser = 1 * time.Minute
	// LimitManageUser 用户管理接口限流次数
	LimitManageUser = 20
)
