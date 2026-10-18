package vo

import "time"

// QuotaWindow 订阅窗口用量快照。
//
// Key 取值：Claude 侧 five_hour / seven_day / seven_day_sonnet；
// Codex 侧 primary（5h）/ secondary（7d）。前端按数组顺序渲染。
type QuotaWindow struct {
	Key           string    `json:"key"`
	UsedPercent   float64   `json:"used_percent"`
	ResetsAt      time.Time `json:"resets_at"`
	WindowMinutes int       `json:"window_minutes"`
}

// QuotaSnapshot 一次额度探测的结果（持久化为 credentials.quota_snapshot 的 JSON）。
//
// Windows 恒为非 nil（空快照序列化为 []，避免前端 null 分支）。
type QuotaSnapshot struct {
	Windows    []QuotaWindow `json:"windows"`
	PlanType   string        `json:"plan_type"`
	CapturedAt time.Time     `json:"captured_at"`
}
