package command

import (
	"slices"

	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
)

// isOwnedByAny 判断会话是否归属于给定 API Key ID 列表中的任意一个。
//
// 归属为 0（存量未知）的会话对任何列表都返回 false（见 Session.IsOwnedByID），
// 因此空归属会话对普通用户一律不可见，仅 admin 路径（跳过本判定）可达。
//
//	@param sess *aggregate.Session
//	@param ownerIDs []uint 请求方名下的归属 API Key ID 列表
//	@return bool
//	@author centonhuang
//	@update 2026-09-27 10:00:00
func isOwnedByAny(sess *aggregate.Session, ownerIDs []uint) bool {
	if sess == nil {
		return false
	}
	return slices.Contains(ownerIDs, sess.OwnerID().Uint()) && !sess.OwnerID().IsEmpty()
}
