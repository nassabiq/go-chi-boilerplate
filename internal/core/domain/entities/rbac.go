package entities

import (
	"database/sql"
	"time"
)

type Role struct {
	ID          int64     `gorm:"primaryKey"`
	Code        string    `gorm:"uniqueIndex:idx_roles_code"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (Role) TableName() string {
	return "roles"
}

type Permission struct {
	ID          int64     `gorm:"primaryKey"`
	Code        string    `gorm:"uniqueIndex:idx_permissions_code"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (Permission) TableName() string {
	return "permissions"
}

type RolePermission struct {
	RoleID       int64 `gorm:"primaryKey"`
	PermissionID int64 `gorm:"primaryKey"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

type UserRole struct {
	UserID    int64      `gorm:"primaryKey"`
	RoleID    int64      `gorm:"primaryKey"`
	AssignedBy sql.NullInt64 `gorm:"column:assigned_by"`
	AssignedAt time.Time `gorm:"column:assigned_at"`
}

func (UserRole) TableName() string {
	return "user_roles"
}