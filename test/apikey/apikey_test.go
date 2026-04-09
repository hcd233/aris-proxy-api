// Package apikey API Key 测试
package apikey

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

const (
	apiKeyPrefix       = "sk-aris-"
	apiKeyRandomLength = 24
	apiKeyTotalLength  = 32
)

// TestAPIKeyConstants 验证 API Key 常量定义
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func TestAPIKeyConstants(t *testing.T) {
	t.Run("prefix constant", func(t *testing.T) {
		if constant.APIKeyPrefix != apiKeyPrefix {
			t.Errorf("APIKeyPrefix = %s, want %s", constant.APIKeyPrefix, apiKeyPrefix)
		}
	})

	t.Run("random length constant", func(t *testing.T) {
		if constant.APIKeyRandomLength != apiKeyRandomLength {
			t.Errorf("APIKeyRandomLength = %d, want %d", constant.APIKeyRandomLength, apiKeyRandomLength)
		}
	})

	t.Run("total length", func(t *testing.T) {
		totalLen := len(apiKeyPrefix) + apiKeyRandomLength
		if totalLen != apiKeyTotalLength {
			t.Errorf("total length = %d, want %d", totalLen, apiKeyTotalLength)
		}
	})
}

// TestCreateAPIKeyReqBody 验证 CreateAPIKeyReqBody 结构体
//
//	@author centonhuang
//	@update 2026-04-08 10:00:00
func TestCreateAPIKeyReqBody(t *testing.T) {
	jsonStr := `{"name":"Test Key"}`
	var req dto.CreateAPIKeyReqBody
	if err := sonic.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Name != "Test Key" {
		t.Errorf("Name = %s, want Test Key", req.Name)
	}
}
