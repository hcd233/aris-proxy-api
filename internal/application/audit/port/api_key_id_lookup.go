// Package port defines application-layer ports for audit use cases.
package port

import "context"

// APIKeyIDLookup 审计查询处理器从用户 ID 解析名下 API Key ID 列表的端口。
//
// 仅暴露审计需要的最小方法集，避免 application 层依赖 apikey 仓储的完整接口。
type APIKeyIDLookup interface {
	LookupIDsByUserID(ctx context.Context, userID uint) ([]uint, error)
}
