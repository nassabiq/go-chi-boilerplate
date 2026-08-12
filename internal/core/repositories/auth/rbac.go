package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"gorm.io/gorm"
)

func (r *AuthRepository) CreateRole(ctx context.Context, role *entities.Role) error {
	if err := r.DB(ctx).Create(role).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}
	return nil
}

func (r *AuthRepository) FindRoleByCode(ctx context.Context, code string) (*entities.Role, error) {
	var role entities.Role
	err := r.DB(ctx).Where("code = ?", code).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeUserNotFound, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}
	return &role, nil
}

func (r *AuthRepository) FindRoleByID(ctx context.Context, id int64) (*entities.Role, error) {
	var role entities.Role
	err := r.DB(ctx).Where("id = ?", id).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeUserNotFound, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}
	return &role, nil
}

func (r *AuthRepository) CreatePermission(ctx context.Context, perm *entities.Permission) error {
	if err := r.DB(ctx).Create(perm).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}
	return nil
}

func (r *AuthRepository) FindPermissionByCode(ctx context.Context, code string) (*entities.Permission, error) {
	var perm entities.Permission
	err := r.DB(ctx).Where("code = ?", code).First(&perm).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeUserNotFound, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}
	return &perm, nil
}

func (r *AuthRepository) AssignPermissionToRole(ctx context.Context, roleID, permissionID int64) error {
	rp := &entities.RolePermission{RoleID: roleID, PermissionID: permissionID}
	if err := r.DB(ctx).Create(rp).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}
	return nil
}

func (r *AuthRepository) GetRolePermissions(ctx context.Context, roleID int64) ([]*entities.Permission, error) {
	var perms []*entities.Permission
	err := r.DB(ctx).
		Table("permissions p").
		Select("p.*").
		Joins("JOIN role_permissions rp ON rp.permission_id = p.id").
		Where("rp.role_id = ?", roleID).
		Scan(&perms).Error
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}
	return perms, nil
}

func (r *AuthRepository) AssignRoleToUser(ctx context.Context, userID, roleID, assignedBy int64) error {
	ur := &entities.UserRole{
		UserID:     userID,
		RoleID:     roleID,
		AssignedBy: sql.NullInt64{Int64: assignedBy, Valid: assignedBy > 0},
		AssignedAt: time.Now(),
	}
	if err := r.DB(ctx).Create(ur).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}
	return nil
}

func (r *AuthRepository) GetUserRoles(ctx context.Context, userID int64) ([]*entities.Role, error) {
	var roles []*entities.Role
	err := r.DB(ctx).
		Table("roles r").
		Select("r.*").
		Joins("JOIN user_roles ur ON ur.role_id = r.id").
		Where("ur.user_id = ?", userID).
		Scan(&roles).Error
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}
	return roles, nil
}

func (r *AuthRepository) GetUserPermissions(ctx context.Context, userID int64) ([]*entities.Permission, error) {
	var perms []*entities.Permission
	err := r.DB(ctx).
		Table("permissions p").
		Select("DISTINCT p.*").
		Joins("JOIN role_permissions rp ON rp.permission_id = p.id").
		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ?", userID).
		Scan(&perms).Error
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}
	return perms, nil
}

func (r *AuthRepository) RemoveRoleFromUser(ctx context.Context, userID, roleID int64) error {
	result := r.DB(ctx).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Delete(&entities.UserRole{})
	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}
	return nil
}

func (r *AuthRepository) HasPermission(ctx context.Context, userID int64, permissionCode string) (bool, error) {
	var count int64
	err := r.DB(ctx).
		Table("permissions p").
		Joins("JOIN role_permissions rp ON rp.permission_id = p.id").
		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ? AND p.code = ?", userID, permissionCode).
		Count(&count).Error
	if err != nil {
		return false, apperror.New(errconstant.ErrCodeInternal, err)
	}
	return count > 0, nil
}
