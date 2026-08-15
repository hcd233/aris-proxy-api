// Package port Demo 演示账户应用的端口定义（handler 接口 + 视图 + 命令）
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// DemoConfigView Demo 配置视图
type DemoConfigView struct {
	LoginEnabled  bool
	SampleModulus uint
	Modules       []enum.DemoModule
	UpdatedAt     time.Time
}

// GetDemoConfigQuery 读取 Demo 配置
type GetDemoConfigQuery struct{}

// GetDemoConfigHandler 读取 Demo 配置
type GetDemoConfigHandler interface {
	Handle(ctx context.Context, q GetDemoConfigQuery) (*DemoConfigView, error)
}

// UpdateDemoConfigCommand 更新 Demo 配置；nil 字段表示不修改
type UpdateDemoConfigCommand struct {
	LoginEnabled  *bool
	SampleModulus *uint
	Modules       []enum.DemoModule
}

// UpdateDemoConfigHandler 更新 Demo 配置（admin）
type UpdateDemoConfigHandler interface {
	Handle(ctx context.Context, cmd UpdateDemoConfigCommand) (*DemoConfigView, error)
}

// DemoLoginCommand Demo 账户登录（无需 OAuth）
type DemoLoginCommand struct{}

// DemoLoginResult Demo 登录结果（JWT token pair）
type DemoLoginResult struct {
	AccessToken  string
	RefreshToken string
	UserID       uint
}

// DemoLoginHandler Demo 账户登录
type DemoLoginHandler interface {
	Handle(ctx context.Context, cmd DemoLoginCommand) (*DemoLoginResult, error)
}

// DemoStatusQuery 登录页 Demo 入口状态查询（无需鉴权）
type DemoStatusQuery struct{}

// DemoStatusResult Demo 入口状态
type DemoStatusResult struct {
	LoginEnabled   bool
	DemoUserExists bool
}

// DemoStatusHandler Demo 入口状态查询
type DemoStatusHandler interface {
	Handle(ctx context.Context, q DemoStatusQuery) (*DemoStatusResult, error)
}

// DemoConfigRepository Demo 配置存取仓储（单行表）
type DemoConfigRepository interface {
	// Get 读取配置行；表为空时返回零值配置（不自动落库）
	Get(ctx context.Context) (*DemoConfigEntity, error)
	// Save 保存配置行（固定单例 ID，全字段覆盖）
	Save(ctx context.Context, entity *DemoConfigEntity) error
}

// DemoConfigEntity 配置仓储实体（与 DB 解耦的基本类型）
type DemoConfigEntity struct {
	LoginEnabled  bool
	SampleModulus uint
	Modules       []enum.DemoModule
	UpdatedAt     time.Time
}

// DemoModuleAccessor Demo 模块放行判断（供权限中间件使用，读取失败 fail-closed）
type DemoModuleAccessor interface {
	// IsModuleOpen 判断模块是否对 Demo 开放；配置读取失败返回 false
	IsModuleOpen(ctx context.Context, module enum.DemoModule) bool
}

// DemoScopeProvider 提供 Demo 数据视角参数（行为数据取模抽样模数）
type DemoScopeProvider interface {
	// SampleModulus 返回抽样模数（>=2）；配置读取失败返回 error，调用方必须拒绝请求
	SampleModulus(ctx context.Context) (uint, error)
}
