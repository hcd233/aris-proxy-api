// Package enum provides common enums for the application.
package enum

// ImageDetail 图片细节级别
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type ImageDetail = string

const (

	// ImageDetailAuto 自动选择细节级别
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ImageDetailAuto ImageDetail = "auto"

	// ImageDetailLow 低细节级别
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ImageDetailLow ImageDetail = "low"

	// ImageDetailHigh 高细节级别
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ImageDetailHigh ImageDetail = "high"
)
