package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListModelUserView 归属用户只读投影
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelUserView struct {
	ID     uint
	Name   string
	Avatar string
}

// ListModelEndpointView 所属端点只读投影（仅列表展示所需最小字段）
//
// 刻意不含 baseURL / apiKey：平铺列表面向模型编目，不应把上游凭据带进每行。
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelEndpointView struct {
	ID   uint
	Name string
}

// ListModelView 平铺模型列表行投影
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelView struct {
	ID              uint
	User            *ListModelUserView     // 归属用户；用户缺失或已删除时为 nil
	Endpoint        *ListModelEndpointView // 所属端点；端点缺失时为 nil
	Alias           string
	ModelID         string
	UpstreamModel   string // demo 权限下已脱敏
	Enabled         bool
	ContextLength   int
	MaxOutputTokens int
	Capabilities    []enum.InputModality
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ListModelQuery 平铺模型列表查询命令
//
// ScopeUserID 三态：nil（admin 视角）不过滤；非 nil 精确匹配 user_id。
// Username 仅在 ScopeUserID 为 nil（admin）时生效，按用户名解析出 scope。
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelQuery struct {
	model.CommonParam
	IsDemo      bool
	ScopeUserID *uint
	Username    string
	Status      string
	EndpointID  uint
	Capability  string
}

// ListModelHandler 平铺模型列表查询处理器
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelHandler interface {
	Handle(ctx context.Context, q ListModelQuery) ([]*ListModelView, *model.PageInfo, error)
}
