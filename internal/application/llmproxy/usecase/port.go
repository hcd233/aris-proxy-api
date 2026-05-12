// Package usecase LLMProxy 域用例层 — 端口定义
package usecase

import "github.com/hcd233/aris-proxy-api/internal/dto"

// TaskSubmitter 异步任务提交端口
//
// 由 infrastructure/pool.PoolManager 实现，避免 application 层直接依赖 infrastructure。
// usecase 通过该接口将模型调用审计和消息存储任务提交到协程池异步执行。
type TaskSubmitter interface {
	SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error
	SubmitMessageStoreTask(task *dto.MessageStoreTask) error
}
