// Package port defines application-layer ports for apikey use cases.
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// IssueAPIKeyCommand 签发新 API Key 命令
type IssueAPIKeyCommand struct {
	UserID uint
	Name   string
}

// IssueAPIKeyResult 签发命令结果
type IssueAPIKeyResult struct {
	KeyID     uint
	Name      string
	Secret    string
	CreatedAt time.Time
}

// IssueAPIKeyHandler 签发命令处理器
type IssueAPIKeyHandler interface {
	Handle(ctx context.Context, cmd IssueAPIKeyCommand) (*IssueAPIKeyResult, error)
}

// RevokeAPIKeyCommand 吊销 API Key 命令
type RevokeAPIKeyCommand struct {
	KeyID               uint
	RequesterID         uint
	RequesterPermission enum.Permission
}

// RevokeAPIKeyHandler 吊销命令处理器
type RevokeAPIKeyHandler interface {
	Handle(ctx context.Context, cmd RevokeAPIKeyCommand) error
}

// APIKeyView 只读 API Key 投影（列表响应）
type APIKeyView struct {
	ID        uint
	Name      string
	MaskedKey string
	CreatedAt time.Time
}

// ListAPIKeysQuery 列出 API Keys 查询命令
type ListAPIKeysQuery struct {
	RequesterID         uint
	RequesterPermission enum.Permission
	model.CommonParam
}

// ListAPIKeysHandler 查询处理器
type ListAPIKeysHandler interface {
	Handle(ctx context.Context, q ListAPIKeysQuery) ([]*APIKeyView, *model.PageInfo, error)
}
