// Package modelsdev models.dev 公开定价数据源客户端
package modelsdev

import (
	"context"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Client models.dev 公开规格查询：拉取公开文档（Redis 缓存原文 24h + 进程内缓存解析结果），
// 按模型 ID 精确匹配。实现 port.ModelSpecProvider。
type Client struct {
	Source string // 数据源地址；空 = constant.ModelsDevAPIURL（测试可注入）
	http   *http.Client
	redis  redis.UniversalClient // nil = 不缓存

	mu       sync.Mutex                   // 串行化加载：并发未命中只回源一次
	catalog  map[string]modelsDevProvider // 解析后的文档：provider → models
	loadedAt time.Time
}

type modelsDevProvider struct {
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	Cost       modelsDevCost       `json:"cost"`
	Limit      modelsDevLimit      `json:"limit"`
	Modalities modelsDevModalities `json:"modalities"`
}

// modelsDevLimit 模型规格上限（tokens）
type modelsDevLimit struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output"`
}

// modelsDevModalities 模态集合（仅 input 对本项目有消费方）
type modelsDevModalities struct {
	Input []string `json:"input"`
}

// modelsDevCost 四类单价（USD/1M tokens）+ 可选上下文分档。
// 值保留原始 JSON：models.dev 中存在 string/number 两种标量形态。
type modelsDevCost struct {
	Input      sonic.NoCopyRawMessage `json:"input"`
	Output     sonic.NoCopyRawMessage `json:"output"`
	CacheRead  sonic.NoCopyRawMessage `json:"cache_read"`
	CacheWrite sonic.NoCopyRawMessage `json:"cache_write"`
	Tiers      []modelsDevTier        `json:"tiers"`
}

// modelsDevTier 上下文分档：tier.size 为该档起始 prompt token（含）
type modelsDevTier struct {
	Input      sonic.NoCopyRawMessage `json:"input"`
	Output     sonic.NoCopyRawMessage `json:"output"`
	CacheRead  sonic.NoCopyRawMessage `json:"cache_read"`
	CacheWrite sonic.NoCopyRawMessage `json:"cache_write"`
	Tier       modelsDevTierBound     `json:"tier"`
}

type modelsDevTierBound struct {
	Type string `json:"type"`
	Size int64  `json:"size"`
}

// NewClient 构造 models.dev 定价客户端
//
//	@param hc *http.Client 通用 HTTP 客户端
//	@param rdb redis.UniversalClient 缓存客户端（nil=不缓存）
//	@return *Client
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func NewClient(hc *http.Client, rdb redis.UniversalClient) *Client {
	return &Client{http: hc, redis: rdb}
}

// Describe 按模型 ID 精确匹配公开规格 + 定价（跨 provider 收集，官方 provider 优先）
//
//	@receiver c *Client
//	@param ctx context.Context
//	@param modelID string 上游模型名（精确匹配，区分大小写）
//	@return port.ModelSpec 规格 + 分档报价（Tiers 按 ContextMin 升序，首档 0）
//	@return bool 是否命中
//	@return error 拉取/解析失败
//	@author centonhuang
//	@update 2026-10-07 16:00:00
func (c *Client) Describe(ctx context.Context, modelID string) (port.ModelSpec, bool, error) {
	doc, err := c.loadCatalog(ctx)
	if err != nil {
		return port.ModelSpec{}, false, err
	}
	entry, ok := pickProvider(doc, modelID)
	if !ok {
		return port.ModelSpec{}, false, nil
	}
	spec := port.ModelSpec{
		ContextLength:   entry.Limit.Context,
		MaxOutputTokens: entry.Limit.Output,
		InputModalities: mapModalities(entry.Modalities.Input),
	}
	if spec.Quote, err = buildQuote(entry.Cost); err != nil {
		return port.ModelSpec{}, false, err
	}
	return spec, true, nil
}

// mapModalities 把 models.dev 输入模态映射为枚举值：未知值静默丢弃，输出按枚举序
func mapModalities(input []string) []enum.InputModality {
	out := make([]enum.InputModality, 0, len(input))
	for _, m := range enum.InputModalities {
		if slices.Contains(input, m) {
			out = append(out, m)
		}
	}
	return out
}

// pickProvider 跨 provider 收集模型命中：官方 provider 优先（列表序靠前者胜），否则 provider 名字典序首个
func pickProvider(doc map[string]modelsDevProvider, modelID string) (modelsDevModel, bool) {
	official := modelsDevModel{}
	officialIdx := len(constant.ModelsDevOfficialProviders)
	officialOK := false
	fallback := modelsDevModel{}
	fallbackName := ""
	fallbackOK := false
	for name, provider := range doc {
		m, ok := provider.Models[modelID]
		if !ok {
			continue
		}
		if idx := slices.Index(constant.ModelsDevOfficialProviders, name); idx >= 0 {
			if !officialOK || idx < officialIdx {
				official, officialIdx, officialOK = m, idx, true
			}
			continue
		}
		if !fallbackOK || name < fallbackName {
			fallback, fallbackName, fallbackOK = m, name, true
		}
	}
	if officialOK {
		return official, true
	}
	return fallback, fallbackOK
}

// buildQuote 平铺四价 + tiers → 分档报价：按档位起点归并（同起点以 tiers 条目为准），
// 输出按 ContextMin 升序，首档必为 0（平铺价兜底）。
func buildQuote(cost modelsDevCost) (port.PricingQuote, error) {
	base := port.PricingTier{}
	if err := fillTier(&base, cost.Input, cost.Output, cost.CacheWrite, cost.CacheRead); err != nil {
		return port.PricingQuote{}, err
	}
	byMin := map[int64]port.PricingTier{0: base}
	for _, t := range cost.Tiers {
		if t.Tier.Type != constant.ModelsDevTierTypeCtx || t.Tier.Size < 0 {
			continue
		}
		tier := port.PricingTier{ContextMin: t.Tier.Size}
		if err := fillTier(&tier, t.Input, t.Output, t.CacheWrite, t.CacheRead); err != nil {
			return port.PricingQuote{}, err
		}
		byMin[tier.ContextMin] = tier // 同起点以 tiers 条目为准
	}
	out := make([]port.PricingTier, 0, len(byMin))
	for _, min := range slices.Sorted(maps.Keys(byMin)) {
		out = append(out, byMin[min])
	}
	return port.PricingQuote{Tiers: out}, nil
}

// fillTier 解析四个价格标量（string/number 兼容）
func fillTier(t *port.PricingTier, input, output, cacheWrite, cacheRead sonic.NoCopyRawMessage) error {
	var err error
	if t.Input, err = parseCost(input); err != nil {
		return err
	}
	if t.Output, err = parseCost(output); err != nil {
		return err
	}
	if t.CacheCreation, err = parseCost(cacheWrite); err != nil {
		return err
	}
	t.CacheRead, err = parseCost(cacheRead)
	return err
}

// loadCatalog 返回解析后的文档：进程内缓存（TTL 内）优先，过期再走 Redis → 回源。
// 文档数 MB，逐请求反序列化在低 CPU 配额下代价可观，故缓存解析结果而非原文。
func (c *Client) loadCatalog(ctx context.Context) (map[string]modelsDevProvider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.catalog != nil && time.Since(c.loadedAt) < constant.PricingModelsDevParsedTTL {
		return c.catalog, nil
	}
	doc, err := c.loadFromSource(ctx)
	if err != nil {
		return nil, err
	}
	c.catalog, c.loadedAt = doc, time.Now()
	return doc, nil
}

// loadFromSource Redis 原文缓存优先（损坏视为未命中）；回源拉取后仅在可完整解析时回填 Redis，
// 防止截断/非 JSON 响应（如代理错误页）被缓存 24h。
func (c *Client) loadFromSource(ctx context.Context) (map[string]modelsDevProvider, error) {
	if c.redis != nil {
		if cached, err := c.redis.Get(ctx, constant.ModelsDevDocCacheKey).Bytes(); err == nil && len(cached) > 0 {
			if doc, perr := parseCatalog(cached); perr == nil {
				return doc, nil
			}
		}
	}
	body, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}
	doc, err := parseCatalog(body)
	if err != nil {
		return nil, err
	}
	if c.redis != nil {
		//nolint:errcheck // 缓存失败不影响查询
		_ = c.redis.Set(ctx, constant.ModelsDevDocCacheKey, body, constant.PricingModelsDevCacheTTL).Err()
	}
	return doc, nil
}

// fetch 回源拉取原文；超过读取上限视为异常响应直接报错（不截断后继续解析）。
func (c *Client) fetch(ctx context.Context) ([]byte, error) {
	url := c.Source
	if url == "" {
		url = constant.ModelsDevAPIURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "build models.dev request")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "fetch models.dev pricing")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // 关闭失败不影响结果
	if resp.StatusCode != http.StatusOK {
		return nil, ierr.New(ierr.ErrInternal, "fetch models.dev pricing failed: "+resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, constant.PricingModelsDevMaxDocBytes+1))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "read models.dev pricing")
	}
	if len(body) > constant.PricingModelsDevMaxDocBytes {
		return nil, ierr.New(ierr.ErrInternal, "models.dev pricing document exceeds size limit")
	}
	return body, nil
}

// parseCatalog 反序列化文档
func parseCatalog(raw []byte) (map[string]modelsDevProvider, error) {
	var doc map[string]modelsDevProvider
	if err := sonic.Unmarshal(raw, &doc); err != nil {
		return nil, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "parse models.dev pricing")
	}
	return doc, nil
}

// parseCost 解析单个价格值（string 或 number 形态），缺省/空串/null（含字符串 "null"）按 0。
func parseCost(raw sonic.NoCopyRawMessage) (float64, error) {
	s := strings.Trim(string(raw), `"`)
	if s == "" || s == constant.NullJSONLiteral {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, constant.PricingParseBitSize)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "parse models.dev cost value")
	}
	return f, nil
}
