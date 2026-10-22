package proxyutil

import (
	"errors"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// ModelResolveError 把模型/端点解析失败映射为协议原生 ProxyError。
//
// 只有「确实不存在」（ErrDataNotExists）才返回 404 model_not_found / not_found_error；
// DB 查询失败、Redis 故障等基础设施错误一律返回 503（server_error / overloaded_error），
// 让 SDK 进入重试分支，而不是误导客户端去检查模型配置。
//
// 背景（2026-10-22 压测）：PG 连接打满时 FindByAlias 失败，旧实现把所有解析错误
// 都映射成 404，压测表现为「模型不存在」，掩盖了真实的 DB 不可用。
//
//	@param err 解析链路返回的错误
//	@param protocol 入口协议（决定错误 envelope）
//	@param model 请求中的模型别名（仅 404 body 使用）
//	@return *port.ProxyError
//	@author centonhuang
//	@update 2026-10-23 01:30:00
func ModelResolveError(err error, protocol enum.ProtocolKind, model string) *port.ProxyError {
	if errors.Is(err, ierr.ErrDataNotExists) {
		if protocol == enum.ProtocolKindAnthropic {
			return SendAnthropicModelNotFoundError(model)
		}
		return SendOpenAIModelNotFoundError(model)
	}
	if protocol == enum.ProtocolKindAnthropic {
		return SendAnthropicUnavailableError(err)
	}
	return SendOpenAIUnavailableError(err)
}
