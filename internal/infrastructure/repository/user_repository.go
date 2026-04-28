package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// userRepository UserRepository 的 GORM 实现
type userRepository struct {
	dao *dao.UserDAO
}

// NewUserRepository 构造
//
//	@return identity.UserRepository
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewUserRepository() identity.UserRepository {
	return &userRepository{dao: dao.GetUserDAO()}
}

// Save 持久化聚合；首次 Save 后回填 ID
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param user *aggregate.User
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *userRepository) Save(ctx context.Context, user *aggregate.User) error {
	db := database.GetDBInstance(ctx)

	if user.AggregateID() == 0 {
		record := &dbmodel.User{
			Name:         user.Name().String(),
			Email:        user.Email().String(),
			Avatar:       user.Avatar().String(),
			Permission:   user.Permission(),
			LastLogin:    user.LastLogin(),
			GithubBindID: user.GithubBindID(),
			GoogleBindID: user.GoogleBindID(),
		}
		if err := r.dao.Create(db, record); err != nil {
			return ierr.Wrap(ierr.ErrDBCreate, err, "create user")
		}
		user.SetID(record.ID)
		return nil
	}

	updates := map[string]any{
		constant.FieldName:       user.Name().String(),
		constant.FieldEmail:      user.Email().String(),
		constant.FieldAvatar:     user.Avatar().String(),
		constant.FieldPermission: user.Permission(),
		constant.FieldLastLogin:  user.LastLogin(),
	}
	if err := r.dao.Update(db, &dbmodel.User{ID: user.AggregateID()}, updates); err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update user")
	}
	return nil
}

// TouchLastLogin 仅更新 last_login 字段
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param userID uint
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:00:00
func (r *userRepository) TouchLastLogin(ctx context.Context, userID uint) error {
	db := database.GetDBInstance(ctx)
	if err := r.dao.Update(db, &dbmodel.User{ID: userID}, map[string]any{
		constant.FieldLastLogin: time.Now().UTC(),
	}); err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "touch last login")
	}
	return nil
}

// FindByID 按 ID 查询用户聚合
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param id uint
//	@return *aggregate.User 未找到返回 nil
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *userRepository) FindByID(ctx context.Context, id uint) (*aggregate.User, error) {
	db := database.GetDBInstance(ctx)
	record, err := r.dao.Get(db, &dbmodel.User{ID: id}, constant.UserRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get user by id")
	}
	return toUserAggregate(record), nil
}

// FindByGithubBindID 按 github 绑定 ID 查询
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param bindID string
//	@return *aggregate.User
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *userRepository) FindByGithubBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	db := database.GetDBInstance(ctx)
	record, err := r.dao.Get(db, &dbmodel.User{GithubBindID: bindID}, constant.UserRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get user by github bind id")
	}
	return toUserAggregate(record), nil
}

// FindByGoogleBindID 按 google 绑定 ID 查询
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param bindID string
//	@return *aggregate.User
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *userRepository) FindByGoogleBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	db := database.GetDBInstance(ctx)
	record, err := r.dao.Get(db, &dbmodel.User{GoogleBindID: bindID}, constant.UserRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get user by google bind id")
	}
	return toUserAggregate(record), nil
}

// toUserAggregate 将 GORM 模型映射为聚合根
func toUserAggregate(m *dbmodel.User) *aggregate.User {
	return aggregate.RestoreUser(
		m.ID,
		vo.UserName(m.Name),
		vo.Email(m.Email),
		vo.Avatar(m.Avatar),
		m.Permission,
		m.LastLogin,
		m.CreatedAt,
		m.GithubBindID,
		m.GoogleBindID,
	)
}
