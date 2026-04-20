package service

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// auditTaskSpec 封装一次代理调用的审计上下文与度量数据，用于消除各转发路径中
// `ModelCallAuditTask{...}` 字面量的重复。
//
// 字段含义：
//
//   - Endpoint：命中的端点（承载 ModelID 与上游 Provider）
//
//   - APIProvider：对外暴露的接口协议（enum.ProviderOpenAI / enum.ProviderAnthropic）
//
//   - ExposedModel：对外暴露的模型别名（审计表存 alias，而非上游实际模型名）
//
//   - FirstTokenLatencyMs / StreamDurationMs：度量时间；非流式可以只填 FirstTokenLatencyMs=总耗时
//
//   - Err：上游错误（nil 表示成功），用于通过 util.ExtractUpstreamStatusAndError 推导状态码
//
//     @author centonhuang
//     @update 2026-04-20 11:00:00
type auditTaskSpec struct {
	Endpoint            *dbmodel.ModelEndpoint
	APIProvider         string
	ExposedModel        string
	FirstTokenLatencyMs int64
	StreamDurationMs    int64
	Err                 error
}
