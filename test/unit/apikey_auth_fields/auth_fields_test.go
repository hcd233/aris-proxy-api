// Package apikey_auth_fields 鉴权路径字段投影回归测试。
//
// 缺陷背景（生产实证）：ProxyAPIKeyRepoFieldsAuth 缺 FieldName 时，
// dao.Get 的 SELECT 不含该列 → apiKey.Name 恒为空串 →
// APIKeyMiddleware 注入空 CtxKeyAPIKeyName → sessions.api_key_name 写空。
// 归属过滤是 api_key_name IN (ownerNames)，空串永不匹配任何 key 名，
// 故 2026-07 起产生的 2623+ 条会话对所有普通用户彻底不可见（仅 admin 可见）。
//
// 对照：UserRepoFieldsAuth 含 FieldName，所以 CtxKeyUserName 一直正常；
// model_call_audits 的 api_key_id 零行为 0，因为 FieldID 在白名单内。
//
//	@author centonhuang
//	@update 2026-09-27
package apikey_auth_fields

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TestProxyAPIKeyAuthFieldsIncludeName 鉴权投影必须含 name 列。
func TestProxyAPIKeyAuthFieldsIncludeName(t *testing.T) {
	t.Parallel()
	if !slices.Contains(constant.ProxyAPIKeyRepoFieldsAuth, constant.FieldName) {
		t.Errorf("ProxyAPIKeyRepoFieldsAuth must contain %q, got %v",
			constant.FieldName, constant.ProxyAPIKeyRepoFieldsAuth)
	}
}

// TestProxyAPIKeyAuthFieldsIncludeOwnership 鉴权投影必须含 id 与 user_id。
//
// id 用于注入 CtxKeyAPIKeyID（归属判定的唯一权威依据），
// user_id 用于孤儿 key 防御（user_id=0 显式拒绝）与用户查询。
func TestProxyAPIKeyAuthFieldsIncludeOwnership(t *testing.T) {
	t.Parallel()
	for _, field := range []string{constant.FieldID, constant.FieldUserID} {
		if !slices.Contains(constant.ProxyAPIKeyRepoFieldsAuth, field) {
			t.Errorf("ProxyAPIKeyRepoFieldsAuth must contain %q, got %v",
				field, constant.ProxyAPIKeyRepoFieldsAuth)
		}
	}
}
