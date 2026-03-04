package dto

// PingResponse 健康检查响应
//
//	@author centonhuang
//	@update 2025-11-07 01:36:32
type PingResponse struct {
	CommonRsp
	Status string `json:"status,omitempty" doc:"Status of the ping response"`
}
