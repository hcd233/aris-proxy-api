package model

import "time"

// EndpointOAuthCredential 订阅账号 OAuth 凭据数据库模型
//
// access/refresh token 以 AES-256-GCM 密文存储（加解密在 repository 层完成）；
// 与 endpoint 一对一（endpoint_id 唯一索引），端点删除时级联清理。
type EndpointOAuthCredential struct {
	BaseModel
	ID              uint               `json:"id" gorm:"column:id;primary_key;auto_increment;comment:凭据ID"`
	EndpointID      uint               `json:"endpoint_id" gorm:"column:endpoint_id;not null;uniqueIndex:idx_oauth_credential_endpoint;comment:逻辑外键→endpoints.id"`
	Provider        string             `json:"provider" gorm:"column:provider;not null;comment:OAuth提供方(anthropic/openai)"`
	AccessTokenEnc  string             `json:"access_token_enc" gorm:"column:access_token_enc;not null;comment:access token(AES-256-GCM密文)"`
	RefreshTokenEnc string             `json:"refresh_token_enc" gorm:"column:refresh_token_enc;not null;comment:refresh token(AES-256-GCM密文)"`
	ExpiresAt       time.Time          `json:"expires_at" gorm:"column:expires_at;not null;comment:access token到期时间"`
	AccountRef      string             `json:"account_ref" gorm:"column:account_ref;not null;default:'';comment:账号标识(Claude organization uuid/Codex chatgpt account id)"`
	AccountEmail    string             `json:"account_email" gorm:"column:account_email;not null;default:'';comment:账号邮箱(展示用)"`
	PlanType        string             `json:"plan_type" gorm:"column:plan_type;not null;default:'';comment:订阅类型(展示用)"`
	Status          string             `json:"status" gorm:"column:status;not null;default:'active';comment:凭据状态(active/reauth_required)"`
	LastError       string             `json:"last_error" gorm:"column:last_error;not null;default:'';comment:最近错误(脱敏展示)"`
	QuotaSnapshot   OAuthQuotaSnapshot `json:"quota_snapshot" gorm:"column:quota_snapshot;not null;default:'{}';serializer:json;comment:额度快照JSON"`
	QuotaUpdatedAt  time.Time          `json:"quota_updated_at" gorm:"column:quota_updated_at;comment:额度快照时间"`
	DeletedAt       int64              `json:"deleted_at" gorm:"column:deleted_at;default:0;comment:删除时间"`
}

// OAuthQuotaSnapshot 额度快照 DB 形态（与 domain vo.QuotaSnapshot 字段对齐）
type OAuthQuotaSnapshot struct {
	Windows    []OAuthQuotaWindow `json:"windows"`
	PlanType   string             `json:"plan_type"`
	CapturedAt time.Time          `json:"captured_at"`
}

// OAuthQuotaWindow 额度窗口 DB 形态
type OAuthQuotaWindow struct {
	Key           string    `json:"key"`
	UsedPercent   float64   `json:"used_percent"`
	ResetsAt      time.Time `json:"resets_at"`
	WindowMinutes int       `json:"window_minutes"`
}
