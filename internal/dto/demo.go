// Package dto Demo 演示账户DTO
package dto

import (
	"time"
)

// DemoLoginRsp Demo 登录响应（JWT token pair）
type DemoLoginRsp struct {
	CommonRsp
	AccessToken  string `json:"accessToken,omitempty" doc:"JWT access token of the demo user"`
	RefreshToken string `json:"refreshToken,omitempty" doc:"JWT refresh token of the demo user"`
}

// DemoStatusRsp Demo 登录入口状态响应（无需鉴权，登录页按钮显隐）
type DemoStatusRsp struct {
	CommonRsp
	LoginEnabled   bool `json:"loginEnabled" doc:"Whether the demo login entry is enabled"`
	DemoUserExists bool `json:"demoUserExists" doc:"Whether a demo account is configured"`
}

// DemoConfig Demo 配置实体
type DemoConfig struct {
	LoginEnabled  bool      `json:"loginEnabled" doc:"Whether the demo login entry is enabled"`
	SampleModulus uint      `json:"sampleModulus" doc:"Modulus for behavior data sampling (id % K == 0, must be >= 2)"`
	Modules       []string  `json:"modules" doc:"Open modules for the demo account"`
	UpdatedAt     time.Time `json:"updatedAt,omitzero" doc:"Last update time"`
}

// GetDemoConfigRsp 读取 Demo 配置响应
type GetDemoConfigRsp struct {
	CommonRsp
	Config *DemoConfig `json:"config,omitempty" doc:"Demo configuration"`
}

// UpdateDemoConfigReq 更新 Demo 配置请求（admin）
type UpdateDemoConfigReq struct {
	Body *UpdateDemoConfigReqBody `json:"body" doc:"Request body containing demo config fields to update"`
}

// UpdateDemoConfigReqBody Demo 配置可更新字段（nil = 不修改）
type UpdateDemoConfigReqBody struct {
	Config *DemoConfigBody `json:"config" required:"true" doc:"Demo config fields to update"`
}

// DemoConfigBody Demo 配置更新体
type DemoConfigBody struct {
	LoginEnabled  *bool    `json:"loginEnabled,omitempty" doc:"Whether the demo login entry is enabled"`
	SampleModulus *uint    `json:"sampleModulus,omitempty" minimum:"2" doc:"Modulus for behavior data sampling (id % K == 0, must be >= 2)"`
	Modules       []string `json:"modules,omitempty" doc:"Open modules for the demo account (dashboard/sessions/audit/models/trigger/endpoints/monitor/cron/cron_audit)"`
}
