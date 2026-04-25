// Package query identity 域只读查询
package query

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/apikey/command"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
)

// userExistenceChecker 实现 application/apikey/command.UserExistenceChecker 接口
type userExistenceChecker struct {
	repo identity.UserRepository
}

// NewUserExistenceChecker 构造存在性校验器
//
//	@param repo identity.UserRepository
//	@return command.UserExistenceChecker
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func NewUserExistenceChecker(repo identity.UserRepository) command.UserExistenceChecker {
	return &userExistenceChecker{repo: repo}
}

// Exists 判断指定用户是否存在
//
//	@receiver c *UserExistenceChecker
//	@param ctx context.Context
//	@param userID uint
//	@return bool
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (c *userExistenceChecker) Exists(ctx context.Context, userID uint) (bool, error) {
	user, err := c.repo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}
