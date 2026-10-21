package llmproxy

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// EndpointRepository Endpoint 聚合根仓储接口
//
// scopeUserID 语义（多租户隔离，三态显式）：nil 时查询/删除不限定用户（仅 admin 视角）；
// 非 nil（含 0）时精确匹配 user_id，0 用于共享池（user_id=0 的存量/共享配置）。
// 禁止用零值 uint 兼作"全量视角"哨兵——那会让认证缺失静默退化为全平台可见。
type EndpointRepository interface {
	FindByID(ctx context.Context, id uint, scopeUserID *uint) (*aggregate.Endpoint, error)
	BatchFindByIDs(ctx context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error)
	Create(ctx context.Context, endpoint *aggregate.Endpoint, ownerUserID uint) (uint, error)
	Update(ctx context.Context, endpoint *aggregate.Endpoint) error // 归属校验由调用方 FindByID 完成
	Delete(ctx context.Context, id uint, scopeUserID *uint) error
	DeleteCascade(ctx context.Context, id uint, scopeUserID *uint) error
	List(ctx context.Context) ([]*aggregate.Endpoint, error)
	Paginate(ctx context.Context, param model.CommonParam, scopeUserID *uint) ([]*aggregate.Endpoint, *model.PageInfo, error)
	// FindIDsByScope 按租户范围返回全部可见 endpoint ID 列表（id 升序）；
	// scopeUserID 为 nil（admin 视角）不过滤；includePublic 为 true 时并集公开端点
	// （普通用户视角）。用于 upstream 分组视图的分页基数。
	FindIDsByScope(ctx context.Context, scopeUserID *uint, includePublic bool) ([]uint, error)
}

// ModelIDSyncCounts 历史 model ID 替换的三表影响行数
//
// 未命中任何行时三值均为 0（幂等）。
type ModelIDSyncCounts struct {
	AuditCount   int64
	SessionCount int64
	MessageCount int64
}

// ModelRepository Model 聚合根仓储接口
//
// scopeUserID 语义同 EndpointRepository；FindByAlias 为网关解析专用，userID 传真实值，
// 传 0（共享池）需在网关回退开关下使用。
type ModelRepository interface {
	FindByAlias(ctx context.Context, alias vo.EndpointAlias, userID *uint) ([]*aggregate.Model, error)
	// FindPublicByAlias 查询公开端点下启用中的模型（网关解析的公开池回退）。
	FindPublicByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error)
	FindByID(ctx context.Context, id uint, scopeUserID *uint) (*aggregate.Model, error)
	Create(ctx context.Context, model *aggregate.Model, ownerUserID uint) (uint, error)
	Update(ctx context.Context, model *aggregate.Model) error
	Delete(ctx context.Context, id uint, scopeUserID *uint) error
	DeleteByEndpointID(ctx context.Context, endpointID uint) error // 级联删除前调用方已校验端点归属
	List(ctx context.Context) ([]*aggregate.Model, error)
	Paginate(ctx context.Context, param model.CommonParam, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error)
	// ListByEndpointIDs 批量拉取一组 endpoint 名下的 model 聚合（不做二次 scope 过滤，
	// 调用方传入的 endpointIDs 必须已经过 scope 解析）。未命中的 endpointID 自然返回空集合。
	ListByEndpointIDs(ctx context.Context, endpointIDs []uint) ([]*aggregate.Model, error)
	// PaginateWithFilter 带筛选的分页查询（Web 平铺模型列表专用）。
	//
	// scopeUserID 采用 *uint 三态：nil（admin 视角）不过滤；非 nil 精确匹配 user_id。
	// 禁止用零值兼作全量哨兵（==0 的 uint 在这里会退化成"查共享池"而非"查全部"）。
	// filter 各字段为零值时该维度不过滤；param.SortField 必须落在 constant.ModelListSortFields
	// 内，否则回退 ModelListDefaultSortField，不报错。
	PaginateWithFilter(ctx context.Context, param model.CommonParam, filter ModelListFilter, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error)
	// UpdateWithHistorySync 在单事务内更新模型聚合，并把归属 user 的历史数据中业务模型 ID
	// oldModelID 批量替换为聚合当前的 ModelID。
	//
	// 模型更新与历史替换必须原子：若分开执行，替换失败时模型本体已改名，同样的更新请求
	// 重试时新旧 ID 相等、同步条件不再触发，历史将永久停留在旧 ID（2026-09-08 CR P1）。
	// 单事务内依次更新 model 行、替换 model_call_audit / session / message 三表，
	// 任一步失败整体回滚；返回各表影响行数，message 的 scope 为「实际命中的会话引用到
	// 的消息」（见 spec 2026-09-04-model-id-history-sync §5.4）。替换以模型归属 user
	// （m.UserID()）为界，不排除已删除的 API Key。
	UpdateWithHistorySync(ctx context.Context, m *aggregate.Model, oldModelID string) (ModelIDSyncCounts, error)
}

// OAuthCredentialRepository 订阅账号 OAuth 凭据仓储接口
//
// 加解密是持久化层职责：聚合持明文令牌，实现负责落库前加密、读出后解密。
// 加密密钥缺失时读写直接失败（fail-closed），API Key 端点不受影响。
//
// 额度与错误摘要只走窄列更新（UpdateQuota / UpdateLastError）：Save 是按 endpoint_id
// 的整行 upsert，维护任务在刷新之后调用会把新 token 覆盖回内存快照里的旧值。
type OAuthCredentialRepository interface {
	// FindByEndpointID 按端点查询凭据；不存在返回 (nil, nil)。
	FindByEndpointID(ctx context.Context, endpointID uint) (*aggregate.OAuthCredential, error)
	// Save 按 endpoint_id upsert（创建与更新共用同一入口）。
	Save(ctx context.Context, cred *aggregate.OAuthCredential) error
	// ListActive 列出全部 active 状态凭据（cron 维护：临期刷新与额度探测）。
	ListActive(ctx context.Context) ([]*aggregate.OAuthCredential, error)
	// ListDisplayByEndpointIDs 批量读取一组端点的凭据展示投影（不含 token，不解密）。
	//
	// 供 upstream 列表等只读场景使用：避免逐端点查询（N+1）与逐行解密 token。
	// 未命中的 endpointID 不出现在返回 map 中。
	ListDisplayByEndpointIDs(ctx context.Context, endpointIDs []uint) (map[uint]vo.OAuthCredentialDisplay, error)
	// UpdateQuota 只更新额度快照与采集时间（不触碰 token/状态列）。
	UpdateQuota(ctx context.Context, endpointID uint, snapshot vo.QuotaSnapshot, at time.Time) error
	// UpdateLastError 只更新最近错误摘要（不触碰 token/状态列）。
	UpdateLastError(ctx context.Context, endpointID uint, lastError string) error
}

// OAuthEndpointRepository 订阅账号端点的原子创建接口（由 endpointRepository 一并实现）
//
// 端点与凭据必须同事务落库：分两次写会留下「OAuth 端点无凭据」的中间态，
// 该状态下解析层按 reauth_required 剔除、用户端表现为端点不可用且无法修复。
type OAuthEndpointRepository interface {
	// CreateWithOAuthCredential 在单事务内创建 OAuth 端点与其凭据（凭据落库加密）。
	CreateWithOAuthCredential(ctx context.Context, endpoint *aggregate.Endpoint, credential *aggregate.OAuthCredential, ownerUserID uint) (uint, error)
}

// ModelListFilter 模型列表筛选条件（零值表示该维度不过滤）
//
// Status 取 constant.ModelStatusEnabled / ModelStatusDisabled，其余值视为不过滤；
// Capability 取 enum.InputModality 成员的字符串形式，未知值视为不过滤。
type ModelListFilter struct {
	Status     string
	EndpointID uint
	Capability string
	// MissingOnly 仅保留配置缺失（未计价或未填规格）的模型。
	MissingOnly bool
	// IncludePublic 并集公开端点下的模型（普通用户视角）。
	IncludePublic bool
}

// ==================== CQRS 读模型 ====================

// ModelAliasProjection 模型别名只读投影
type ModelAliasProjection struct {
	Alias string
}

// ModelDetailProjection 启用模型的完整只读投影
type ModelDetailProjection struct {
	Alias           string
	UpstreamModel   string
	ContextLength   int
	MaxOutputTokens int
	Capabilities    []string
	Pricing         vo.Pricing
}

// EndpointProjection 端点只读投影
type EndpointProjection struct {
	ID                          uint
	Name                        string
	OpenaiBaseURL               string
	AnthropicBaseURL            string
	APIKey                      string
	SupportOpenAIChatCompletion bool
	SupportOpenAIResponse       bool
	SupportAnthropicMessage     bool
}

// EndpointReadRepository CQRS 读模型仓储接口
//
// userID 语义：网关路径必传真实用户 ID；0（认证缺失）防御性返回空结果，
// 绝不退化为全平台可见（与实现侧 ListAliases 等方法的 fail-closed 行为一致）。
type EndpointReadRepository interface {
	ListAliases(ctx context.Context, userID uint) ([]*ModelAliasProjection, error)
	ListEnabledModelDetails(ctx context.Context, userID uint) ([]*ModelDetailProjection, error)
	FindEndpointByAlias(ctx context.Context, userID uint, alias string, matcher func(*EndpointProjection) bool) (*EndpointProjection, *ModelAliasProjection, error)
}
