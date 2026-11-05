package model

// CronCallAuditMetadata 定时任务执行审计元数据
//
// 每个字段对应一种定时任务产出的统计指标，未执行的统计项保持 0。
//
//	@author centonhuang
//	@update 2026-06-24 10:00:00
type CronCallAuditMetadata struct {
	CheckedSessions   int64 `json:"checked_sessions_count" doc:"Session去重：检查的Session数"`
	DedupedSessions   int64 `json:"deduped_sessions_count" doc:"Session去重：去重的Session数"`
	PurgedMessages    int64 `json:"purged_messages_count" doc:"软删除清理：清理的消息数"`
	PurgedTools       int64 `json:"purged_tools_count" doc:"软删除清理：清理的工具数"`
	ScannedMessages   int64 `json:"scanned_messages_count" doc:"Think提取：扫描的消息数"`
	ExtractedMessages int64 `json:"extracted_messages_count" doc:"Think提取：提取的消息数"`
	SyncedHits        int64 `json:"synced_hits_count" doc:"封锁点击同步：同步的命中数"`
	UploadedBlobs     int64 `json:"uploaded_blobs_count" doc:"图片补传：本次上传成功数"`
	PendingBlobs      int64 `json:"pending_blobs_count" doc:"图片补传：剩余待上传数"`
	FailedBlobs       int64 `json:"failed_blobs_count" doc:"图片补传：超过重试上限数"`
}
