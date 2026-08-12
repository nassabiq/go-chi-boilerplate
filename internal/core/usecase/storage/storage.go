package storageusecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/storage"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
)

type StorageDependencies struct {
	Repo      *repositories.BaseRepository
	Validator *validator.Validate
	Storage   *storage.S3Storage
	Config    *config.Config
	Logger    *slog.Logger
}

type StorageUsecase struct {
	repo      *repositories.BaseRepository
	validator *validator.Validate
	storage   *storage.S3Storage
	cfg       *config.Config
	logger    *slog.Logger
}

func NewStorageUsecase(deps *StorageDependencies) *StorageUsecase {
	return &StorageUsecase{
		repo:      deps.Repo,
		validator: deps.Validator,
		storage:   deps.Storage,
		cfg:       deps.Config,
		logger:    deps.Logger,
	}
}

func (uc *StorageUsecase) UploadFile(ctx context.Context, fileName string, contentType string, size int64, body io.Reader, folder string) (*dto.UploadFileResponse, error) {
	if fileName == "" || body == nil {
		return nil, apperror.New(errconstant.ErrCodeStorageInvalidFile, fmt.Errorf("invalid filename or content"))
	}

	if folder == "" {
		folder = "uploads"
	}

	folder = strings.Trim(folder, "/")
	ext := filepath.Ext(fileName)
	uniqueKey := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	url, err := uc.storage.Upload(ctx, uniqueKey, body, size, contentType)
	if err != nil {
		uc.logger.ErrorContext(ctx, "failed to upload file to storage", slog.String("error", err.Error()))
		return nil, apperror.New(errconstant.ErrCodeStorageUploadFailed, err)
	}

	return &dto.UploadFileResponse{
		FileKey:     uniqueKey,
		URL:         url,
		ContentType: contentType,
		Size:        size,
	}, nil
}

func (uc *StorageUsecase) GetPresignedUploadURL(ctx context.Context, payload payloads.PresignUploadRequest) (*dto.PresignedUploadResponse, error) {
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	folder := payload.Folder
	if folder == "" {
		folder = "uploads"
	}
	folder = strings.Trim(folder, "/")
	ext := filepath.Ext(payload.Filename)
	uniqueKey := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	uploadURL, expiresAt, err := uc.storage.GeneratePresignedUploadURL(ctx, uniqueKey, payload.ContentType, 0)
	if err != nil {
		uc.logger.ErrorContext(ctx, "failed to generate presigned upload url", slog.String("error", err.Error()))
		return nil, apperror.New(errconstant.ErrCodeStorageUploadFailed, err)
	}

	return &dto.PresignedUploadResponse{
		UploadURL: uploadURL,
		FileKey:   uniqueKey,
		ExpiresAt: expiresAt,
		Headers: map[string]string{
			"Content-Type": payload.ContentType,
		},
	}, nil
}

func (uc *StorageUsecase) GetPresignedPreviewURL(ctx context.Context, payload payloads.PresignPreviewRequest) (*dto.PresignedPreviewResponse, error) {
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	var expiryDuration time.Duration
	if payload.ExpiresIn != "" {
		d, err := time.ParseDuration(payload.ExpiresIn)
		if err == nil && d > 0 {
			expiryDuration = d
		}
	}

	previewURL, expiresAt, err := uc.storage.GeneratePresignedPreviewURL(ctx, payload.FileKey, expiryDuration)
	if err != nil {
		uc.logger.ErrorContext(ctx, "failed to generate presigned preview url", slog.String("error", err.Error()))
		return nil, apperror.New(errconstant.ErrCodeStorageFileNotFound, err)
	}

	return &dto.PresignedPreviewResponse{
		PreviewURL: previewURL,
		FileKey:    payload.FileKey,
		ExpiresAt:  expiresAt,
	}, nil
}

func (uc *StorageUsecase) DeleteFile(ctx context.Context, payload payloads.DeleteFileRequest) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	if err := uc.storage.Delete(ctx, payload.FileKey); err != nil {
		uc.logger.ErrorContext(ctx, "failed to delete file from storage", slog.String("error", err.Error()))
		return apperror.New(errconstant.ErrCodeStorageUploadFailed, err)
	}

	return nil
}

func (uc *StorageUsecase) UpdateUserAvatar(ctx context.Context, userID int64, fileName string, contentType string, size int64, body io.Reader) (*dto.User, error) {
	user, err := uc.repo.FindUserById(ctx, userID)
	if err != nil {
		return nil, err
	}

	uploadRes, err := uc.UploadFile(ctx, fileName, contentType, size, body, "avatars")
	if err != nil {
		return nil, err
	}

	if err := uc.repo.UpdateUserAvatarURL(ctx, userID, uploadRes.URL); err != nil {
		uc.logger.ErrorContext(ctx, "failed to update user avatar url in db", slog.String("error", err.Error()))
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	user.AvatarURL = &uploadRes.URL
	user.Version++
	return &dto.User{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		StatusID:  int16(user.StatusID),
		Version:   user.Version,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}
