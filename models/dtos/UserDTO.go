package dtos

import "time"

type UserDto struct {
	Id           int64     `json:"id"`
	Email        string    `json:"email"`
	Password     string    `json:"-"`
	Username     string    `json:"username"`
	ParentId     *int64    `json:"parent_id"`
	RoleId       int64     `json:"role_id"`
	RoleName     string    `json:"role_name"`
	WebToken     string    `json:"web_token"`
	DepartmentId int64     `json:"department_id"`
	CreateTime   time.Time `json:"create_time"`
}

type UpdateUserRequest struct {
	Id           int64  `json:"id"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	Username     string `json:"username"`
	RoleId       int64  `json:"role_id"`
	DepartmentId int64  `json:"department_id"`
}

type RegisterDto struct {
	UserName     string `json:"username"`
	Password     string `json:"password"`
	RoleId       int64  `json:"role_id"`
	UserId       int64  `json:"user_id"`
	DepartmentId int64  `json:"department_id"`
}
