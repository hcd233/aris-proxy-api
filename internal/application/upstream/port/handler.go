// Package port defines application-layer ports for upstream use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// UpstreamUserView 归属用户只读投影（嵌套展示）
type UpstreamUserView struct {
	ID     uint
	Name   string
	Avatar string
}

// UpstreamEndpointView Endpoint 只读投影（分组头）
type UpstreamEndpointView struct {
	ID                          uint
	User                        *UpstreamUserView // 归属用户；用户缺失/软删时为 nil
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	MaskedAPIKey                string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// UpstreamModelView Model 只读投影（组内行）
type UpstreamModelView struct {
	ID              uint
	User            *UpstreamUserView // 归属用户；用户缺失/软删时为 nil
	Alias           string
	ModelID         string
	UpstreamModel   string
	Enabled         bool
	ContextLength   int
	MaxOutputTokens int
	Capabilities    []enum.InputModality
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpstreamGroupView 单个 endpoint 及其名下全部模型的分组视图
type UpstreamGroupView struct {
	Endpoint   *UpstreamEndpointView
	Models     []*UpstreamModelView
	ModelCount int  // 展示计数（截断后口径）
	Truncated  bool // 组内模型是否超过单组上限被截断
}

// ListUpstreamQuery 列出 upstream 分组的查询命令
//
// ScopeUserID 多租户隔离：>0 时只返回该用户的配置；==0（admin 视角）不过滤。
// CommonParam.Page/PageSize 的分页对象是 endpoint 组（每页 N 个端点）。
type ListUpstreamQuery struct {
	model.CommonParam
	IsDemo      bool
	ScopeUserID uint
	Username    string // 仅 admin 视角生效：按归属用户名过滤
}

// ListUpstreamHandler 查询处理器
type ListUpstreamHandler interface {
	// Handle 返回当前页的分组视图、当前筛选范围内的模型总数、以 endpoint 组数为 total 的分页信息。
	Handle(ctx context.Context, q ListUpstreamQuery) ([]*UpstreamGroupView, int64, *model.PageInfo, error)
}
