// Package enum provides common enums for the application.
package enum

// ServiceTier 服务层级
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type ServiceTier = string

const (

	// ServiceTierAuto 自动，使用项目设置中配置的服务层级
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ServiceTierAuto ServiceTier = "auto"

	// ServiceTierDefault 默认，使用标准定价和性能
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ServiceTierDefault ServiceTier = "default"

	// ServiceTierFlex 弹性处理
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ServiceTierFlex ServiceTier = "flex"

	// ServiceTierScale 规模处理
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ServiceTierScale ServiceTier = "scale"

	// ServiceTierPriority 优先处理
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ServiceTierPriority ServiceTier = "priority"
)
