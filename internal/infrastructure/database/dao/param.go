package dao

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

// PageInfo 分页信息
//
//	author centonhuang
//	update 2024-11-01 05:17:51
type PageInfo struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

// PageParam 列表参数
//
//	author centonhuang
//	update 2024-09-21 09:00:57
type PageParam struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// QueryParam 查询参数
//
//	author centonhuang
//	update 2024-09-18 02:56:39
type QueryParam struct {
	Query       string   `json:"query"`
	QueryFields []string `json:"queryFields"`
}

// SortParam 排序参数
//
//	@author centonhuang
//	@update 2025-11-07 01:41:47
type SortParam struct {
	Sort      enum.Sort `json:"sort"`
	SortField string    `json:"sortField"`
}

// CommonParam 分页查询参数
//
//	@author centonhuang
//	@update 2025-08-25 12:30:17
type CommonParam struct {
	PageParam
	QueryParam
	SortParam
}
