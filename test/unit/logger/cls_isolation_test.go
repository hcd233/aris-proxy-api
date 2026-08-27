package logger_test

import (
	"testing"
	"unsafe"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// 测试用常量，模拟 fasthttp 缓冲区零拷贝字符串场景
const (
	testRealPath   = "/api/openai/v1/chat/completions"
	testStickyPath = "/ready"
	testFieldKey   = "path"
	testMutatedKey = "msg!"
)

// TestEncodeFieldStringIsolation 验证字段字符串与底层字节缓冲区隔离：
// fasthttp 复用请求字节缓冲区，字段若为零拷贝引用，异步上报时内容会被
// 后续请求覆盖（router 粘连 bug），EncodeCLSFields 必须输出独立副本。
//
//	@author centonhuang
//	@update 2026-08-24 10:00:00
func TestEncodeFieldStringIsolation(t *testing.T) {
	t.Parallel()

	buf := []byte(testRealPath)
	field := zap.String(testFieldKey, unsafe.String(&buf[0], len(buf)))

	encoded := logger.EncodeCLSFields([]zap.Field{field})

	copy(buf, testStickyPath)
	if got := encoded[testFieldKey]; got != testRealPath {
		t.Errorf("encoded value aliases the original buffer: got %q", got)
	}
}

// TestEncodeFieldKeyIsolation 验证字段 key 与底层字节缓冲区隔离。
//
//	@author centonhuang
//	@update 2026-08-24 10:00:00
func TestEncodeFieldKeyIsolation(t *testing.T) {
	t.Parallel()

	buf := []byte(testFieldKey)
	field := zap.String(unsafe.String(&buf[0], len(buf)), testStickyPath)

	encoded := logger.EncodeCLSFields([]zap.Field{field})
	for k := range encoded {
		copy(buf, testMutatedKey)
		if k != testFieldKey {
			t.Errorf("encoded key aliases the original buffer: got %q", k)
		}
	}
}
