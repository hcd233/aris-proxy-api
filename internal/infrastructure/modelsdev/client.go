// Package modelsdev models.dev 公开定价数据源客户端
package modelsdev

import (
	"context"
	"io"
	"net/http"
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

// modelsDevEntry models.dev 文档条目（cost 值兼容 string/number 两种形态，故保留原始 JSON）
type modelsDevEntry struct {
	Cost modelsDevCost `json:"cost"`
}

// modelsDevCost 四类单价的原始 JSON 值
type modelsDevCost struct {
	Input      sonic.NoCopyRawMessage `json:"input"`
	Output     sonic.NoCopyRawMessage `json:"output"`
	CacheRead  sonic.NoCopyRawMessage `json:"cache_read"`
	CacheWrite sonic.NoCopyRawMessage `json:"cache_write"`
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

// Quote 按模型 ID 精确匹配公开定价
//
//	@receiver c *Client
//	@param ctx context.Context
//	@param modelID string 上游模型名（精确匹配，区分大小写）
//	@return port.PricingQuote
//	@return bool 是否命中
//	@return error 拉取/解析失败
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (c *Client) Quote(ctx context.Context, modelID string) (port.PricingQuote, bool, error) {
	raw, err := c.doc(ctx)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	var doc map[string]modelsDevEntry
	if err := sonic.Unmarshal(raw, &doc); err != nil {
		return port.PricingQuote{}, false, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "parse models.dev pricing")
	}
	entry, ok := doc[modelID]
	if !ok {
		return port.PricingQuote{}, false, nil
	}
	input, err := parseCost(entry.Cost.Input)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	output, err := parseCost(entry.Cost.Output)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	cacheCreation, err := parseCost(entry.Cost.CacheWrite)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	cacheRead, err := parseCost(entry.Cost.CacheRead)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	return port.PricingQuote{
		Input:         input,
		Output:        output,
		CacheCreation: cacheCreation,
		CacheRead:     cacheRead,
	}, true, nil
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
