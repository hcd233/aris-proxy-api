// Package demo_access_audit 验证 Demo 登录埋点：成功/被拒均产生审计任务，字段正确。
package demo_access_audit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/command"
	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityaggregate "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ── fakes ──

type fakeConfigRepo struct {
	loginEnabled bool
}

func (f *fakeConfigRepo) Get(context.Context) (*port.DemoConfigEntity, error) {
	return &port.DemoConfigEntity{LoginEnabled: f.loginEnabled}, nil
}

func (f *fakeConfigRepo) Save(context.Context, *port.DemoConfigEntity) error { return nil }

type fakeUserRepo struct {
	identity.UserRepository // 仅 demo login 用例用到 FindByPermission / TouchLastLogin，其余方法不应被调用
	demoUser                *identityaggregate.User
}

func (f *fakeUserRepo) FindByPermission(_ context.Context, permission enum.Permission) (*identityaggregate.User, error) {
	if permission == enum.PermissionDemo && f.demoUser != nil {
		return f.demoUser, nil
	}
	return nil, nil
}

func (f *fakeUserRepo) TouchLastLogin(context.Context, uint) error { return nil }

type fakeSigner struct{}

func (f *fakeSigner) EncodeToken(userID uint) (string, error) {
	return "token-for-user", nil
}

func (f *fakeSigner) DecodeToken(token string) (uint, error) { return 1, nil }

type fakeSubmitter struct {
	tasks []*dto.DemoAccessAuditTask
}

func (f *fakeSubmitter) SubmitDemoAccessAuditTask(task *dto.DemoAccessAuditTask) error {
	f.tasks = append(f.tasks, task)
	return nil
}

// ── helpers ──

var zeroTime = time.Time{}

func newDemoUser() *identityaggregate.User {
	return identityaggregate.RestoreUser(42, "demo", "demo@example.com", "",
		enum.PermissionDemo, zeroTime, zeroTime, "", "")
}

func newHandler(config *fakeConfigRepo, userRepo *fakeUserRepo, submitter port.DemoSubmitter) port.DemoLoginHandler {
	var accessSigner, refreshSigner identityservice.TokenSigner = &fakeSigner{}, &fakeSigner{}
	return command.NewDemoLoginHandler(config, userRepo, accessSigner, refreshSigner, submitter)
}

var testCommand = port.DemoLoginCommand{ClientIP: "1.2.3.4", UserAgent: "test-ua"}

// ── tests ──

func TestDemoLogin_AuditOnSuccess(t *testing.T) {
	t.Parallel()
	submitter := &fakeSubmitter{}
	h := newHandler(&fakeConfigRepo{loginEnabled: true}, &fakeUserRepo{demoUser: newDemoUser()}, submitter)

	result, err := h.Handle(context.Background(), testCommand)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatal("expected tokens in result")
	}
	if len(submitter.tasks) != 1 {
		t.Fatalf("expected 1 audit task, got %d", len(submitter.tasks))
	}
	task := submitter.tasks[0]
	if task.Action != enum.DemoAccessActionLogin {
		t.Errorf("action = %q, want %q", task.Action, enum.DemoAccessActionLogin)
	}
	if task.Reason != "" {
		t.Errorf("reason = %q, want empty", task.Reason)
	}
	if task.Module != "" || task.Path != "" {
		t.Errorf("login audit should not carry module/path, got module=%q path=%q", task.Module, task.Path)
	}
	if task.IP != "1.2.3.4" || task.UserAgent != "test-ua" {
		t.Errorf("ip/ua = %q/%q, want 1.2.3.4/test-ua", task.IP, task.UserAgent)
	}
}

func TestDemoLogin_AuditDeniedWhenDisabled(t *testing.T) {
	t.Parallel()
	submitter := &fakeSubmitter{}
	h := newHandler(&fakeConfigRepo{loginEnabled: false}, &fakeUserRepo{demoUser: newDemoUser()}, submitter)

	_, err := h.Handle(context.Background(), testCommand)
	if err == nil {
		t.Fatal("expected error when login disabled")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("error = %v, want entry disabled", err)
	}
	if len(submitter.tasks) != 1 {
		t.Fatalf("expected 1 audit task, got %d", len(submitter.tasks))
	}
	task := submitter.tasks[0]
	if task.Action != enum.DemoAccessActionLoginDenied {
		t.Errorf("action = %q, want %q", task.Action, enum.DemoAccessActionLoginDenied)
	}
	if task.Reason != constant.DemoAccessReasonLoginDisabled {
		t.Errorf("reason = %q, want %q", task.Reason, constant.DemoAccessReasonLoginDisabled)
	}
	if task.IP != "1.2.3.4" {
		t.Errorf("ip = %q, want 1.2.3.4", task.IP)
	}
}

func TestDemoLogin_AuditDeniedWhenNoDemoUser(t *testing.T) {
	t.Parallel()
	submitter := &fakeSubmitter{}
	h := newHandler(&fakeConfigRepo{loginEnabled: true}, &fakeUserRepo{}, submitter)

	_, err := h.Handle(context.Background(), testCommand)
	if err == nil {
		t.Fatal("expected error when no demo user")
	}
	if len(submitter.tasks) != 1 {
		t.Fatalf("expected 1 audit task, got %d", len(submitter.tasks))
	}
	task := submitter.tasks[0]
	if task.Action != enum.DemoAccessActionLoginDenied {
		t.Errorf("action = %q, want %q", task.Action, enum.DemoAccessActionLoginDenied)
	}
	if task.Reason != constant.DemoAccessReasonNoDemoUser {
		t.Errorf("reason = %q, want %q", task.Reason, constant.DemoAccessReasonNoDemoUser)
	}
}

func TestDemoLogin_NoAuditWhenSubmitterNil(t *testing.T) {
	t.Parallel()
	h := newHandler(&fakeConfigRepo{loginEnabled: true}, &fakeUserRepo{demoUser: newDemoUser()}, nil)

	if _, err := h.Handle(context.Background(), testCommand); err != nil {
		t.Fatalf("Handle returned error with nil submitter: %v", err)
	}
}
