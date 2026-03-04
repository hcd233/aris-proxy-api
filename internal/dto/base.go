package dto

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

// HTTPResponse HTTP响应
//
//	author centonhuang
//	update 2025-10-31 01:38:26
type HTTPResponse[BodyT any] struct {
	Body BodyT `json:"data"`
}

// RedirectResponse 重定向响应
//
//	@author centonhuang
//	@update 2025-11-02 04:01:39
type RedirectResponse struct {
	Status int    `json:"status" doc:"Status code"`
	Url    string `json:"url" doc:"URL for redirect"`
}

// SSEResponse SSE响应
//
//	@author centonhuang
//	@update 2025-11-08 04:20:42
type SSEResponse struct {
	DataType enum.SSEDataType `json:"dataType" doc:"Data type"`
	Status   enum.SSEStatus   `json:"status" doc:"Status"`
	Data     any              `json:"data" doc:"Data"`
}
