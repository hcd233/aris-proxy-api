package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// TraceDAO Trace 数据访问对象
type TraceDAO struct {
	baseDAO[dbmodel.Trace]
}

// EventDAO TraceEvent 数据访问对象
type EventDAO struct {
	baseDAO[dbmodel.TraceEvent]
}
