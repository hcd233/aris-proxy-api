package repository

import (
	"context"
	"errors"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// userRepository UserRepository 的 GORM 实现
type userRepository struct {
	dao *dao.UserDAO
	db  *gorm.DB
}

// NewUserRepository 构造
//
//	@return identity.UserRepository
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewUserRepository(db *gorm.DB) identity.UserRepository {
	return &userRepository{dao: dao.GetUserDAO(), db: db}
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
	db := r.db.WithContext(ctx)

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
	db := r.db.WithContext(ctx)
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
	db := r.db.WithContext(ctx)
	record, err := r.dao.Get(db, &dbmodel.User{ID: id}, constant.UserRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get user by id")
	}
	return toUserAggregate(record), nil
}

// BatchFindByIDs 批量按 ID 查询用户聚合
//
// 入参去重并过滤 0；未找到的 ID 不出现在返回 map 中。
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param ids []uint
//	@return map[uint]*aggregate.User
//	@return error
//	@author centonhuang
//	@update 2026-08-25 10:00:00
func (r *userRepository) BatchFindByIDs(ctx context.Context, ids []uint) (map[uint]*aggregate.User, error) {
	ids = lo.Uniq(lo.Filter(ids, func(id uint, _ int) bool { return id != 0 }))
	out := make(map[uint]*aggregate.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	db := r.db.WithContext(ctx)
	records, err := r.dao.BatchGetByField(db, constant.FieldID, ids, []string{constant.FieldID, constant.FieldName, constant.FieldAvatar})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch find users by ids")
	}
	for _, record := range records {
		out[record.ID] = toUserAggregate(record)
	}
	return out, nil
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
	db := r.db.WithContext(ctx)
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
	db := r.db.WithContext(ctx)
	record, err := r.dao.Get(db, &dbmodel.User{GoogleBindID: bindID}, constant.UserRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get user by google bind id")
	}
	return toUserAggregate(record), nil
}

// FindByPermission 按权限精确查询（全局单例 Demo 账户定位）
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param permission enum.Permission
//	@return *aggregate.User 未找到返回 nil
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (r *userRepository) FindByPermission(ctx context.Context, permission enum.Permission) (*aggregate.User, error) {
	db := r.db.WithContext(ctx)
	record, err := r.dao.Get(db, &dbmodel.User{Permission: permission}, constant.UserRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get user by permission")
	}
	return toUserAggregate(record), nil
}

// FindByName 按用户名精确查询；未找到返回 (nil, nil)
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param name string 用户名
//	@return *aggregate.User 未找到返回 nil
//	@return error
//	@author centonhuang
//	@update 2026-08-26
func (r *userRepository) FindByName(ctx context.Context, name string) (*aggregate.User, error) {
	db := r.db.WithContext(ctx)
	record, err := r.dao.Get(db, &dbmodel.User{Name: name}, constant.UserRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get user by name")
	}
	return toUserAggregate(record), nil
}

// ReplaceDemoUser 在同一事务内替换全局 Demo 用户。
func (r *userRepository) ReplaceDemoUser(ctx context.Context, targetID uint) (previousDemoID uint, err error) {
	db := r.db.WithContext(ctx)
	err = db.Transaction(func(tx *gorm.DB) error {
		target := &dbmodel.User{}
		if queryErr := tx.Clauses(clause.Locking{Strength: constant.DBLockStrengthUpdate}).
			Where(constant.FieldID+" = ? AND "+constant.DBConditionDeletedAtZero, targetID).
			First(target).Error; queryErr != nil {
			if errors.Is(queryErr, gorm.ErrRecordNotFound) {
				return ierr.New(ierr.ErrDataNotExists, "user not found")
			}
			return ierr.Wrap(ierr.ErrDBQuery, queryErr, "lock demo target user")
		}
		if target.Permission != enum.PermissionPending && target.Permission != enum.PermissionUser {
			return ierr.Newf(ierr.ErrValidation, "user %d is not pending or user", targetID)
		}

		currentDemo := &dbmodel.User{}
		queryErr := tx.Clauses(clause.Locking{Strength: constant.DBLockStrengthUpdate}).
			Where(constant.FieldPermission+" = ? AND "+constant.DBConditionDeletedAtZero, enum.PermissionDemo).
			First(currentDemo).Error
		if queryErr != nil && !errors.Is(queryErr, gorm.ErrRecordNotFound) {
			return ierr.Wrap(ierr.ErrDBQuery, queryErr, "lock current demo user")
		}
		if queryErr == nil {
			previousDemoID = currentDemo.ID
			if updateErr := tx.Model(&dbmodel.User{}).
				Where(constant.FieldID+" = ?", currentDemo.ID).
				Update(constant.FieldPermission, enum.PermissionPending).Error; updateErr != nil {
				return ierr.Wrap(ierr.ErrDBUpdate, updateErr, "demote current demo user")
			}
		}
		if updateErr := tx.Model(&dbmodel.User{}).
			Where(constant.FieldID+" = ?", targetID).
			Update(constant.FieldPermission, enum.PermissionDemo).Error; updateErr != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, updateErr, "promote demo user")
		}
		return nil
	})
	return previousDemoID, err
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

// ListUsers 分页查询用户（管理员视图）
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param param model.CommonParam 分页/搜索参数
//	@param permission enum.Permission 权限过滤，空串=全部
//	@return []*aggregate.User
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-08-07 10:00:00
func (r *userRepository) ListUsers(ctx context.Context, param model.CommonParam, permission enum.Permission) ([]*aggregate.User, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.User{Permission: permission},
		constant.UserRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldName, constant.FieldEmail}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate users")
	}
	return lo.Map(records, func(m *dbmodel.User, _ int) *aggregate.User {
		return toUserAggregate(m)
	}), pageInfo, nil
}

// DeleteCascade 软删除用户及其全部 API Keys（事务保护）
//
//	@receiver r *userRepository
//	@param ctx context.Context
//	@param id uint
//	@return error
//	@author centonhuang
//	@update 2026-08-08 10:00:00
func (r *userRepository) DeleteCascade(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := dao.GetProxyAPIKeyDAO().BatchDeleteByField(tx, constant.FieldUserID, []uint{id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "cascade delete api keys by user id")
		}
		if err := r.dao.Delete(tx, &dbmodel.User{ID: id}); err != nil {
			return ierr.Wrap(ierr.ErrDBDelete, err, "delete user")
		}
		return nil
	})
}
