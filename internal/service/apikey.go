// Package service API Key 服务
package service

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// APIKeyService API Key 服务
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type APIKeyService interface {
	CreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.CreateAPIKeyRsp, error)
	ListAPIKeys(ctx context.Context) (*dto.ListAPIKeysRsp, error)
	DeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.EmptyRsp, error)
}

type apikeyService struct {
	proxyAPIKeyDAO *dao.ProxyAPIKeyDAO
	userDAO        *dao.UserDAO
}

// NewAPIKeyService 创建 API Key 服务
//
//	@return APIKeyService
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func NewAPIKeyService() APIKeyService {
	return &apikeyService{
		proxyAPIKeyDAO: dao.GetProxyAPIKeyDAO(),
		userDAO:        dao.GetUserDAO(),
	}
}

// byteMax byte 类型的取值范围上限（256）
const byteMax = 256

// generateAPIKey 生成随机 API Key，使用 rejection sampling 避免字节分布偏差
//
//	@return string
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func generateAPIKey() (string, error) {
	charsetLen := byte(len(constant.APIKeyCharset))
	// rejection sampling: 只保留 [0, byteMax - byteMax%charsetLen) 范围内的字节，避免分布偏差
	maxAccepted := byte(byteMax - byteMax%int(charsetLen))
	result := make([]byte, constant.APIKeyRandomLength)
	buf := make([]byte, constant.APIKeyRandomLength*2)
	filled := 0
	for filled < constant.APIKeyRandomLength {
		if _, err := rand.Read(buf); err != nil {
			return "", ierr.Wrap(ierr.ErrInternal, err, "generate random bytes for API key")
		}
		for _, b := range buf {
			if filled >= constant.APIKeyRandomLength {
				break
			}
			if b < maxAccepted {
				result[filled] = constant.APIKeyCharset[int(b)%int(charsetLen)]
				filled++
			}
		}
	}
	return constant.APIKeyPrefix + string(result), nil
}

// countAPIKeys 统计用户已有的 API Key 数量（含历史 user_id=0 的 key）
//
//	@receiver s *apikeyService
//	@param db *gorm.DB
//	@param userID uint
//	@return int64
//	@return error
func (s *apikeyService) countAPIKeys(db *gorm.DB, userID uint) (int64, error) {
	return s.proxyAPIKeyDAO.Count(db, &dbmodel.ProxyAPIKey{UserID: userID})
}

// CreateAPIKey 创建 API Key
//
//	@receiver s *apikeyService
//	@param ctx context.Context
//	@param req *dto.CreateAPIKeyReq
//	@return *dto.CreateAPIKeyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (s *apikeyService) CreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.CreateAPIKeyRsp, error) {
	rsp := &dto.CreateAPIKeyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	user, err := s.userDAO.Get(db, &dbmodel.User{ID: userID}, []string{"id", "name"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("[APIKeyService] User not found when creating API key", zap.Uint("userID", userID))
			rsp.Error = ierr.ErrDataNotExists.BizError()
			return rsp, nil
		}
		log.Error("[APIKeyService] Failed to get user when creating API key", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	if err := s.checkAPIKeyQuota(ctx, db, userID, log); err != nil {
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return rsp, nil
	}

	key, err := generateAPIKey()
	if err != nil {
		log.Error("[APIKeyService] Failed to generate API key", zap.Error(err))
		rsp.Error = ierr.ErrInternal.BizError()
		return rsp, nil
	}

	proxyAPIKey := &dbmodel.ProxyAPIKey{
		UserID: userID,
		Name:   req.Body.Name,
		Key:    key,
	}
	if err := s.proxyAPIKeyDAO.Create(db, proxyAPIKey); err != nil {
		log.Error("[APIKeyService] Failed to create API key", zap.Error(err), zap.String("name", req.Body.Name))
		rsp.Error = ierr.ErrDBCreate.BizError()
		return rsp, nil
	}

	rsp.Key = &dto.APIKeyDetail{
		ID:        proxyAPIKey.ID,
		Name:      proxyAPIKey.Name,
		Key:       proxyAPIKey.Key,
		CreatedAt: proxyAPIKey.CreatedAt,
	}

	log.Info("[APIKeyService] API key created",
		zap.Uint("userID", userID),
		zap.String("userName", user.Name),
		zap.String("keyName", req.Body.Name),
		zap.String("key", util.MaskSecret(proxyAPIKey.Key)))

	return rsp, nil
}

// checkAPIKeyQuota 检查用户是否超出 API Key 数量配额
//
//	@receiver s *apikeyService
//	@param ctx context.Context
//	@param db *gorm.DB
//	@param userID uint
//	@param log *zap.Logger
//	@return error
func (s *apikeyService) checkAPIKeyQuota(ctx context.Context, db *gorm.DB, userID uint, log *zap.Logger) error {
	count, err := s.countAPIKeys(db, userID)
	if err != nil {
		log.Error("[APIKeyService] Failed to count API keys", zap.Error(err), zap.Uint("userID", userID))
		return ierr.Wrap(ierr.ErrDBQuery, err, "count api keys for quota check")
	}
	if count >= constant.APIKeyMaxCount {
		log.Warn("[APIKeyService] API key count exceeds limit",
			zap.Uint("userID", userID),
			zap.Int64("existingCount", count),
			zap.Int("maxCount", constant.APIKeyMaxCount))
		return ierr.New(ierr.ErrQuotaExceeded, "api key count exceeds limit")
	}
	return nil
}

// ListAPIKeys 列出 API Keys
//
//	@receiver s *apikeyService
//	@param ctx context.Context
//	@return *dto.ListAPIKeysRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (s *apikeyService) ListAPIKeys(ctx context.Context) (*dto.ListAPIKeysRsp, error) {
	rsp := &dto.ListAPIKeysRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	keys, err := s.queryAPIKeys(db, userID, permission)
	if err != nil {
		log.Error("[APIKeyService] Failed to list API keys", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	rsp.Keys = toAPIKeyItems(keys)

	log.Info("[APIKeyService] List API keys",
		zap.Uint("userID", userID),
		zap.Bool("isAdmin", permission == enum.PermissionAdmin),
		zap.Int("keyCount", len(rsp.Keys)))

	return rsp, nil
}

// queryAPIKeys 根据权限查询 API Key 列表
func (s *apikeyService) queryAPIKeys(db *gorm.DB, userID uint, permission enum.Permission) ([]*dbmodel.ProxyAPIKey, error) {
	fields := []string{"id", "user_id", "name", "key", "created_at"}
	if permission == enum.PermissionAdmin {
		return s.proxyAPIKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{}, fields)
	}
	return s.proxyAPIKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{UserID: userID}, fields)
}

// toAPIKeyItems 将数据库模型列表转为 DTO 列表
func toAPIKeyItems(keys []*dbmodel.ProxyAPIKey) []*dto.APIKeyItem {
	items := make([]*dto.APIKeyItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, &dto.APIKeyItem{
			ID:        k.ID,
			Name:      k.Name,
			Key:       util.MaskSecret(k.Key),
			CreatedAt: k.CreatedAt,
		})
	}
	return items
}

// DeleteAPIKey 删除 API Key
//
//	@receiver s *apikeyService
//	@param ctx context.Context
//	@param req *dto.DeleteAPIKeyReq
//	@return *dto.EmptyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (s *apikeyService) DeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.EmptyRsp, error) {
	rsp := &dto.EmptyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	apiKey, err := s.proxyAPIKeyDAO.Get(db, &dbmodel.ProxyAPIKey{ID: req.ID}, []string{"id", "user_id", "name", "key"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("[APIKeyService] API key not found when deleting", zap.Uint("keyID", req.ID))
			rsp.Error = ierr.ErrDataNotExists.BizError()
			return rsp, nil
		}
		log.Error("[APIKeyService] Failed to get API key when deleting", zap.Error(err), zap.Uint("keyID", req.ID))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	// user_id == 0 为历史数据，普通用户可操作自己账号下所有历史 key；admin 可操作任意 key
	isOwner := apiKey.UserID == userID || apiKey.UserID == 0
	if permission != enum.PermissionAdmin && !isOwner {
		log.Warn("[APIKeyService] No permission to delete API key",
			zap.Uint("keyID", req.ID),
			zap.Uint("keyOwnerID", apiKey.UserID),
			zap.Uint("requesterID", userID))
		rsp.Error = ierr.ErrNoPermission.BizError()
		return rsp, nil
	}

	if err := s.proxyAPIKeyDAO.Delete(db, &dbmodel.ProxyAPIKey{ID: req.ID}); err != nil {
		log.Error("[APIKeyService] Failed to delete API key", zap.Error(err), zap.Uint("keyID", req.ID))
		rsp.Error = ierr.ErrDBDelete.BizError()
		return rsp, nil
	}

	log.Info("[APIKeyService] API key deleted",
		zap.Uint("keyID", req.ID),
		zap.Uint("userID", userID),
		zap.String("keyName", apiKey.Name),
		zap.String("keyMasked", util.MaskSecret(apiKey.Key)))

	return rsp, nil
}
