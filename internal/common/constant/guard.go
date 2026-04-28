package constant

import "time"

const (
	GuardStrikeThreshold = 5
	GuardStrikeWindow    = 1 * time.Minute
	GuardBanDuration     = 1 * time.Hour
)
