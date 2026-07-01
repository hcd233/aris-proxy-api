package dto

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// HTTPResponse HTTP响应
//
//	author centonhuang
//	update 2025-10-31 01:38:26
type HTTPResponse[BodyT any] struct {
	Body BodyT `json:"data"`
}

// SSEResponse SSE响应
//
//	@author centonhuang
//	@update 2025-11-08 04:20:42
type SSEResponse struct {
	DataType enum.SSEDataType       `json:"dataType" doc:"Data type"`
	Status   enum.SSEStatus         `json:"status" doc:"Status"`
	Data     sonic.NoCopyRawMessage `json:"data" doc:"Data"`
}
