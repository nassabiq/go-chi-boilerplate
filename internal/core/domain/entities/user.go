package entities

import (
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	"gorm.io/gorm"
)

type User struct {
	ID        int64
	Name      string
	Email     string
	Password  string
	AvatarURL *string
	StatusID  enums.Status
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}
