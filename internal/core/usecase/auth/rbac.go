package auth

import (
	"context"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
)

// CreateRole creates a new role
func (uc *AuthUsecase) CreateRole(ctx context.Context, payload payloads.CreateRoleRequest) (*dto.Role, error) {
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	role := &entities.Role{
		Code:        payload.Code,
		Name:        payload.Name,
		Description: payload.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := uc.repo.CreateRole(ctx, role); err != nil {
		return nil, err
	}

	return uc.toRoleDTO(role), nil
}

// GetRoleByCode retrieves a role by code
func (uc *AuthUsecase) GetRoleByCode(ctx context.Context, code string) (*dto.Role, error) {
	role, err := uc.repo.FindRoleByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return uc.toRoleDTO(role), nil
}

// GetRoleByID retrieves a role by ID
func (uc *AuthUsecase) GetRoleByID(ctx context.Context, id int64) (*dto.Role, error) {
	role, err := uc.repo.FindRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return uc.toRoleDTO(role), nil
}

// CreatePermission creates a new permission
func (uc *AuthUsecase) CreatePermission(ctx context.Context, payload payloads.CreatePermissionRequest) (*dto.Permission, error) {
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	perm := &entities.Permission{
		Code:        payload.Code,
		Name:        payload.Name,
		Description: payload.Description,
		CreatedAt:   time.Now(),
	}

	if err := uc.repo.CreatePermission(ctx, perm); err != nil {
		return nil, err
	}

	return uc.toPermissionDTO(perm), nil
}

// GetPermissionByCode retrieves a permission by code
func (uc *AuthUsecase) GetPermissionByCode(ctx context.Context, code string) (*dto.Permission, error) {
	perm, err := uc.repo.FindPermissionByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return uc.toPermissionDTO(perm), nil
}

// AssignPermissionToRole assigns a permission to a role
func (uc *AuthUsecase) AssignPermissionToRole(ctx context.Context, roleCode, permissionCode string) error {
	role, err := uc.repo.FindRoleByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	perm, err := uc.repo.FindPermissionByCode(ctx, permissionCode)
	if err != nil {
		return err
	}
	return uc.repo.AssignPermissionToRole(ctx, role.ID, perm.ID)
}

// GetRolePermissions retrieves all permissions for a role
func (uc *AuthUsecase) GetRolePermissions(ctx context.Context, roleCode string) ([]*dto.Permission, error) {
	role, err := uc.repo.FindRoleByCode(ctx, roleCode)
	if err != nil {
		return nil, err
	}
	perms, err := uc.repo.GetRolePermissions(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	return uc.toPermissionDTOs(perms), nil
}

// AssignRoleToUser assigns a role to a user
func (uc *AuthUsecase) AssignRoleToUser(ctx context.Context, userID int64, roleCode string, assignedBy int64) error {
	role, err := uc.repo.FindRoleByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	return uc.repo.AssignRoleToUser(ctx, userID, role.ID, assignedBy)
}

// GetUserRoles retrieves all roles for a user
func (uc *AuthUsecase) GetUserRoles(ctx context.Context, userID int64) ([]*dto.Role, error) {
	roles, err := uc.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	return uc.toRoleDTOs(roles), nil
}

// GetUserPermissions retrieves all permissions for a user
func (uc *AuthUsecase) GetUserPermissions(ctx context.Context, userID int64) ([]*dto.Permission, error) {
	perms, err := uc.repo.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	return uc.toPermissionDTOs(perms), nil
}

// RemoveRoleFromUser removes a role from a user
func (uc *AuthUsecase) RemoveRoleFromUser(ctx context.Context, userID int64, roleCode string) error {
	role, err := uc.repo.FindRoleByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	return uc.repo.RemoveRoleFromUser(ctx, userID, role.ID)
}

// CheckUserPermission checks if a user has a specific permission
func (uc *AuthUsecase) CheckUserPermission(ctx context.Context, userID int64, permissionCode string) (bool, error) {
	return uc.repo.HasPermission(ctx, userID, permissionCode)
}

func (uc *AuthUsecase) toRoleDTO(role *entities.Role) *dto.Role {
	return &dto.Role{
		ID:          role.ID,
		Code:        role.Code,
		Name:        role.Name,
		Description: role.Description,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
}

func (uc *AuthUsecase) toRoleDTOs(roles []*entities.Role) []*dto.Role {
	result := make([]*dto.Role, len(roles))
	for i, r := range roles {
		result[i] = uc.toRoleDTO(r)
	}
	return result
}

func (uc *AuthUsecase) toPermissionDTO(perm *entities.Permission) *dto.Permission {
	return &dto.Permission{
		ID:          perm.ID,
		Code:        perm.Code,
		Name:        perm.Name,
		Description: perm.Description,
		CreatedAt:   perm.CreatedAt,
	}
}

func (uc *AuthUsecase) toPermissionDTOs(perms []*entities.Permission) []*dto.Permission {
	result := make([]*dto.Permission, len(perms))
	for i, p := range perms {
		result[i] = uc.toPermissionDTO(p)
	}
	return result
}
