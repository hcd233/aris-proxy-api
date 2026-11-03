package enum

type Granularity = string

const (
	GranularityMinute        Granularity = "minute"
	GranularityFiveMinutes   Granularity = "5minute"
	GranularityTenMinutes    Granularity = "10minute"
	GranularityThirtyMinutes Granularity = "30minute"
	GranularityHour          Granularity = "hour"
	GranularityFourHours     Granularity = "4hour"
	GranularityDay           Granularity = "day"
	GranularityWeek          Granularity = "week"
)
