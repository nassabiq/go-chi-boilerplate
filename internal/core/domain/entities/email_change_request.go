package entities

import (
	"database/sql"
	"time"
)

type EmailChangeRequest struct {
	ID          int64          `gorm:"primaryKey"`
	UserID      int64          `gorm:"index:idx_email_change_requests_user_id"`
	NewEmail    string         `gorm:"column:new_email"`
	TokenHash   string         `gorm:"uniqueIndex:idx_email_change_requests_token_hash"`
	ExpiresAt   time.Time      `gorm:"index:idx_email_change_requests_expires_at"`
	UsedAt      sql.NullTime   `gorm:"column:used_at"`
	RequestedIP sql.NullString `gorm:"column:requested_ip"`
	UserAgent   sql.NullString `gorm:"column:user_agent"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
}

func (EmailChangeRequest) TableName() string {
	return "email_change_requests"
}
