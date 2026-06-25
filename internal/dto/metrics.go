package dto

// RuntimeMetricsReq 运行时指标查询请求
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type RuntimeMetricsReq struct {
	Range string `query:"range" default:"1h" enum:"15m,1h,6h,24h" doc:"时间范围档"`
	Since int64  `query:"since" doc:"客户端已持有的最后一个桶的 unix 秒；0 表示拉取全窗口"`
}

// RuntimeMetricsRsp 运行时指标查询响应
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type RuntimeMetricsRsp struct {
	CommonRsp
	Series     RuntimeSeries `json:"series" doc:"运行时指标时序（跨 pod 聚合后）"`
	LatestTime int64         `json:"latestTime" doc:"最新桶的 unix 秒"`
}

// RuntimeSeries 各运行时指标的时序
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type RuntimeSeries struct {
	Goroutines []RuntimePoint            `json:"goroutines" doc:"goroutine 数（集群求和）"`
	HeapMB     []RuntimePoint            `json:"heapMB" doc:"堆内存 MB（集群求和）"`
	QPS        []RuntimePoint            `json:"qps" doc:"每秒请求数（集群求和）"`
	CPUPercent []RuntimePoint            `json:"cpuPercent" doc:"CPU 使用率 %（集群求和）"`
	P95Ms      []RuntimePoint            `json:"p95Ms" doc:"P95 请求时延 ms（跨 pod 合并 bucket）"`
	SSEActive  map[string][]RuntimePoint `json:"sseActive" doc:"各 provider 的 SSE 活跃连接数"`
}

// RuntimePoint 时序点
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type RuntimePoint struct {
	Time  int64   `json:"time" doc:"桶起始 unix 秒"`
	Value float64 `json:"value" doc:"值"`
}
