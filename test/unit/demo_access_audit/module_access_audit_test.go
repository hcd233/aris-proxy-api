// Package demo_access_audit 验证 demo 模块访问审计分类：demo 放行/被拒产生记录，admin/user 不产生。
package demo_access_audit

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func TestClassifyDemoAccess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		permission  enum.Permission
		open        bool
		wantAction  enum.DemoAccessAction
		wantReason  string
		wantAudited bool
	}{
		{"demo allowed", enum.PermissionDemo, true, enum.DemoAccessActionModuleAccess, "", true},
		{"demo denied", enum.PermissionDemo, false, enum.DemoAccessActionModuleDenied, constant.DemoAccessReasonModuleClosed, true},
		{"user not audited", enum.PermissionUser, true, "", "", false},
		{"user denied not audited", enum.PermissionUser, false, "", "", false},
		{"admin not audited", enum.PermissionAdmin, true, "", "", false},
		{"pending not audited", enum.PermissionPending, true, "", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			action, reason, ok := middleware.ClassifyDemoAccess(tc.permission, tc.open)
			if action != tc.wantAction || reason != tc.wantReason || ok != tc.wantAudited {
				t.Fatalf("got (%q,%q,%v), want (%q,%q,%v)", action, reason, ok, tc.wantAction, tc.wantReason, tc.wantAudited)
			}
		})
	}
}
