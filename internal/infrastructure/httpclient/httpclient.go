// Package httpclient 通用 HTTP 客户端模块
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

var client *http.Client

// GetHTTPClient 获取通用 HTTP 客户端单例
//
//	@return *http.Client
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func GetHTTPClient() *http.Client {
	return client
}

// InitHTTPClient 初始化通用 HTTP 客户端
//
// Transport 细粒度超时配置：
//   - DialContext: 连接建立超时 10s
//   - TLSHandshakeTimeout: TLS 握手超时 10s
//   - ResponseHeaderTimeout: 等待响应头超时 30s（仅约束首字节，不影响流式读取）
//   - MaxIdleConns: 全局空闲连接上限 100
//   - MaxIdleConnsPerHost: 每个 host 空闲连接上限 20
//   - IdleConnTimeout: 空闲连接回收时间 90s
//
// Client.Timeout 保持 5min，因为 LLM 流式响应的总传输时长可能很长
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
func InitHTTPClient() {
	client = &http.Client{
		Timeout: constant.HTTPClientTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   constant.HTTPDialTimeout,
				KeepAlive: constant.HTTPKeepAlive,
			}).DialContext,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			TLSHandshakeTimeout:   constant.HTTPTLSHandshakeTimeout,
			ResponseHeaderTimeout: constant.HTTPResponseHeaderTimeout,
			MaxIdleConns:          constant.HTTPMaxIdleConns,
			MaxIdleConnsPerHost:   constant.HTTPMaxIdleConnsPerHost,
			IdleConnTimeout:       constant.HTTPIdleConnTimeout,
			ForceAttemptHTTP2:     true,
		},
	}

	logger.Logger().Info("[HTTPClient] Initialized upstream HTTP client",
		zap.Duration("timeout", constant.HTTPClientTimeout),
		zap.Int("maxIdleConns", constant.HTTPMaxIdleConns),
		zap.Int("maxIdleConnsPerHost", constant.HTTPMaxIdleConnsPerHost),
	)
}
