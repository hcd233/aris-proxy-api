package vo

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// OAuthFlow 订阅账号授权流临时记录（Redis，TTL 见 constant.OAuthFlowTTL）。
//
// 一次性消费（Take 即取即删）防重放；verifier 只在服务端流转，
// 换 token 时才与回调的 code 配对使用，绝不随授权链接下发。
type OAuthFlow struct {
	UserID    uint               `json:"user_id"`
	Provider  enum.OAuthProvider `json:"provider"`
	Verifier  string             `json:"verifier"`
	CreatedAt time.Time          `json:"created_at"`
}
