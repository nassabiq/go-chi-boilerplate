package entities

import (
	"database/sql"
	"time"
)

type PasswordResetToken struct {
	ID          int64          `gorm:"primaryKey"`
	UserID      int64          `gorm:"index:idx_password_reset_tokens_user_id"`
	TokenHash   string         `gorm:"uniqueIndex:idx_password_reset_tokens_token_hash"`
	ExpiresAt   time.Time      `gorm:"index:idx_password_reset_tokens_expires_at"`
	UsedAt      sql.NullTime   `gorm:"column:used_at"`
	RequestedIP sql.NullString `gorm:"column:requested_ip"`
	UserAgent   sql.NullString `gorm:"column:user_agent"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
}

func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}
