// Package api 提供 aris 客户端访问服务端控制面端点的 HTTP 客户端。
package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// Client 控制面客户端（health / client check）
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New 构造控制面客户端；hc 为 nil 或超时为 0 时使用默认超时
func New(baseURL, apiKey string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: constant.ArisClientHTTPTimeout}
	} else if hc.Timeout == 0 {
		clone := *hc
		clone.Timeout = constant.ArisClientHTTPTimeout
		hc = &clone
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: hc}
}

// CheckHealth 请求 GET {base}/health，返回 RTT；非 2xx 视为失败
func (c *Client) CheckHealth(ctx context.Context) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+constant.RoutePathHealth, http.NoBody)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrBadRequest, err, "create health check request")
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrProxySend, err, "send health check request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, ierr.Newf(ierr.ErrBadRequest, "health check rejected with status %d", resp.StatusCode)
	}
	return time.Since(start), nil
}

// clientModelsList 客户端模型分发接口响应信封（仅客户端内部消费）
type clientModelsList struct {
	Models []ClientModel `json:"models"`
}

// ClientModel 服务端返回的启用模型条目
type ClientModel struct {
	Alias           string   `json:"alias"`
	UpstreamModel   string   `json:"upstreamModel"`
	ContextLength   int      `json:"contextLength"`
	MaxOutputTokens int      `json:"maxOutputTokens"`
	Capabilities    []string `json:"capabilities"`
}

// ListModels 拉取服务端启用模型列表（GET /api/cli/v1/model/list，API Key 鉴权）
func (c *Client) ListModels(ctx context.Context) ([]ClientModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+constant.ClientModelsListPath, http.NoBody)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrBadRequest, err, "create list models request")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxySend, err, "send list models request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, ierr.Newf(ierr.ErrBadRequest, "list models rejected with status %d", resp.StatusCode)
	}
	var rsp clientModelsList
	data, err := io.ReadAll(io.LimitReader(resp.Body, constant.ClientModelsListMaxBodyBytes))
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrProxySend, err, "read list models response")
	}
	if err := sonic.Unmarshal(data, &rsp); err != nil {
		return nil, ierr.Wrap(ierr.ErrBadRequest, err, "decode list models response")
	}
	return rsp.Models, nil
}

// CheckAPIKey 请求 GET {base}/api/cli/v1/aris/client/check 校验 API Key；2xx 视为有效
func (c *Client) CheckAPIKey(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+constant.ArisClientCheckPath, http.NoBody)
	if err != nil {
		return ierr.Wrap(ierr.ErrBadRequest, err, "create API key check request")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return ierr.Wrap(ierr.ErrProxySend, err, "send API key check request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ierr.Newf(ierr.ErrBadRequest, "API key check rejected with status %d", resp.StatusCode)
	}
	return nil
}
