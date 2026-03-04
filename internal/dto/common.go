package dto

import "github.com/hcd233/aris-proxy-api/internal/common/model"

// CommonRsp 通用响应
//
//	@author centonhuang
//	@update 2025-11-10 19:29:36
type CommonRsp struct {
	Error *model.Error `json:"error,omitempty" doc:"Error body"`
}

// EmptyReq 空请求
//
//	@author centonhuang
//	@update 2025-10-31 02:32:07
type EmptyReq struct{}

// EmptyRsp 空响应
//
//	author centonhuang
//	update 2025-01-05 15:33:11
type EmptyRsp struct {
	CommonRsp
}
