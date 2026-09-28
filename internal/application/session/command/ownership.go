package command

import (
	"slices"

	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
)

// isOwnedByAny 判断会话是否归属于给定 API Key ID 列表中的任意一个。
//
// 零值语义（空归属 / 认证缺失恒不匹配）完全委托给 Session.IsOwnedByID，
// 此处不重复实现，保证归属判定只有一个真实来源。
func isOwnedByAny(sess *aggregate.Session, ownerIDs []uint) bool {
	return slices.ContainsFunc(ownerIDs, sess.IsOwnedByID)
}
