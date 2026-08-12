package handler

import (
	"fmt"
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/request"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

const maxUploadSize = 10 << 20 // 10 MB limit

// UploadFile godoc
// @Summary Upload a file
// @Description Direct multipart upload to object storage
// @Tags Storage
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Param folder formData string false "Destination folder"
// @Success 201 {object} dto.UploadFileResponse
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /storage/upload [post]
func (handler *BaseHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeStorageInvalidFile, err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeStorageInvalidFile, fmt.Errorf("file field is required")))
		return
	}
	defer file.Close()

	folder := r.FormValue("folder")
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	res, err := handler.uc.UploadFile(r.Context(), header.Filename, contentType, header.Size, file, folder)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusCreated, res)
}

// GetPresignedUploadURL godoc
// @Summary Generate presigned upload URL
// @Description Creates a temporary direct upload URL for object storage
// @Tags Storage
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body payloads.PresignUploadRequest true "Upload presign payload"
// @Success 200 {object} dto.PresignedUploadResponse
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /storage/presign-upload [post]
func (handler *BaseHandler) GetPresignedUploadURL(w http.ResponseWriter, r *http.Request) {
	payload, err := request.DecodeJSON[payloads.PresignUploadRequest](r)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	res, err := handler.uc.GetPresignedUploadURL(r.Context(), payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, res)
}

// GetPresignedPreviewURL godoc
// @Summary Generate presigned preview URL
// @Description Creates a temporary signed GET URL to view/download a private file
// @Tags Storage
// @Produce json
// @Security BearerAuth
// @Param file_key query string true "File Key in storage"
// @Param expires_in query string false "Expiry duration (e.g. 15m)"
// @Success 200 {object} dto.PresignedPreviewResponse
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /storage/presign-preview [get]
func (handler *BaseHandler) GetPresignedPreviewURL(w http.ResponseWriter, r *http.Request) {
	var payload payloads.PresignPreviewRequest

	if r.Method == http.MethodGet {
		payload.FileKey = r.URL.Query().Get("file_key")
		payload.ExpiresIn = r.URL.Query().Get("expires_in")
	} else {
		p, err := request.DecodeJSON[payloads.PresignPreviewRequest](r)
		if err != nil {
			response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
			return
		}
		payload = p
	}

	res, err := handler.uc.GetPresignedPreviewURL(r.Context(), payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, res)
}

// DeleteFile godoc
// @Summary Delete file from storage
// @Description Removes an object from storage bucket
// @Tags Storage
// @Produce json
// @Security BearerAuth
// @Param file_key query string true "File Key to delete"
// @Success 204 "No Content"
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /storage/file [delete]
func (handler *BaseHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	var payload payloads.DeleteFileRequest
	if r.Method == http.MethodDelete && r.Header.Get("Content-Type") == "" {
		payload.FileKey = r.URL.Query().Get("file_key")
	} else {
		p, err := request.DecodeJSON[payloads.DeleteFileRequest](r)
		if err != nil {
			payload.FileKey = r.URL.Query().Get("file_key")
		} else {
			payload = p
		}
	}

	if err := handler.uc.DeleteFile(r.Context(), payload); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateAvatar godoc
// @Summary Update user avatar
// @Description Uploads an avatar image and updates authenticated user profile
// @Tags Storage
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param avatar formData file true "Avatar image file"
// @Success 200 {object} dto.User
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /users/avatar [put]
func (handler *BaseHandler) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeStorageInvalidFile, err))
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		file, header, err = r.FormFile("file")
		if err != nil {
			response.ErrorResponse(w, apperror.New(errconstant.ErrCodeStorageInvalidFile, fmt.Errorf("avatar or file field is required")))
			return
		}
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	user, err := handler.uc.UpdateUserAvatar(r.Context(), claims.UserID, header.Filename, contentType, header.Size, file)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, user)
}
