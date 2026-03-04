package enum

type (
	// Permission string 权限
	//	update 2024-09-21 01:34:29
	Permission string
)

const (

	// PermissionPending general permission
	//	update 2024-06-22 10:05:15
	PermissionPending Permission = "pending"

	// PermissionUser user permission
	//	update 2024-06-22 10:05:17
	PermissionUser Permission = "user"

	// PermissionAdmin admin permission
	//	update 2024-06-22 10:05:17
	PermissionAdmin Permission = "admin"
)

// Level 获取权限等级
//
//	@param p Permission
//	@return int8
//	@author centonhuang
//	@update 2025-11-07 15:05:26
func (p Permission) Level() int8 {
	return permissionLevelMapping[p]
}

// PermissionLevelMapping 权限等级映射
//
//	update 2024-09-21 01:34:29
var (
	permissionLevelMapping = map[Permission]int8{
		PermissionPending: 1,
		PermissionUser:    2,
		PermissionAdmin:   3,
	}
)
