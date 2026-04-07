// Package service API Key 服务
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"

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

const (
	// APIKeyMaxCount 单用户最大 API Key 数量
	APIKeyMaxCount = 5
	// APIKeyPrefix API Key 前缀
	APIKeyPrefix = "sk-aris-"
	// APIKeyRandomLength API Key 随机字符串长度
	APIKeyRandomLength = 24
	// APIKeyCharset API Key 字符集
	APIKeyCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// APIKeyService API Key 服务
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
type APIKeyService interface {
	CreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.CreateAPIKeyRsp, error)
	ListAPIKeys(ctx context.Context) (*dto.ListAPIKeyRsp, error)
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

// generateAPIKey 生成随机 API Key
//
//	@return string
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func generateAPIKey() (string, error) {
	result := make([]byte, APIKeyRandomLength)
	charsetLen := len(APIKeyCharset)
	if _, err := rand.Read(result); err != nil {
		return "", err
	}
	for i := range result {
		result[i] = APIKeyCharset[int(result[i])%charsetLen]
	}
	return APIKeyPrefix + strings.ToLower(string(result)), nil
}

// CreateAPIKey 创建 API Key
//
//	@receiver s *apikeyService
//	@param ctx context.Context
//	@param req *dto.CreateAPIKeyReq
//	@return *dto.CreateAPIKeyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (s *apikeyService) CreateAPIKey(ctx context.Context, req *dto.CreateAPIKeyReq) (*dto.CreateAPIKeyRsp, error) {
	rsp := &dto.CreateAPIKeyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	// 检查用户是否存在
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

	// 检查用户已创建的 key 数量
	existingKeys, err := s.proxyAPIKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{UserID: userID}, []string{"id"})
	if err != nil {
		log.Error("[APIKeyService] Failed to batch get API keys when creating", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}
	if len(existingKeys) >= APIKeyMaxCount {
		log.Warn("[APIKeyService] API key count exceeds limit",
			zap.Uint("userID", userID),
			zap.Int("existingCount", len(existingKeys)),
			zap.Int("maxCount", APIKeyMaxCount))
		rsp.Error = ierr.ErrQuotaExceeded.BizError()
		return rsp, nil
	}

	// 生成 API Key
	apiKey, err := generateAPIKey()
	if err != nil {
		log.Error("[APIKeyService] Failed to generate API key", zap.Error(err))
		rsp.Error = ierr.ErrInternal.BizError()
		return rsp, nil
	}

	// 创建记录
	proxyAPIKey := &dbmodel.ProxyAPIKey{
		UserID: userID,
		Name:   req.Body.Name,
		Key:    apiKey,
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
		zap.String("keyID", util.MaskSecret(proxyAPIKey.Key)))

	return rsp, nil
}

// ListAPIKeys 列出 API Keys
//
//	@receiver s *apikeyService
//	@param ctx context.Context
//	@return *dto.ListAPIKeyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (s *apikeyService) ListAPIKeys(ctx context.Context) (*dto.ListAPIKeyRsp, error) {
	rsp := &dto.ListAPIKeyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValueString(ctx, constant.CtxKeyPermission)

	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	var keys []*dbmodel.ProxyAPIKey
	var err error

	if enum.Permission(permission) == enum.PermissionAdmin {
		// admin: 返回所有 key
		keys, err = s.proxyAPIKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{}, []string{"id", "user_id", "name", "key", "created_at"})
	} else {
		// 普通用户: 只返回自己的 key
		keys, err = s.proxyAPIKeyDAO.BatchGet(db, &dbmodel.ProxyAPIKey{UserID: userID}, []string{"id", "user_id", "name", "key", "created_at"})
	}
	if err != nil {
		log.Error("[APIKeyService] Failed to list API keys", zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	rsp.Keys = make([]*dto.APIKeyItem, 0, len(keys))
	for _, key := range keys {
		rsp.Keys = append(rsp.Keys, &dto.APIKeyItem{
			ID:        key.ID,
			Name:      key.Name,
			Key:       util.MaskSecret(key.Key),
			CreatedAt: key.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	log.Info("[APIKeyService] List API keys",
		zap.Uint("userID", userID),
		zap.Bool("isAdmin", enum.Permission(permission) == enum.PermissionAdmin),
		zap.Int("keyCount", len(rsp.Keys)))

	return rsp, nil
}

// DeleteAPIKey 删除 API Key
//
//	@receiver s *apikeyService
//	@param ctx context.Context
//	@param req *dto.DeleteAPIKeyReq
//	@return *dto.EmptyRsp
//	@return error
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func (s *apikeyService) DeleteAPIKey(ctx context.Context, req *dto.DeleteAPIKeyReq) (*dto.EmptyRsp, error) {
	rsp := &dto.EmptyRsp{}

	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValueString(ctx, constant.CtxKeyPermission)

	log := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	// 查询要删除的 key
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

	// 权限检查: admin 可以删除任意 key, 普通用户只能删除自己的 key
	if enum.Permission(permission) != enum.PermissionAdmin && apiKey.UserID != userID {
		log.Warn("[APIKeyService] No permission to delete API key",
			zap.Uint("keyID", req.ID),
			zap.Uint("keyOwnerID", apiKey.UserID),
			zap.Uint("requesterID", userID))
		rsp.Error = ierr.ErrNoPermission.BizError()
		return rsp, nil
	}

	// 软删除
	if err := s.proxyAPIKeyDAO.Delete(db, &dbmodel.ProxyAPIKey{ID: req.ID}); err != nil {
		log.Error("[APIKeyService] Failed to delete API key", zap.Error(err), zap.Uint("keyID", req.ID))
		rsp.Error = ierr.ErrDBUpdate.BizError()
		return rsp, nil
	}

	log.Info("[APIKeyService] API key deleted",
		zap.Uint("keyID", req.ID),
		zap.Uint("userID", userID),
		zap.String("keyName", apiKey.Name),
		zap.String("keyMasked", util.MaskSecret(apiKey.Key)))

	return rsp, nil
}
