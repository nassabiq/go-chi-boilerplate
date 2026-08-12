package entities

import (
	"database/sql"
	"encoding/json"
	"time"
)

type ActivityLog struct {
	ID        int64           `gorm:"primaryKey"`
	UserID    int64           `gorm:"index:idx_activity_logs_user_id"`
	EventType string          `gorm:"index:idx_activity_logs_event_type"`
	IPAddress sql.NullString  `gorm:"column:ip_address"`
	UserAgent sql.NullString  `gorm:"column:user_agent"`
	Metadata  json.RawMessage `gorm:"type:jsonb;column:metadata"`
	CreatedAt time.Time       `gorm:"column:created_at;index:idx_activity_logs_created_at"`
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}