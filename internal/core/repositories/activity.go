package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
)

func (repo *BaseRepository) LogActivity(ctx context.Context, userID int64, eventType string, ip, ua string, metadata map[string]any) error {
	var metaBytes []byte
	if metadata != nil {
		var err error
		metaBytes, err = json.Marshal(metadata)
		if err != nil {
			return apperror.New(errconstant.ErrCodeInternal, err)
		}
	}

	log := &entities.ActivityLog{
		UserID:    userID,
		EventType: eventType,
		IPAddress: helper.NullableString(ip),
		UserAgent: helper.NullableString(ua),
		Metadata:  metaBytes,
		CreatedAt: time.Now(),
	}

	if err := repo.DB(ctx).Create(log).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}

	return nil
}
