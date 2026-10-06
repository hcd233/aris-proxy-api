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

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Client models.dev 定价查询：拉取公开文档（Redis 缓存 24h），按模型 ID 精确匹配。
// 实现 port.PricingQuoteProvider。
type Client struct {
	Source string // 数据源地址；空 = constant.ModelsDevAPIURL（测试可注入）
	http   *http.Client
	redis  redis.UniversalClient // nil = 不缓存
}

type modelsDevProvider struct {
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	Cost modelsDevCost `json:"cost"`
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

// Quote 按模型 ID 精确匹配公开定价（跨 provider 收集，官方 provider 优先）
//
//	@receiver c *Client
//	@param ctx context.Context
//	@param modelID string 上游模型名（精确匹配，区分大小写）
//	@return port.PricingQuote 分档报价（按 ContextMin 升序，首档 0）
//	@return bool 是否命中
//	@return error 拉取/解析失败
//	@author centonhuang
//	@update 2026-10-07 10:00:00
func (c *Client) Quote(ctx context.Context, modelID string) (port.PricingQuote, bool, error) {
	raw, err := c.doc(ctx)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	var doc map[string]modelsDevProvider
	if err := sonic.Unmarshal(raw, &doc); err != nil {
		return port.PricingQuote{}, false, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "parse models.dev pricing")
	}
	entry, ok := pickProvider(doc, modelID)
	if !ok {
		return port.PricingQuote{}, false, nil
	}
	quote, err := buildQuote(entry.Cost)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	return quote, true, nil
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

// doc 读取定价文档：Redis 缓存优先，未命中则拉取并回填缓存
func (c *Client) doc(ctx context.Context) ([]byte, error) {
	if c.redis != nil {
		if cached, err := c.redis.Get(ctx, constant.ModelsDevDocCacheKey).Bytes(); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, constant.PricingModelsDevMaxDocBytes))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "read models.dev pricing")
	}
	if c.redis != nil {
		//nolint:errcheck // 缓存失败不影响查询
		_ = c.redis.Set(ctx, constant.ModelsDevDocCacheKey, body, constant.PricingModelsDevCacheTTL).Err()
	}
	return body, nil
}

// parseCost 解析单个价格值（string 或 number 形态），空值按 0。
func parseCost(raw sonic.NoCopyRawMessage) (float64, error) {
	s := strings.Trim(string(raw), `"`)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, constant.PricingParseBitSize)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "parse models.dev cost value")
	}
	return f, nil
}
