// Package dto 用户DTO
package dto

// User 用户实体
//
//	author centonhuang
//	update 2025-01-05 11:37:01
type User struct {
	Name   string `json:"name,omitempty" doc:"Display name of the user"`
	Email  string `json:"email,omitempty" doc:"Email address of the user"`
	Avatar string `json:"avatar,omitempty" doc:"URL or path to the user's avatar image"`
}

// DetailedUser 显示用户实体
//
//	@author centonhuang
//	@update 2025-11-07 02:43:56
type DetailedUser struct {
	ID         uint   `json:"id" doc:"Unique identifier for the user"`
	CreatedAt  string `json:"createdAt,omitempty" doc:"Timestamp when the user account was created"`
	LastLogin  string `json:"lastLogin,omitempty" doc:"Timestamp of the user's last login"`
	Permission string `json:"permission,omitempty" doc:"Permission level of the user"`
	User
}

// GetCurUserRsp represents the response containing the current user's detailed information
//
//	author centonhuang
//	update 2025-01-04 21:00:59
type GetCurUserRsp struct {
	CommonRsp
	User *DetailedUser `json:"user,omitempty" doc:"Complete user information including permissions"`
}

// UpdateUserReq represents a request to update the current user's information
//
//	author centonhuang
//	update 2025-01-04 21:19:47
type UpdateUserReq struct {
	Body *UpdateUserReqBody `json:"body" doc:"Request body containing fields to update"`
}

// UpdateUserReqBody contains the fields that can be updated for a user
//
//	author centonhuang
//	update 2025-10-31 02:33:48
type UpdateUserReqBody struct {
	User *User `json:"user" required:"true" doc:"User information to update"`
}
