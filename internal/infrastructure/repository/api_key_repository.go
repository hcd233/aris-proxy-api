package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// apiKeyRepoFields ProxyAPIKey 查询默认字段集（与原 service 行为一致）
var apiKeyRepoFields = []string{"id", "user_id", "name", "key", "created_at"}

// apiKeyRepository APIKeyRepository 的 GORM 实现
type apiKeyRepository struct {
	dao *dao.ProxyAPIKeyDAO
}

// NewAPIKeyRepository 构造仓储
//
//	@return apikey.APIKeyRepository
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewAPIKeyRepository() apikey.APIKeyRepository {
	return &apiKeyRepository{dao: dao.GetProxyAPIKeyDAO()}
}

// Save 持久化聚合；首次 Save 后回填 ID
//
//	@receiver r *apiKeyRepository
//	@param ctx context.Context
//	@param key *aggregate.ProxyAPIKey
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *apiKeyRepository) Save(ctx context.Context, key *aggregate.ProxyAPIKey) error {
	db := database.GetDBInstance(ctx)

	if key.AggregateID() == 0 {
		record := &dbmodel.ProxyAPIKey{
			UserID: key.UserID(),
			Name:   key.Name().String(),
			Key:    key.Secret().Raw(),
		}
		if err := r.dao.Create(db, record); err != nil {
			return ierr.Wrap(ierr.ErrDBCreate, err, "create api key")
		}
		key.SetID(record.ID)
		return nil
	}

	updates := map[string]any{
		"name": key.Name().String(),
		"key":  key.Secret().Raw(),
	}
	if err := r.dao.Update(db, &dbmodel.ProxyAPIKey{ID: key.AggregateID()}, updates); err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update api key")
	}
	return nil
}

// FindByID 按 ID 查询聚合
//
//	@receiver r *apiKeyRepository
//	@param ctx context.Context
//	@param id uint
//	@return *aggregate.ProxyAPIKey
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *apiKeyRepository) FindByID(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error) {
	db := database.GetDBInstance(ctx)
	record, err := r.dao.Get(db, &dbmodel.ProxyAPIKey{ID: id}, apiKeyRepoFields)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get api key by id")
	}
	return toAPIKeyAggregate(record), nil
}

// ListByUser 查询用户持有的 Key 列表
//
//	@receiver r *apiKeyRepository
//	@param ctx context.Context
//	@param userID uint
//	@return []*aggregate.ProxyAPIKey
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *apiKeyRepository) ListByUser(ctx context.Context, userID uint) ([]*aggregate.ProxyAPIKey, error) {
	db := database.GetDBInstance(ctx)
	records, err := r.dao.BatchGet(db, &dbmodel.ProxyAPIKey{UserID: userID}, apiKeyRepoFields)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list api keys by user")
	}
	return toAPIKeyAggregateList(records), nil
}

// ListAll 查询所有 Key（admin 视图）
//
//	@receiver r *apiKeyRepository
//	@param ctx context.Context
//	@return []*aggregate.ProxyAPIKey
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *apiKeyRepository) ListAll(ctx context.Context) ([]*aggregate.ProxyAPIKey, error) {
	db := database.GetDBInstance(ctx)
	records, err := r.dao.BatchGet(db, &dbmodel.ProxyAPIKey{}, apiKeyRepoFields)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list all api keys")
	}
	return toAPIKeyAggregateList(records), nil
}

// CountByUser 统计用户持有的 Key 总数（含 UserID==0 的历史 key）
//
//	@receiver r *apiKeyRepository
//	@param ctx context.Context
//	@param userID uint
//	@return int64
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *apiKeyRepository) CountByUser(ctx context.Context, userID uint) (int64, error) {
	db := database.GetDBInstance(ctx)
	count, err := r.dao.Count(db, &dbmodel.ProxyAPIKey{UserID: userID})
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDBQuery, err, "count api keys by user")
	}
	return count, nil
}

// Delete 删除 Key（软删除）
//
//	@receiver r *apiKeyRepository
//	@param ctx context.Context
//	@param id uint
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *apiKeyRepository) Delete(ctx context.Context, id uint) error {
	db := database.GetDBInstance(ctx)
	if err := r.dao.Delete(db, &dbmodel.ProxyAPIKey{ID: id}); err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "delete api key")
	}
	return nil
}

// toAPIKeyAggregate 将 GORM 模型映射为聚合根
func toAPIKeyAggregate(m *dbmodel.ProxyAPIKey) *aggregate.ProxyAPIKey {
	return aggregate.RestoreProxyAPIKey(
		m.ID,
		m.UserID,
		vo.APIKeyName(m.Name),
		vo.NewAPIKeySecret(m.Key),
		m.CreatedAt,
	)
}

// toAPIKeyAggregateList 批量映射
func toAPIKeyAggregateList(records []*dbmodel.ProxyAPIKey) []*aggregate.ProxyAPIKey {
	out := make([]*aggregate.ProxyAPIKey, 0, len(records))
	for _, r := range records {
		out = append(out, toAPIKeyAggregate(r))
	}
	return out
}
