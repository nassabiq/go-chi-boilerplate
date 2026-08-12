package usecase

import (
	"context"
	"errors"
	"strconv"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
)

func composeUser(user *entities.User) *dto.User {
	return &dto.User{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		StatusID:  int16(user.StatusID),
		Version:   user.Version,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func parseUserStatusFilter(value string) (string, error) {
	status, ok := enums.StatusFromString(value)
	if !ok {
		return "", errors.New("invalid status")
	}

	return strconv.Itoa(int(status)), nil
}

var userListFilterParsers = map[string]helper.FilterParser{"status": parseUserStatusFilter}

func (uc *BaseUsecase) FindUsers(ctx context.Context, payload payloads.ListPayload) ([]*dto.User, int64, helper.ListParams, error) {
	params, err := helper.ValidatePayload(payload, userListFilterParsers)
	if err != nil {
		return nil, 0, params, apperror.New(errconstant.ErrCodeValidation, err)
	}

	users, total, err := uc.repo.FindUsers(ctx, params)
	if err != nil {
		return nil, 0, params, err
	}

	items := make([]*dto.User, 0, len(users))
	for i := range users {
		items = append(items, composeUser(users[i]))
	}

	return items, total, params, nil
}

func (uc *BaseUsecase) FindUserByID(ctx context.Context, id int64) (*dto.User, error) {
	user, err := uc.repo.FindUserById(ctx, id)
	if err != nil {
		return nil, err
	}

	return composeUser(user), nil
}

func (uc *BaseUsecase) CreateUser(ctx context.Context, payload payloads.CreateUserRequest) (*dto.User, error) {
	payload.Email = helper.NormalizeEmail(payload.Email)
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	password, err := auth.HashPassword(payload.Password, uc.bcryptCost())
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	user := &entities.User{
		Name:     payload.Name,
		Email:    payload.Email,
		Password: password,
		StatusID: enums.StatusPending,
		Version:  1,
	}

	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return composeUser(user), nil
}

func (uc *BaseUsecase) UpdateUser(ctx context.Context, id int64, payload payloads.UpdateUserRequest) (*dto.User, error) {
	payload.Email = helper.NormalizeEmail(payload.Email)
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	if !payload.StatusID.IsValid() {
		return nil, apperror.New(errconstant.ErrCodeValidation, nil)
	}

	user := &entities.User{
		ID:       id,
		Name:     payload.Name,
		Email:    payload.Email,
		StatusID: payload.StatusID,
		Version:  payload.Version,
	}

	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return composeUser(user), nil
}

func (uc *BaseUsecase) DeleteUser(ctx context.Context, payload payloads.MutationRequest) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	return uc.repo.DeleteUser(ctx, payload.ID, payload.Version)
}

func (uc *BaseUsecase) RestoreUser(ctx context.Context, payload payloads.MutationRequest) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	return uc.repo.RestoreUser(ctx, payload.ID, payload.Version)
}
