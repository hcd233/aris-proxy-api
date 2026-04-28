// Package logger provides a logger that can be used throughout the application.
package logger

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/samber/lo"
	tencentcloud_cls_sdk_go "github.com/tencentcloud/tencentcloud-cls-sdk-go"
	"go.uber.org/zap/zapcore"
)

// clsCore 实现 zapcore.Core，将日志异步上报到腾讯云 CLS。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
type clsCore struct {
	producer *tencentcloud_cls_sdk_go.AsyncProducerClient
	topicID  string
	level    zapcore.LevelEnabler
	fields   []zapcore.Field
}

// clsCallback CLS 发送结果回调，避免使用 logger 防止循环。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
type clsCallback struct{}

// Success 发送成功回调。
func (c *clsCallback) Success(_ *tencentcloud_cls_sdk_go.Result) {}

// Fail 发送失败回调，输出到 stderr。
func (c *clsCallback) Fail(result *tencentcloud_cls_sdk_go.Result) {
	fmt.Fprintf(
		os.Stderr,
		"[CLS] Send log failed: code=%s, message=%s, requestId=%s\n",
		result.GetErrorCode(),
		result.GetErrorMessage(),
		result.GetRequestId(),
	)
}

// newCLSCore 创建 CLS Core。如果配置不完整则返回 nil。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func newCLSCore() *clsCore {
	if config.CLSEndpoint == "" || config.CLSSecretID == "" || config.CLSSecretKey == "" || config.CLSTopicID == "" {
		return nil
	}

	level := zapcore.InfoLevel
	switch config.CLSLevel {
	case constant.CLSLevelDebug:
		level = zapcore.DebugLevel
	case constant.CLSLevelWarn:
		level = zapcore.WarnLevel
	case constant.CLSLevelError:
		level = zapcore.ErrorLevel
	}

	producerConfig := tencentcloud_cls_sdk_go.GetDefaultAsyncProducerClientConfig()
	producerConfig.Endpoint = config.CLSEndpoint
	producerConfig.AccessKeyID = config.CLSSecretID
	producerConfig.AccessKeySecret = config.CLSSecretKey

	producer, err := tencentcloud_cls_sdk_go.NewAsyncProducerClient(producerConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CLS] Failed to create producer: %v\n", err)
		return nil
	}
	producer.Start()

	return &clsCore{
		producer: producer,
		topicID:  config.CLSTopicID,
		level:    level,
	}
}

// Enabled 判断指定日志级别是否启用。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c *clsCore) Enabled(level zapcore.Level) bool {
	return c.level.Enabled(level)
}

// With 返回附加字段后的新 Core。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c *clsCore) With(fields []zapcore.Field) zapcore.Core {
	merged := lo.Values(lo.Assign(lo.SliceToMap(c.fields, func(field zapcore.Field) (string, zapcore.Field) {
		return field.Key, field
	}), lo.SliceToMap(fields, func(field zapcore.Field) (string, zapcore.Field) {
		return field.Key, field
	})))
	return &clsCore{
		producer: c.producer,
		topicID:  c.topicID,
		level:    c.level,
		fields:   merged,
	}
}

// Check 检查日志级别，满足条件时将自身加入写入列表。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c *clsCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// Write 将日志条目和字段转换为 CLS 日志并异步发送。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c *clsCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	contents := make(map[string]string, len(c.fields)+len(fields)+4)

	contents[constant.CLSFieldMessage] = entry.Message
	contents[constant.CLSFieldLevel] = entry.Level.String()
	contents[constant.CLSFieldTimestamp] = entry.Time.Format(time.RFC3339Nano)
	if entry.Caller.Defined {
		contents[constant.CLSFieldCaller] = entry.Caller.String()
	}
	if entry.Stack != "" {
		contents[constant.CLSFieldStack] = entry.Stack
	}

	allFields := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	allFields = append(allFields, c.fields...)
	allFields = append(allFields, fields...)

	for k, v := range encodeFields(allFields) {
		if _, exists := contents[k]; !exists {
			contents[k] = v
		}
	}

	log := tencentcloud_cls_sdk_go.NewCLSLog(entry.Time.Unix(), contents)
	return c.producer.SendLog(c.topicID, log, &clsCallback{})
}

// Sync 优雅关闭 CLS Producer，等待内部队列发送完成。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func (c *clsCore) Sync() error {
	if c.producer != nil {
		if err := c.producer.Close(constant.CLSProducerCloseTimeoutMs); err != nil {
			return err
		}
	}
	return nil
}

// encodeFields 将 zapcore.Field 列表转换为 map[string]string。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func encodeFields(fields []zapcore.Field) map[string]string {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	result := make(map[string]string, len(enc.Fields))
	for k, v := range enc.Fields {
		result[k] = valueToString(v)
	}
	return result
}

// valueToString 将任意值转换为字符串表示。
//
//	@author centonhuang
//	@update 2026-04-25 10:00:00
func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.FormatInt(int64(val), constant.DecimalBase)
	case int8:
		return strconv.FormatInt(int64(val), constant.DecimalBase)
	case int16:
		return strconv.FormatInt(int64(val), constant.DecimalBase)
	case int32:
		return strconv.FormatInt(int64(val), constant.DecimalBase)
	case int64:
		return strconv.FormatInt(val, constant.DecimalBase)
	case uint:
		return strconv.FormatUint(uint64(val), constant.DecimalBase)
	case uint8:
		return strconv.FormatUint(uint64(val), constant.DecimalBase)
	case uint16:
		return strconv.FormatUint(uint64(val), constant.DecimalBase)
	case uint32:
		return strconv.FormatUint(uint64(val), constant.DecimalBase)
	case uint64:
		return strconv.FormatUint(val, constant.DecimalBase)
	case float32:
		return fmt.Sprintf(constant.FormatFloatCompact, val)
	case float64:
		return fmt.Sprintf(constant.FormatFloatCompact, val)
	case bool:
		return strconv.FormatBool(val)
	case []byte:
		return string(val)
	case nil:
		return ""
	default:
		if b, err := sonic.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprint(v)
	}
}
