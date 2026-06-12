// Package command APIKey 域命令处理器
package command

import (
	"context"
	"errors"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
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
	r := c.repo.FindByID(ctx, userID)
	if r.IsOk() {
		return true, nil
	}
	if errors.Is(r.Error(), ierr.ErrDataNotExists) {
		return false, nil
	}
	return false, r.Error()
}
