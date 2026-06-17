package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

// CronJobDAO CronJob 数据访问对象
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJobDAO struct{ baseDAO[dbmodel.CronJob] }

// CronCallAuditDAO CronCallAudit 数据访问对象
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronCallAuditDAO struct{ baseDAO[dbmodel.CronCallAudit] }
