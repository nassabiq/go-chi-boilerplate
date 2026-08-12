package handler

import (
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/request"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

// FindUsers godoc
// @Summary List users
// @Description Returns paginated list of users
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param skip query int false "Skip count"
// @Param limit query int false "Limit count"
// @Success 200 {object} dto.UserListResponse
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /users [get]
func (handler *BaseHandler) FindUsers(w http.ResponseWriter, r *http.Request) {
	payload, err := request.List(r)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	users, total, params, err := handler.uc.FindUsers(r.Context(), payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.SuccessResponse(w, users, total, params.Skip, params.Limit)
}

// FindUserByID godoc
// @Summary Get user by ID
// @Description Returns user details by user ID
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} dto.User
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Failure 404 {object} response.Error
// @Router /users/{id} [get]
func (handler *BaseHandler) FindUserByID(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	user, err := handler.uc.FindUserByID(r.Context(), id)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, user)
}

// CreateUser godoc
// @Summary Create a user
// @Description Creates a new user
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body payloads.CreateUserRequest true "Create user payload"
// @Success 201 {object} dto.User
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /users [post]
func (handler *BaseHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	payload, err := request.DecodeJSON[payloads.CreateUserRequest](r)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	user, err := handler.uc.CreateUser(r.Context(), payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusCreated, user)
}

// UpdateUser godoc
// @Summary Update user
// @Description Updates user details by ID
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body payloads.UpdateUserRequest true "Update user payload"
// @Success 200 {object} dto.User
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /users/{id} [put]
func (handler *BaseHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	payload, err := request.DecodeJSON[payloads.UpdateUserRequest](r)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	user, err := handler.uc.UpdateUser(r.Context(), id, payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Soft delete user
// @Description Marks user as deleted
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body payloads.MutationRequest true "Mutation payload"
// @Success 204 "No Content"
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /users/{id} [delete]
func (handler *BaseHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	payload, err := request.DecodeJSON[payloads.MutationRequest](r)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	payload.ID = id
	if err := handler.uc.DeleteUser(r.Context(), payload); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestoreUser godoc
// @Summary Restore deleted user
// @Description Restores a soft-deleted user
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body payloads.MutationRequest true "Mutation payload"
// @Success 204 "No Content"
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /users/{id}/restore [post]
func (handler *BaseHandler) RestoreUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	payload, err := request.DecodeJSON[payloads.MutationRequest](r)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	payload.ID = id

	if err := handler.uc.RestoreUser(r.Context(), payload); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
