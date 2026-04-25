// Package command APIKey 域命令处理器
package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
)

// userExistenceChecker UserExistenceChecker 的实现，通过 identity.UserRepository 查询用户存在性
type userExistenceChecker struct {
	repo identity.UserRepository
}

// NewUserExistenceChecker 构造存在性校验器
//
//	@param repo identity.UserRepository
//	@return UserExistenceChecker
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func NewUserExistenceChecker(repo identity.UserRepository) UserExistenceChecker {
	return &userExistenceChecker{repo: repo}
}

// Exists 判断指定用户是否存在
//
//	@receiver c *userExistenceChecker
//	@param ctx context.Context
//	@param userID uint
//	@return bool
//	@return error
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c *userExistenceChecker) Exists(ctx context.Context, userID uint) (bool, error) {
	user, err := c.repo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}
