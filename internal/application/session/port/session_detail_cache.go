// Package port defines application-layer ports for session use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

// SessionMetaCacheRecord is the cache payload for session metadata.
//
// MessageIDs/ToolIDs are internal fields used by paginated detail queries and
// are not exposed directly by API responses.
type SessionMetaCacheRecord struct {
	ID         uint              `json:"id"`
	APIKeyName string            `json:"apiKeyName"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Score      *int              `json:"score"`
	ScoredAt   *time.Time        `json:"scoredAt"`
	MessageIDs []uint            `json:"messageIds"`
	ToolIDs    []uint            `json:"toolIds"`
}

// MessageCacheRecord is the cache payload for one immutable message.
type MessageCacheRecord struct {
	ID        uint               `json:"id"`
	Model     string             `json:"model"`
	Message   *vo.UnifiedMessage `json:"message"`
	CreatedAt time.Time          `json:"createdAt"`
}

// ToolCacheRecord is the cache payload for one immutable tool definition.
type ToolCacheRecord struct {
	ID        uint            `json:"id"`
	Tool      *vo.UnifiedTool `json:"tool"`
	CreatedAt time.Time       `json:"createdAt"`
}

// SessionDetailCache caches session detail metadata and immutable child records.
type SessionDetailCache interface {
	GetSessionMeta(ctx context.Context, sessionID uint) (*SessionMetaCacheRecord, error)
	SetSessionMeta(ctx context.Context, record *SessionMetaCacheRecord) error
	DeleteSessionMeta(ctx context.Context, sessionID uint) error

	GetMessages(ctx context.Context, ids []uint) (hits map[uint]*MessageCacheRecord, missing []uint, err error)
	SetMessages(ctx context.Context, records []*MessageCacheRecord) error

	GetTools(ctx context.Context, ids []uint) (hits map[uint]*ToolCacheRecord, missing []uint, err error)
	SetTools(ctx context.Context, records []*ToolCacheRecord) error
}
