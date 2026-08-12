package repositories

import (
	"context"
	"errors"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
	"gorm.io/gorm"
)

var userListConfig = helper.ListConfig{
	SearchColumns: []string{"name", "email"},
	SortColumns: map[string]string{
		"name":       "name",
		"email":      "email",
		"created_at": "created_at",
		"updated_at": "updated_at",
	},
	FilterColumns: map[string]string{
		"status": "status_id",
	},
	DefaultSortBy:        "created_at",
	DefaultSortDirection: "DESC",
}

func (repo *BaseRepository) FindUsers(ctx context.Context, params helper.ListParams) ([]*entities.User, int64, error) {
	var (
		users []*entities.User
		total int64
	)

	query := repo.DB(ctx).Model(&entities.User{}).Where("deleted_at IS NULL")
	query = helper.ApplySearch(query, params.Search, userListConfig.SearchColumns)
	query = helper.ApplyFilter(query, params.Filters, userListConfig.FilterColumns)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.New(errconstant.ErrCodeInternal, err)
	}

	query = helper.ApplySort(query, params, userListConfig)
	query = helper.ApplyPagination(query, params)

	if err := query.Find(&users).Error; err != nil {
		return nil, 0, apperror.New(errconstant.ErrCodeInternal, err)
	}

	return users, total, nil
}

func (repo *BaseRepository) FindUserById(ctx context.Context, id int64) (*entities.User, error) {
	var user entities.User

	err := repo.DB(ctx).Where("id = ?", id).Where("deleted_at IS NULL").First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeUserNotFound, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	return &user, nil
}

func (repo *BaseRepository) FindUserByEmail(ctx context.Context, email string) (*entities.User, error) {
	var user entities.User

	err := repo.DB(ctx).Where("email = ?", email).Where("deleted_at IS NULL").First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeUserNotFound, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	return &user, nil
}

func (repo *BaseRepository) CreateUser(ctx context.Context, user *entities.User) error {
	err := repo.DB(ctx).Create(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apperror.New(errconstant.ErrCodeUserAlreadyExists, err)
		}
		return apperror.New(errconstant.ErrCodeInternal, err)
	}

	return nil
}

func (repo *BaseRepository) UpdateUser(ctx context.Context, user *entities.User) error {
	result := repo.DB(ctx).Model(&entities.User{}).
		Where("id = ?", user.ID).
		Where("deleted_at IS NULL").
		Where("version = ?", user.Version).
		Updates(map[string]any{
			"name":       user.Name,
			"email":      user.Email,
			"status_id":  user.StatusID,
			"version":    gorm.Expr("version + 1"),
			"updated_at": gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return apperror.New(errconstant.ErrCodeUserAlreadyExists, result.Error)
		}
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeStaleResource, nil)
	}

	user.Version++

	return nil
}

func (repo *BaseRepository) DeleteUser(ctx context.Context, id int64, version int64) error {
	result := repo.DB(ctx).Model(&entities.User{}).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Where("version = ?", version).
		Updates(map[string]any{
			"deleted_at": gorm.Expr("NOW()"),
			"updated_at": gorm.Expr("NOW()"),
			"version":    gorm.Expr("version + 1"),
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeStaleResource, nil)
	}

	return nil
}

// UpdateUserStatus transitions a user to newStatus, bumping the optimistic-lock
// version. The caller (usecase) is responsible for validating that the
// transition is allowed by the account state machine; this method only
// persists it atomically.
func (repo *BaseRepository) UpdateUserStatus(ctx context.Context, userID int64, statusID enums.Status) error {
	result := repo.DB(ctx).
		Model(&entities.User{}).
		Where("id = ?", userID).
		Where("deleted_at IS NULL").
		Updates(map[string]any{
			"status_id":  statusID,
			"version":    gorm.Expr("version + 1"),
			"updated_at": gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeUserNotFound, nil)
	}

	return nil
}

func (repo *BaseRepository) RestoreUser(ctx context.Context, id int64, version int64) error {
	result := repo.DB(ctx).Model(&entities.User{}).
		Where("id = ?", id).
		Where("deleted_at IS NOT NULL").
		Where("version = ?", version).
		Updates(map[string]any{
			"deleted_at": nil,
			"updated_at": gorm.Expr("NOW()"),
			"version":    gorm.Expr("version + 1"),
		})

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return apperror.New(errconstant.ErrCodeUserAlreadyExists, result.Error)
		}
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeStaleResource, nil)
	}

	return nil
}

func (repo *BaseRepository) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	result := repo.DB(ctx).
		Model(&entities.User{}).
		Where("id = ?", userID).
		Where("deleted_at IS NULL").
		Updates(map[string]any{
			"password":   passwordHash,
			"version":    gorm.Expr("version + 1"),
			"updated_at": gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeUserNotFound, nil)
	}

	return nil
}

func (repo *BaseRepository) UpdateUserAvatarURL(ctx context.Context, userID int64, avatarURL string) error {
	result := repo.DB(ctx).
		Model(&entities.User{}).
		Where("id = ?", userID).
		Where("deleted_at IS NULL").
		Updates(map[string]any{
			"avatar_url": avatarURL,
			"version":    gorm.Expr("version + 1"),
			"updated_at": gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeUserNotFound, nil)
	}

	return nil
}
