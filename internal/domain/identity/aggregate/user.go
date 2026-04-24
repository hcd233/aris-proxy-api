// Package aggregate Identity 域聚合根
package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonenum "github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
)

// User 用户聚合根
//
// 封装用户档案（Name/Email/Avatar）、权限和第三方绑定 ID。
// 行为：Register（新用户）、UpdateProfile、ChangePermission、RecordLogin。
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
type User struct {
	aggregate.Base

	name         vo.UserName
	email        vo.Email
	avatar       vo.Avatar
	permission   commonenum.Permission
	lastLogin    time.Time
	githubBindID string
	googleBindID string
	createdAt    time.Time
}

// RegisterUser 创建新用户聚合
//
//	@param name vo.UserName
//	@param email vo.Email
//	@param avatar vo.Avatar
//	@param authProvider string OAuth 平台（github/google）
//	@param bindID string 第三方绑定 ID
//	@return *User
//	@return error
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func RegisterUser(name vo.UserName, email vo.Email, avatar vo.Avatar, authProvider, bindID string) (*User, error) {
	if name.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "user name is empty")
	}
	u := &User{
		name:       name,
		email:      email,
		avatar:     avatar,
		permission: commonenum.PermissionPending,
		lastLogin:  time.Now().UTC(),
		createdAt:  time.Now().UTC(),
	}
	switch authProvider {
	case constant.OAuthProviderGithub:
		u.githubBindID = bindID
	case constant.OAuthProviderGoogle:
		u.googleBindID = bindID
	}
	return u, nil
}

// RestoreUser 从仓储重建聚合
//
//	@param id uint
//	@param name vo.UserName
//	@param email vo.Email
//	@param avatar vo.Avatar
//	@param permission commonenum.Permission
//	@param lastLogin time.Time
//	@param createdAt time.Time
//	@param githubBindID string
//	@param googleBindID string
//	@return *User
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func RestoreUser(id uint, name vo.UserName, email vo.Email, avatar vo.Avatar,
	permission commonenum.Permission, lastLogin, createdAt time.Time,
	githubBindID, googleBindID string) *User {
	u := &User{
		name:         name,
		email:        email,
		avatar:       avatar,
		permission:   permission,
		lastLogin:    lastLogin,
		createdAt:    createdAt,
		githubBindID: githubBindID,
		googleBindID: googleBindID,
	}
	u.SetID(id)
	return u
}

// UpdateProfile 更新用户档案
//
// 与 RegisterUser 对齐：Name 不允许为空（返回 ErrValidation）。
//
//	@receiver u *User
//	@param name vo.UserName
//	@param email vo.Email
//	@param avatar vo.Avatar
//	@return error
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func (u *User) UpdateProfile(name vo.UserName, email vo.Email, avatar vo.Avatar) error {
	if name.IsEmpty() {
		return ierr.New(ierr.ErrValidation, "user name is empty")
	}
	u.name = name
	u.email = email
	u.avatar = avatar
	return nil
}

// RecordLogin 记录最新登录时间
//
//	@receiver u *User
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func (u *User) RecordLogin() {
	u.lastLogin = time.Now().UTC()
}

// ChangePermission 变更权限
//
//	@receiver u *User
//	@param newPerm commonenum.Permission
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func (u *User) ChangePermission(newPerm commonenum.Permission) {
	if u.permission == newPerm {
		return
	}
	u.permission = newPerm
}

// AggregateType 实现 aggregate.Root 接口
func (*User) AggregateType() string { return constant.AggregateTypeUser }

// Name 返回用户名
func (u *User) Name() vo.UserName { return u.name }

// Email 返回邮箱
func (u *User) Email() vo.Email { return u.email }

// Avatar 返回头像
func (u *User) Avatar() vo.Avatar { return u.avatar }

// Permission 返回权限
func (u *User) Permission() commonenum.Permission { return u.permission }

// LastLogin 返回最近登录时间
func (u *User) LastLogin() time.Time { return u.lastLogin }

// CreatedAt 返回创建时间
func (u *User) CreatedAt() time.Time { return u.createdAt }

// GithubBindID 返回 Github 绑定 ID
func (u *User) GithubBindID() string { return u.githubBindID }

// GoogleBindID 返回 Google 绑定 ID
func (u *User) GoogleBindID() string { return u.googleBindID }
