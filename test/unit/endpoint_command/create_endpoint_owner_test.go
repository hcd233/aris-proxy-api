// Package endpoint_command 验证 endpoint 创建命令的归属计算与用户隔离。
package endpoint_command

import (
	"context"
	"net"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// 命令层 baseURL 校验会做 DNS 解析；注入固定公网解析结果，
// 禁用真实 DNS（防 NXDOMAIN / CI 无网抖动），SSRF 目标走 IP 字面量与黑名单分支。
func init() {
	util.LookupIPFn = func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.10")}, nil
	}
}

type capturingEndpointRepo struct {
	gotOwner   uint
	findByIDN  int
	createCall int
}

func (f *capturingEndpointRepo) FindByID(context.Context, uint, *uint) (*aggregate.Endpoint, error) {
	f.findByIDN++
	return nil, nil
}
func (f *capturingEndpointRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *capturingEndpointRepo) Create(_ context.Context, _ *aggregate.Endpoint, ownerUserID uint) (uint, error) {
	f.createCall++
	f.gotOwner = ownerUserID
	return 1, nil
}
func (f *capturingEndpointRepo) Update(context.Context, *aggregate.Endpoint) error { return nil }
func (f *capturingEndpointRepo) Delete(context.Context, uint, *uint) error         { return nil }
func (f *capturingEndpointRepo) DeleteCascade(context.Context, uint, *uint) error  { return nil }
func (f *capturingEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *capturingEndpointRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *capturingEndpointRepo) FindIDsByScope(context.Context, *uint) ([]uint, error) {
	return nil, nil
}

var _ llmproxy.EndpointRepository = (*capturingEndpointRepo)(nil)

func TestCreateEndpoint_RejectsZeroOwner(t *testing.T) {
	t.Parallel()
	repo := &capturingEndpointRepo{}
	h := command.NewCreateEndpointHandler(repo)

	cmd := port.CreateEndpointCommand{OwnerUserID: 0, Name: "ep", APIKey: "k", OpenaiBaseURL: "https://o.example.com"}
	cmd.SupportOpenAIChatCompletion = true
	if _, err := h.Handle(t.Context(), cmd); err == nil {
		t.Fatal("zero owner must be rejected")
	}
}

func TestCreateEndpoint_PassesOwnerToRepo(t *testing.T) {
	t.Parallel()
	repo := &capturingEndpointRepo{}
	h := command.NewCreateEndpointHandler(repo)

	cmd := port.CreateEndpointCommand{
		OwnerUserID:                 303,
		Name:                        "ep",
		APIKey:                      "k",
		OpenaiBaseURL:               "https://o.example.com",
		SupportOpenAIChatCompletion: true,
	}
	if _, err := h.Handle(t.Context(), cmd); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if repo.gotOwner != 303 {
		t.Fatalf("repo got owner %d, want 303", repo.gotOwner)
	}
}

// TestCreateEndpoint_RejectsSSRFBaseURL 验证创建命令在应用层拒绝内网/元数据 baseURL，
// 且拒绝路径不触达仓储（两个 baseURL 字段都覆盖）。
//
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func TestCreateEndpoint_RejectsSSRFBaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		openai           string
		anthropic        string
		supportChat      bool
		supportAnthropic bool
	}{
		{"openai points to metadata ip", "http://169.254.169.254", "", true, false},
		{"openai points to loopback", "http://127.0.0.1:8080", "", true, false},
		{"anthropic points to metadata host", "", "https://metadata.google.internal", false, true},
		{"anthropic points to private net", "", "http://10.0.0.5", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &capturingEndpointRepo{}
			h := command.NewCreateEndpointHandler(repo)

			cmd := port.CreateEndpointCommand{
				OwnerUserID:                 303,
				Name:                        "ep",
				APIKey:                      "k",
				OpenaiBaseURL:               tc.openai,
				AnthropicBaseURL:            tc.anthropic,
				SupportOpenAIChatCompletion: tc.supportChat,
				SupportAnthropicMessage:     tc.supportAnthropic,
			}
			if _, err := h.Handle(t.Context(), cmd); err == nil {
				t.Fatalf("ssrf base url must be rejected: openai=%q anthropic=%q", tc.openai, tc.anthropic)
			}
			if repo.createCall != 0 {
				t.Errorf("repo.Create must not be called on rejection, got %d", repo.createCall)
			}
		})
	}
}

// TestUpdateEndpoint_RejectsSSRFBaseURL 验证更新命令在应用层拒绝内网/元数据 baseURL，
// 校验先于仓储查询（不触达 repo，避免报错差异探测端点存在性）。
//
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func TestUpdateEndpoint_RejectsSSRFBaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		openai    *string
		anthropic *string
	}{
		{"openai update points to link local", lo.ToPtr("http://169.254.169.254"), nil},
		{"anthropic update points to localhost", nil, lo.ToPtr("http://localhost:9200")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &capturingEndpointRepo{}
			h := command.NewUpdateEndpointHandler(repo)
			scope := uint(303)

			err := h.Handle(t.Context(), port.UpdateEndpointCommand{
				ScopeUserID:      &scope,
				EndpointID:       1,
				OpenaiBaseURL:    tc.openai,
				AnthropicBaseURL: tc.anthropic,
			})
			if err == nil {
				t.Fatal("ssrf base url must be rejected")
			}
			if repo.findByIDN != 0 {
				t.Errorf("repo.FindByID must not be called before validation, got %d", repo.findByIDN)
			}
		})
	}
}
