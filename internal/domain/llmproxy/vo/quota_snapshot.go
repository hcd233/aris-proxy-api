package vo

import "time"

// QuotaWindow 订阅窗口用量快照。
//
// Key 取值：Claude 侧 five_hour / seven_day / seven_day_sonnet；
// Codex 侧 primary（5h）/ secondary（7d）。前端按数组顺序渲染。
//
// UsedPercent 量纲固定为 0~100 百分比：provider 客户端负责把上游的异构形态
// （Anthropic utilization 为 0.0~1.0 比例、OpenAI used_percent 为 0~100）归一后写入。
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
