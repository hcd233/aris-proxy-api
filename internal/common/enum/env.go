// Package enum provides common enums for the application.
package enum

// Env 环境类型
//
//	@author centonhuang
//	@update 2026-01-31 15:22:17
type Env = string

const (

	// EnvProduction 生产环境
	//
	//	@author centonhuang
	//	@update 2025-11-07 01:42:02
	EnvProduction Env = "production"

	// EnvDevelopment 开发环境
	//
	//	@author centonhuang
	//	@update 2025-11-07 01:42:02
	EnvDevelopment Env = "development"
)
