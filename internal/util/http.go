// Package util 工具包
package util

import (
	"io"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// WrapHTTPResponse 包装HTTP响应错误
//
//	@param rsp rspT
//	@param err error
//	@return *dto.HTTPResponse[rspT]
//	@return error
//	@author centonhuang
//	@update 2025-11-11 04:58:31
func WrapHTTPResponse[rspT any](rsp rspT, err error) (*dto.HTTPResponse[rspT], error) {
	return &dto.HTTPResponse[rspT]{
		Body: rsp,
	}, err
}

// WriteErrorResponse 写入错误响应
//
//	@param ctx
//	@param err
//	@return error
//	@author centonhuang
//	@update 2025-11-10 20:55:14
func WriteErrorResponse(bodyWriter io.Writer, err *model.Error) error {
	_, writeErr := bodyWriter.Write(lo.Must1(sonic.Marshal(&dto.CommonRsp{Error: err})))
	return writeErr
}
