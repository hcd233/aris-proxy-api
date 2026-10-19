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
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// oauthCredentialRepository OAuthCredentialRepository 的 GORM 实现
//
// 加解密边界：落库前加密（AES-256-GCM），读出后解密；密钥缺失
// （config.OAuthCredentialKey 为 nil）时读写直接失败（fail-closed）。
type oauthCredentialRepository struct {
	credentialDAO *dao.EndpointOAuthCredentialDAO
	db            *gorm.DB
}

// NewOAuthCredentialRepository 构造 OAuth 凭据仓储
func NewOAuthCredentialRepository(db *gorm.DB) llmproxy.OAuthCredentialRepository {
	return &oauthCredentialRepository{credentialDAO: dao.GetEndpointOAuthCredentialDAO(), db: db}
}

// FindByEndpointID 按端点查询凭据；不存在返回 (nil, nil)。
func (r *oauthCredentialRepository) FindByEndpointID(ctx context.Context, endpointID uint) (*aggregate.OAuthCredential, error) {
	record, err := r.credentialDAO.Get(r.db.WithContext(ctx), &dbmodel.EndpointOAuthCredential{EndpointID: endpointID}, constant.OAuthCredentialRepoFieldsFull)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find oauth credential by endpoint id")
	}
	return buildOAuthCredentialAggregate(record)
}

// Save 按 endpoint_id upsert（创建与更新共用同一入口）。
func (r *oauthCredentialRepository) Save(ctx context.Context, cred *aggregate.OAuthCredential) error {
	record, err := buildOAuthCredentialModel(cred)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: constant.FieldOAuthCredentialEndpointID}},
		DoUpdates: clause.AssignmentColumns(constant.OAuthCredentialUpdateColumns),
	}).Create(record).Error
	if err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "save oauth credential")
	}
	return nil
}

// ListActive 列出全部 active 状态凭据（cron 维护：临期刷新与额度探测）。
func (r *oauthCredentialRepository) ListActive(ctx context.Context) ([]*aggregate.OAuthCredential, error) {
	db := r.db.WithContext(ctx).
		Where(constant.WhereOAuthCredentialStatusEquals, enum.OAuthCredentialStatusActive).
		Where(constant.DBConditionDeletedAtZero)
	var records []*dbmodel.EndpointOAuthCredential
	if err := db.Find(&records).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list active oauth credentials")
	}
	creds := make([]*aggregate.OAuthCredential, 0, len(records))
	for _, record := range records {
		cred, err := buildOAuthCredentialAggregate(record)
		if err != nil {
			return nil, err
		}
		creds = append(creds, cred)
	}
	return creds, nil
}

// ListDisplayByEndpointIDs 批量读取一组端点的凭据展示投影（只选展示列，不解密 token）。
//
// 列表场景逐端点 FindByEndpointID 会退化成 N+1 查询 + 逐行 AES 解密两个 token，
// 这里一次 IN 查询取展示列（token 密文列不进入内存）。
func (r *oauthCredentialRepository) ListDisplayByEndpointIDs(ctx context.Context, endpointIDs []uint) (map[uint]vo.OAuthCredentialDisplay, error) {
	displays := make(map[uint]vo.OAuthCredentialDisplay, len(endpointIDs))
	if len(endpointIDs) == 0 {
		return displays, nil
	}
	var records []*dbmodel.EndpointOAuthCredential
	if err := r.db.WithContext(ctx).
		Select(constant.OAuthCredentialRepoFieldsDisplay).
		Where(constant.WhereEndpointIDIn, endpointIDs).
		Where(constant.DBConditionDeletedAtZero).
		Find(&records).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list oauth credential displays")
	}
	for _, record := range records {
		displays[record.EndpointID] = vo.OAuthCredentialDisplay{
			EndpointID:     record.EndpointID,
			Provider:       enum.OAuthProvider(record.Provider),
			AccountEmail:   record.AccountEmail,
			PlanType:       record.PlanType,
			Status:         enum.OAuthCredentialStatus(record.Status),
			LastError:      record.LastError,
			Quota:          toOAuthQuotaSnapshotVO(record.QuotaSnapshot),
			QuotaUpdatedAt: record.QuotaUpdatedAt,
		}
	}
	return displays, nil
}

// UpdateQuota 只更新额度快照与采集时间。
//
// 不走 Save：Save 是按 endpoint_id 的整行 upsert（含 token/状态列），维护任务里在
// 刷新之后调用会把内存快照中的旧 token 写回，覆盖刚轮换出的新 token。
func (r *oauthCredentialRepository) UpdateQuota(ctx context.Context, endpointID uint, snapshot vo.QuotaSnapshot, at time.Time) error {
	record := &dbmodel.EndpointOAuthCredential{
		QuotaSnapshot:  toOAuthQuotaSnapshotModel(snapshot),
		QuotaUpdatedAt: at,
	}
	err := r.db.WithContext(ctx).
		Model(record).
		Where(constant.WhereEndpointIDEquals, endpointID).
		Where(constant.DBConditionDeletedAtZero).
		Select(constant.FieldOAuthCredentialQuotaSnapshot, constant.FieldOAuthCredentialQuotaUpdatedAt, constant.FieldUpdatedAt).
		Updates(record).Error
	if err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update oauth credential quota")
	}
	return nil
}

// UpdateLastError 只更新最近错误摘要（不清空额度快照，也不改凭据状态）。
func (r *oauthCredentialRepository) UpdateLastError(ctx context.Context, endpointID uint, lastError string) error {
	record := &dbmodel.EndpointOAuthCredential{LastError: lastError}
	err := r.db.WithContext(ctx).
		Model(record).
		Where(constant.WhereEndpointIDEquals, endpointID).
		Where(constant.DBConditionDeletedAtZero).
		Select(constant.FieldOAuthCredentialLastError, constant.FieldUpdatedAt).
		Updates(record).Error
	if err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "update oauth credential last error")
	}
	return nil
}

// buildOAuthCredentialModel 聚合 → DB 模型（token 加密）
func buildOAuthCredentialModel(cred *aggregate.OAuthCredential) (*dbmodel.EndpointOAuthCredential, error) {
	key := config.OAuthCredentialKey
	if len(key) == 0 {
		return nil, ierr.New(ierr.ErrInternal, "oauth credential key is not configured")
	}
	accessTokenEnc, err := commonutil.EncryptSecret(key, cred.AccessToken())
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "encrypt access token")
	}
	refreshTokenEnc, err := commonutil.EncryptSecret(key, cred.RefreshToken())
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "encrypt refresh token")
	}
	return &dbmodel.EndpointOAuthCredential{
		ID:              cred.AggregateID(),
		EndpointID:      cred.EndpointID(),
		Provider:        string(cred.Provider()),
		AccessTokenEnc:  accessTokenEnc,
		RefreshTokenEnc: refreshTokenEnc,
		ExpiresAt:       cred.ExpiresAt(),
		AccountRef:      cred.AccountRef(),
		AccountEmail:    cred.AccountEmail(),
		PlanType:        cred.PlanType(),
		Status:          string(cred.Status()),
		LastError:       cred.LastError(),
		QuotaSnapshot:   toOAuthQuotaSnapshotModel(cred.Quota()),
		QuotaUpdatedAt:  cred.QuotaUpdatedAt(),
	}, nil
}

// buildOAuthCredentialAggregate DB 模型 → 聚合（token 解密）
func buildOAuthCredentialAggregate(record *dbmodel.EndpointOAuthCredential) (*aggregate.OAuthCredential, error) {
	key := config.OAuthCredentialKey
	if len(key) == 0 {
		return nil, ierr.New(ierr.ErrInternal, "oauth credential key is not configured")
	}
	accessToken, err := commonutil.DecryptSecret(key, record.AccessTokenEnc)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "decrypt access token")
	}
	refreshToken, err := commonutil.DecryptSecret(key, record.RefreshTokenEnc)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrInternal, err, "decrypt refresh token")
	}
	cred := aggregate.RehydrateOAuthCredential(
		record.ID,
		record.EndpointID,
		enum.OAuthProvider(record.Provider),
		accessToken,
		refreshToken,
		record.ExpiresAt,
		record.AccountRef,
		record.AccountEmail,
		record.PlanType,
		enum.OAuthCredentialStatus(record.Status),
		record.LastError,
		toOAuthQuotaSnapshotVO(record.QuotaSnapshot),
		record.QuotaUpdatedAt,
	)
	cred.SetTimestamps(record.CreatedAt, record.UpdatedAt)
	return cred, nil
}

// toOAuthQuotaSnapshotModel 领域快照 → DB 形态
func toOAuthQuotaSnapshotModel(snapshot vo.QuotaSnapshot) dbmodel.OAuthQuotaSnapshot {
	return dbmodel.OAuthQuotaSnapshot{
		Windows: lo.Map(snapshot.Windows, func(w vo.QuotaWindow, _ int) dbmodel.OAuthQuotaWindow {
			return dbmodel.OAuthQuotaWindow{
				Key:           w.Key,
				UsedPercent:   w.UsedPercent,
				ResetsAt:      w.ResetsAt,
				WindowMinutes: w.WindowMinutes,
			}
		}),
		PlanType:   snapshot.PlanType,
		CapturedAt: snapshot.CapturedAt,
	}
}

// toOAuthQuotaSnapshotVO DB 形态 → 领域快照
func toOAuthQuotaSnapshotVO(snapshot dbmodel.OAuthQuotaSnapshot) vo.QuotaSnapshot {
	return vo.QuotaSnapshot{
		Windows: lo.Map(snapshot.Windows, func(w dbmodel.OAuthQuotaWindow, _ int) vo.QuotaWindow {
			return vo.QuotaWindow{
				Key:           w.Key,
				UsedPercent:   w.UsedPercent,
				ResetsAt:      w.ResetsAt,
				WindowMinutes: w.WindowMinutes,
			}
		}),
		PlanType:   snapshot.PlanType,
		CapturedAt: snapshot.CapturedAt,
	}
}
