// Package upstream_error 验证 util.ExtractUpstreamStatusAndError 对不同错误类型的状态码语义。
//
// 回归保护范围：
//   - 提交 74c598d：新增 UpstreamConnectionError 类型后，对网络层错误返回 -1。
//   - 提交 2026-04-20：区分 UpstreamConnectionError（-1）与未知错误（0），
//     避免把 DTO 转换失败、context 取消等非传输错误混记为连接错误，影响观测性。
package upstream_error

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// testCase 与 fixtures/cases.json 对齐
type testCase struct {
	Name                    string `json:"name"`
	Description             string `json:"description"`
	ErrorKind               string `json:"error_kind"`
	UpstreamStatus          int    `json:"upstream_status"`
	UpstreamBody            string `json:"upstream_body"`
	CauseMessage            string `json:"cause_message"`
	ExpectedStatus          int    `json:"expected_status"`
	ExpectedMessageContains string `json:"expected_message_contains"`
}

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

// buildError 根据 fixture 描述构造错误，避免在测试代码中内联硬编码
func buildError(t *testing.T, tc testCase) error {
	t.Helper()
	switch tc.ErrorKind {
	case "nil":
		return nil
	case "upstream_http":
		return &model.UpstreamError{StatusCode: tc.UpstreamStatus, Body: tc.UpstreamBody}
	case "upstream_connection":
		return &model.UpstreamConnectionError{Cause: errors.New(tc.CauseMessage)}
	case "upstream_connection_nil_cause":
		return &model.UpstreamConnectionError{}
	case "plain":
		return errors.New(tc.CauseMessage)
	case "context_canceled":
		return context.Canceled
	}
	t.Fatalf("unsupported error_kind: %q", tc.ErrorKind)
	return nil
}

// TestExtractUpstreamStatusAndError 覆盖所有错误分支的状态码/消息语义
func TestExtractUpstreamStatusAndError(t *testing.T) {
	for _, tc := range loadCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			err := buildError(t, tc)

			status, message := util.ExtractUpstreamStatusAndError(err)

			if status != tc.ExpectedStatus {
				t.Errorf("status = %d, want %d (desc: %s)", status, tc.ExpectedStatus, tc.Description)
			}
			if tc.ExpectedMessageContains == "" {
				if message != "" {
					t.Errorf("message = %q, want empty", message)
				}
				return
			}
			if !strings.Contains(message, tc.ExpectedMessageContains) {
				t.Errorf("message = %q, want contains %q", message, tc.ExpectedMessageContains)
			}
		})
	}
}

// TestExtractUpstreamStatusAndError_ConnectionErrorIsDistinctFromUnknown 专项回归：
// UpstreamConnectionError 必须映射为 -1，与未知错误的 0 区分。
func TestExtractUpstreamStatusAndError_ConnectionErrorIsDistinctFromUnknown(t *testing.T) {
	connStatus, _ := util.ExtractUpstreamStatusAndError(&model.UpstreamConnectionError{Cause: errors.New("dial failed")})
	unknownStatus, _ := util.ExtractUpstreamStatusAndError(errors.New("convert failed"))

	if connStatus != -1 {
		t.Fatalf("connection error status = %d, want -1", connStatus)
	}
	if unknownStatus != 0 {
		t.Fatalf("unknown error status = %d, want 0", unknownStatus)
	}
	if connStatus == unknownStatus {
		t.Fatalf("connection error (%d) and unknown error (%d) must have distinct status codes", connStatus, unknownStatus)
	}
}
