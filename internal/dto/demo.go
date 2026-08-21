// Package dto Demo 演示账户DTO
package dto

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
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
	LoginEnabled bool      `json:"loginEnabled" doc:"Whether the demo login entry is enabled"`
	Modules      []string  `json:"modules" doc:"Open modules for the demo account"`
	UpdatedAt    time.Time `json:"updatedAt,omitzero" doc:"Last update time"`
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
	LoginEnabled *bool    `json:"loginEnabled,omitempty" doc:"Whether the demo login entry is enabled"`
	Modules      []string `json:"modules,omitempty" doc:"Open modules for the demo account (dashboard/sessions/audit/models/trigger/endpoints/monitor/cron/cron_audit)"`
}

// DemoSession 白名单会话摘要
type DemoSession struct {
	ID           uint      `json:"id" doc:"Session ID"`
	Summary      string    `json:"summary,omitempty" doc:"会话摘要"`
	Score        *int      `json:"score,omitempty" doc:"人工评分(1-5)"`
	MessageCount int       `json:"messageCount" doc:"消息数"`
	ToolCount    int       `json:"toolCount" doc:"工具调用数"`
	CreatedAt    time.Time `json:"createdAt,omitzero" doc:"创建时间"`
	ModelIDs     []string  `json:"modelIds,omitempty" doc:"回答模型ID列表"`
}

// ListDemoSessionsReq 白名单会话列表请求
type ListDemoSessionsReq struct {
	Page     int `query:"page" required:"true" minimum:"1" doc:"页码"`
	PageSize int `query:"pageSize" required:"true" minimum:"1" maximum:"500" doc:"每页条数"`
}

// ListDemoSessionsRsp 白名单会话列表响应
type ListDemoSessionsRsp struct {
	CommonRsp
	Sessions []*DemoSession  `json:"sessions,omitempty" doc:"白名单会话列表"`
	PageInfo *model.PageInfo `json:"pageInfo,omitempty" doc:"分页信息"`
}

// AddDemoSessionsReq 批量添加白名单会话请求
type AddDemoSessionsReq struct {
	Body *AddDemoSessionsReqBody `json:"body" required:"true" doc:"请求体"`
}

// AddDemoSessionsReqBody 批量添加白名单会话请求体
type AddDemoSessionsReqBody struct {
	SessionIDs []uint `json:"sessionIds" required:"true" minItems:"1" doc:"会话 ID 列表"`
}

// RemoveDemoSessionsReq 批量移除白名单会话请求
type RemoveDemoSessionsReq struct {
	IDs []uint `query:"ids" required:"true" doc:"会话 ID 列表，逗号分隔"`
}

// RemoveDemoSessionsRsp 批量移除白名单会话响应
type RemoveDemoSessionsRsp struct {
	CommonRsp
}
