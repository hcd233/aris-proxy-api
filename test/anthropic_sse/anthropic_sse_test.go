// Package anthropic_sse 验证 Anthropic 流式结束帧 (message_stop) 的写入行为。
//
// 回归保护：
//   - 提交 184dcf9 将 forwardViaOpenAI 的 message_stop data 从 `{}` 统一为
//     `{"type":"message_stop"}`，与 forwardNative 对齐，符合 Anthropic SSE 协议规范。
//     本测试断言 util.WriteAnthropicMessageStop 产出的字节序列完全一致。
package anthropic_sse

import (
	"bufio"
	"bytes"
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type testCase struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	ExpectedFrame string `json:"expected_frame"`
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

// TestWriteAnthropicMessageStop_Frame 断言两条转发路径下生成的 SSE frame 完全一致且符合协议
func TestWriteAnthropicMessageStop_Frame(t *testing.T) {
	for _, tc := range loadCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			var buf bytes.Buffer
			w := bufio.NewWriter(&buf)

			if err := util.WriteAnthropicMessageStop(w); err != nil {
				t.Fatalf("WriteAnthropicMessageStop unexpected error: %v", err)
			}

			got := buf.String()
			if got != tc.ExpectedFrame {
				t.Errorf("frame mismatch:\n  got:  %q\n  want: %q", got, tc.ExpectedFrame)
			}
		})
	}
}

// TestAnthropicMessageStopSSEFrame_Format 保护常量本身的格式稳定
func TestAnthropicMessageStopSSEFrame_Format(t *testing.T) {
	frame := constant.AnthropicMessageStopSSEFrame

	if !bytes.HasPrefix([]byte(frame), []byte("event: message_stop\n")) {
		t.Errorf("frame must start with 'event: message_stop\\n', got %q", frame)
	}
	if !bytes.HasSuffix([]byte(frame), []byte("\n\n")) {
		t.Errorf("frame must end with double newline, got %q", frame)
	}
	// data 行必须为规范 JSON（非空对象），不能退化回 `data: {}`
	if bytes.Contains([]byte(frame), []byte("data: {}\n")) {
		t.Errorf("frame must NOT contain empty-object data payload (regression of 184dcf9), got %q", frame)
	}
	if !bytes.Contains([]byte(frame), []byte(`"type":"message_stop"`)) {
		t.Errorf("frame data must include type=message_stop, got %q", frame)
	}
}
