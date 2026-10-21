package enum

import (
	"slices"
)

// EndpointVisibility 端点可见性
//
// private（默认）仅归属者与 admin 可见；public 对全体登录用户只读可见、可调用。
// 只有 admin 可以写为 public；公开端点的归属仍是创建它的 admin。
type EndpointVisibility string

const (
	// EndpointVisibilityPrivate 私有（默认，仅归属者与 admin 可见）
	EndpointVisibilityPrivate EndpointVisibility = "private"

	// EndpointVisibilityPublic 公开（全体登录用户只读可见、可调用）
	EndpointVisibilityPublic EndpointVisibility = "public"
)

// EndpointVisibilities 全部合法可见性。解析/白名单判断必须基于该集合，
// 新增取值时同步补充此处，避免多层口径不一致。
var EndpointVisibilities = []EndpointVisibility{EndpointVisibilityPrivate, EndpointVisibilityPublic}

// IsValid 是否为合法可见性（enum 包禁止依赖 ierr：由调用方据此构造 ErrValidation）
func (v EndpointVisibility) IsValid() bool {
	return slices.Contains(EndpointVisibilities, v)
}

// IsPublic 是否为公开可见性
func (v EndpointVisibility) IsPublic() bool {
	return v == EndpointVisibilityPublic
}

// Normalize 空值归一为 private（调用方漏传时保持默认语义，不产生脏数据）
func (v EndpointVisibility) Normalize() EndpointVisibility {
	if v == "" {
		return EndpointVisibilityPrivate
	}
	return v
}
