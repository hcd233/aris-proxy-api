package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
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
