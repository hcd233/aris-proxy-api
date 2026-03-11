package dto

import "context"

// PingTask 健康检查任务
//
//	author centonhuang
//	update 2026-02-04 16:30:00
type PingTask struct {
	Ctx context.Context
}

// MessageStoreTask 消息存储任务
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type MessageStoreTask struct {
	Ctx        context.Context
	APIKeyName string
	Model      string
	Messages   []*ChatCompletionMessageParam // 用户请求消息列表
}
