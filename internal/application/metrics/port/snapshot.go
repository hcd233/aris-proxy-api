package port

// Snapshot 单个 instance 在某一时刻的运行时指标快照（写入 Redis 的最小单位）。
//
// 仅存"可直接相加的原值"：gauge 原值 + counter 累计值 + histogram 桶计数；
// 速率与分位的计算全部留给聚合层。
//
// 放在应用层端口包：基础设施负责采集与存储（infrastructure/metrics、
// infrastructure/cache），应用层聚合只依赖这个数据契约，不再反向 import 基础设施实现。
//
//	@author centonhuang
//	@update 2026-09-15 10:00:00
type Snapshot struct {
	TS          int64              `json:"ts"`                   // unix 秒
	Goroutines  float64            `json:"goroutines"`           // gauge
	Threads     float64            `json:"threads"`              // gauge：OS 线程数 M（旧快照无此字段解码为 0）
	HeapBytes   float64            `json:"heapBytes"`            // gauge
	CPUSeconds  float64            `json:"cpuSeconds"`           // counter 累计值 → 聚合层求 CPU%
	SSEActive   map[string]float64 `json:"sseActive,omitempty"`  // provider -> gauge
	LatBuckets  map[string]float64 `json:"latBuckets,omitempty"` // le -> 累计计数 → 聚合层求 P95
	LatCount    float64            `json:"latCount"`             // histogram 累计样本数 → 聚合层求 QPS
	TokenInput  float64            `json:"tokenInput"`           // counter 累计输入 token → 聚合层求输入速率
	TokenOutput float64            `json:"tokenOutput"`          // counter 累计输出 token → 聚合层求输出速率
	ReqStatus   map[string]float64 `json:"reqStatus,omitempty"`  // status code -> counter 累计业务请求数 → 聚合层求各状态码数量
}
