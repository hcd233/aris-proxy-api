package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/samber/mo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// modelPageCache PaginateWithFilter 的读缓存载荷（DB 行 + 分页信息）
//
// 缓存 DB 行而非领域聚合：aggregate.Model 是私有字段聚合根不可序列化，
// dbmodel.Model 全字段 JSON 可序列化，回源后仍经 toModelAggregate 装配。
type modelPageCache struct {
	Records  []*dbmodel.Model `json:"records"`
	PageInfo *model.PageInfo  `json:"pageInfo"`
}

// modelListCacheSign 模型列表缓存键的查询签名（scope 已编码在键前缀，不重复入签名）
type modelListCacheSign struct {
	Param  model.CommonParam        `json:"param"`
	Filter llmproxy.ModelListFilter `json:"filter"`
}

// modelListCacheKey 计算 Web 平铺模型列表缓存键
//
// 键 = scope 片段（userID 或 admin 全量）+ 查询签名（分页/筛选/排序参数）的 SHA-256 摘要。
// 查询参数由前端任意组合，摘要保证键长固定且不碰撞。
//
//	@param param model.CommonParam 已完成排序列白名单归一化的列表参数
//	@param filter llmproxy.ModelListFilter
//	@param scopeUserID *uint nil = admin 全量视角
//	@return string
func modelListCacheKey(param model.CommonParam, filter llmproxy.ModelListFilter, scopeUserID *uint) string {
	scope := constant.ReadCacheModelListScopeAll
	if scopeUserID != nil {
		scope = strconv.FormatUint(uint64(*scopeUserID), constant.DecimalBase)
	}
	sign, _ := sonic.MarshalString(modelListCacheSign{Param: param, Filter: filter}) //nolint:errcheck // 纯数据结构序列化不会失败
	sum := sha256.Sum256([]byte(sign))
	return fmt.Sprintf(constant.ReadCacheModelListKeyTemplate, scope, hex.EncodeToString(sum[:]))
}

// readCacheKey 由用途 + 参数签名摘要构造时间窗查询缓存键。
//
// 用途（如 "audit:distinct:model"）与全部影响结果的参数一起进摘要：键长固定、无分隔符歧义，
// 调用方必须把 scope（apiKeyIDs / ownerIDs / sessionIDs）显式放进 parts——漏传即跨租户串数据。
//
//	@param purpose 用途标识，需全局唯一
//	@param parts 影响结果的全部参数
//	@return string
func readCacheKey(purpose string, parts ...any) string {
	sign, _ := sonic.MarshalString(parts) //nolint:errcheck // 纯数据结构序列化不会失败
	sum := sha256.Sum256([]byte(purpose + "\x00" + sign))
	return fmt.Sprintf(constant.ReadCacheWindowKeyTemplate, hex.EncodeToString(sum[:]))
}

// cacheScopeIDs 把 ID 集合归一化为升序去重切片，保证同一集合不同顺序得到同一个键。
//
// nil 必须保持 nil（语义是 admin 全量），空非 nil 保持空（语义是名下无 Key 的越权短路），
// 两者不可合并——否则无 Key 用户可能命中 admin 的全量缓存。
func cacheScopeIDs(ids []uint) []uint {
	if ids == nil {
		return nil
	}
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

// cacheTimeBucket 把查询时间窗对齐到缓存键粒度；零值原样返回。
func cacheTimeBucket(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.Truncate(constant.ReadCacheWindowBucket)
}

// cachedSlice 时间窗查询的 cache-aside 封装。
//
// 空结果由 GetOrLoad 写空值标记（防穿透），回源错误不写缓存，Redis 故障由 ReadCache fail-open 回源。
//
//	@param ctx context.Context
//	@param rc *cache.ReadCache 为 nil 时退化为直接回源
//	@param key 缓存键（readCacheKey 产出）
//	@param loader 回源加载器
//	@return []T
//	@return error
func cachedSlice[T any](ctx context.Context, rc *cache.ReadCache, key string, loader func(context.Context) ([]T, error)) ([]T, error) {
	opt, err := cache.GetOrLoadWithTTL(ctx, rc, key, constant.ReadCacheWindowTTL, constant.ReadCacheWindowTTLJitter,
		func(loadCtx context.Context) (mo.Option[[]T], error) {
			values, loadErr := loader(loadCtx)
			if loadErr != nil {
				return mo.None[[]T](), loadErr
			}
			if len(values) == 0 {
				return mo.None[[]T](), nil
			}
			return mo.Some(values), nil
		})
	if err != nil {
		return nil, err
	}
	// 空结果统一返回空切片而非 nil：响应体里 "[]" 比 "null" 对前端安全（后者要求空值兜底）。
	return opt.OrElse([]T{}), nil
}

// cachedSliceByKeyword 联想类查询（带 keyword 过滤）的 cache-aside 入口
//
// keyword 由用户逐字符输入：每个片段都生成独立缓存键、几乎不复用，缓存只会多付一次
// SET 并把 Redis 键空间按「scope × 输入片段 × 分钟」放大。因此 keyword 非空时直接回源，
// 只有「无关键词」这类可复用的全量选项查询才落缓存。
//
//	@param ctx context.Context
//	@param rc *cache.ReadCache 为 nil 时退化为直接回源
//	@param key 缓存键（readCacheKey 产出）
//	@param keyword string 用户输入的关键词；非空时不走缓存
//	@param loader 回源加载器
//	@return []T
//	@return error
func cachedSliceByKeyword[T any](
	ctx context.Context,
	rc *cache.ReadCache,
	key string,
	keyword string,
	loader func(context.Context) ([]T, error),
) ([]T, error) {
	if keyword != "" {
		return loader(ctx)
	}
	return cachedSlice(ctx, rc, key, loader)
}
