package payloads

import "github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,max=255"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,password"`
}

type UpdateUserRequest struct {
	Name     string       `json:"name" validate:"required,max=255"`
	Email    string       `json:"email" validate:"required,email,max=255"`
	StatusID enums.Status `json:"status_id" validate:"required"`
	Version  int64        `json:"version" validate:"required,gt=0"`
}

type Mutation struct {
	Version int64 `json:"version" validate:"required,gt=0"`
}

type CreateRoleRequest struct {
	Code        string `json:"code" validate:"required,max=64"`
	Name        string `json:"name" validate:"required,max=128"`
	Description string `json:"description"`
}

type CreatePermissionRequest struct {
	Code        string `json:"code" validate:"required,max=64"`
	Name        string `json:"name" validate:"required,max=128"`
	Description string `json:"description"`
}

type AssignPermissionToRoleRequest struct {
	RoleCode       string `json:"role_code" validate:"required"`
	PermissionCode string `json:"permission_code" validate:"required"`
}

type AssignRoleToUserRequest struct {
	UserID   int64  `json:"user_id" validate:"required,gt=0"`
	RoleCode string `json:"role_code" validate:"required"`
}
