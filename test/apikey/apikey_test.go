// Package apikey API Key 测试
package apikey

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// testCase 测试用例结构体
type testCase struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	ExpectedPrefix      string `json:"expectedPrefix,omitempty"`
	ExpectedLength      int    `json:"expectedLength,omitempty"`
	ExpectedTotalLength int    `json:"expectedTotalLength,omitempty"`
	JSONInput           string `json:"jsonInput,omitempty"`
	ExpectedName        string `json:"expectedName,omitempty"`
}

// loadCases 加载测试用例
func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// findCase 按名称查找用例
func findCase(t *testing.T, cases []testCase, name string) testCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return testCase{}
}

// TestAPIKeyConstants_Prefix 验证 API Key 前缀常量
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func TestAPIKeyConstants_Prefix(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "constant_prefix")

	if constant.APIKeyPrefix != tc.ExpectedPrefix {
		t.Errorf("APIKeyPrefix = %s, want %s", constant.APIKeyPrefix, tc.ExpectedPrefix)
	}
}

// TestAPIKeyConstants_RandomLength 验证 API Key 随机部分长度
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func TestAPIKeyConstants_RandomLength(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "constant_random_length")

	if constant.APIKeyRandomLength != tc.ExpectedLength {
		t.Errorf("APIKeyRandomLength = %d, want %d", constant.APIKeyRandomLength, tc.ExpectedLength)
	}
}

// TestAPIKeyConstants_TotalLength 验证 API Key 总长度
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func TestAPIKeyConstants_TotalLength(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "constant_total_length")

	totalLen := len(constant.APIKeyPrefix) + constant.APIKeyRandomLength
	if totalLen != tc.ExpectedTotalLength {
		t.Errorf("total length = %d, want %d", totalLen, tc.ExpectedTotalLength)
	}
}

// TestCreateAPIKeyReqBody_ValidJSON 验证 CreateAPIKeyReqBody JSON 反序列化
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func TestCreateAPIKeyReqBody_ValidJSON(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "dto_unmarshal_valid")

	var req dto.CreateAPIKeyReqBody
	if err := sonic.Unmarshal([]byte(tc.JSONInput), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Name != tc.ExpectedName {
		t.Errorf("Name = %s, want %s", req.Name, tc.ExpectedName)
	}
}

// TestCreateAPIKeyReqBody_EmptyName 验证空名称的 CreateAPIKeyReqBody
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func TestCreateAPIKeyReqBody_EmptyName(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "dto_unmarshal_empty_name")

	var req dto.CreateAPIKeyReqBody
	if err := sonic.Unmarshal([]byte(tc.JSONInput), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Name != tc.ExpectedName {
		t.Errorf("Name = %q, want %q", req.Name, tc.ExpectedName)
	}
}

// TestCreateAPIKeyReqBody_UnicodeName 验证 Unicode 名称的 CreateAPIKeyReqBody
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func TestCreateAPIKeyReqBody_UnicodeName(t *testing.T) {
	cases := loadCases(t)
	tc := findCase(t, cases, "dto_unmarshal_unicode_name")

	var req dto.CreateAPIKeyReqBody
	if err := sonic.Unmarshal([]byte(tc.JSONInput), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Name != tc.ExpectedName {
		t.Errorf("Name = %q, want %q", req.Name, tc.ExpectedName)
	}
}
