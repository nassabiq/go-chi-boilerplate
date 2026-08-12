package auth

import (
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/request"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

// CreateRole creates a new role
func (h *AuthHandler) CreateRole(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.CreateRoleRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.CreateRole(req.Context(), payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusCreated, result)
}

// GetRoleByCode retrieves a role by code
func (h *AuthHandler) GetRoleByCode(w http.ResponseWriter, req *http.Request) {
	code, err := request.PathParam(req, "code")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.GetRoleByCode(req.Context(), code)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// CreatePermission creates a new permission
func (h *AuthHandler) CreatePermission(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.CreatePermissionRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.CreatePermission(req.Context(), payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusCreated, result)
}

// GetPermissionByCode retrieves a permission by code
func (h *AuthHandler) GetPermissionByCode(w http.ResponseWriter, req *http.Request) {
	code, err := request.PathParam(req, "code")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.GetPermissionByCode(req.Context(), code)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// AssignPermissionToRole assigns a permission to a role
func (h *AuthHandler) AssignPermissionToRole(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.AssignPermissionToRoleRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	if err := h.uc.AssignPermissionToRole(req.Context(), payload.RoleCode, payload.PermissionCode); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Permission assigned to role.",
	})
}

// GetRolePermissions retrieves all permissions for a role
func (h *AuthHandler) GetRolePermissions(w http.ResponseWriter, req *http.Request) {
	code, err := request.PathParam(req, "code")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.GetRolePermissions(req.Context(), code)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// AssignRoleToUser assigns a role to a user
func (h *AuthHandler) AssignRoleToUser(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.AssignRoleToUserRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	// Get the authenticated user ID as the assigner
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	if err := h.uc.AssignRoleToUser(req.Context(), payload.UserID, payload.RoleCode, claims.UserID); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Role assigned to user.",
	})
}

// GetUserRoles retrieves all roles for a user
func (h *AuthHandler) GetUserRoles(w http.ResponseWriter, req *http.Request) {
	userID, err := request.PathInt64(req, "user_id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.GetUserRoles(req.Context(), userID)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// GetUserPermissions retrieves all permissions for a user
func (h *AuthHandler) GetUserPermissions(w http.ResponseWriter, req *http.Request) {
	userID, err := request.PathInt64(req, "user_id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.GetUserPermissions(req.Context(), userID)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// RemoveRoleFromUser removes a role from a user
func (h *AuthHandler) RemoveRoleFromUser(w http.ResponseWriter, req *http.Request) {
	userID, err := request.PathInt64(req, "user_id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	code, err := request.PathParam(req, "code")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	if err := h.uc.RemoveRoleFromUser(req.Context(), userID, code); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Role removed from user.",
	})
}

// CheckUserPermission checks if a user has a specific permission
func (h *AuthHandler) CheckUserPermission(w http.ResponseWriter, req *http.Request) {
	userID, err := request.PathInt64(req, "user_id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	permissionCode := req.URL.Query().Get("permission")
	if permissionCode == "" {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, nil))
		return
	}

	has, err := h.uc.CheckUserPermission(req.Context(), userID, permissionCode)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]bool{"has_permission": has})
}
