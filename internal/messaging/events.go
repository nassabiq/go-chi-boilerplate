package messaging

import "time"

type PasswordResetRequested struct {
	EventID    string    `json:"event_id"`
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	ResetURL   string    `json:"reset_url"`
	ExpiresAt  time.Time `json:"expires_at"`
	OccurredAt time.Time `json:"occurred_at"`
}

type PasswordChanged struct {
	EventID    string    `json:"event_id"`
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurred_at"`
}

type EmailVerificationRequested struct {
	EventID        string    `json:"event_id"`
	UserID         int64     `json:"user_id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	VerificationURL string `json:"verification_url"`
	ExpiresAt      time.Time `json:"expires_at"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type EmailChangeRequested struct {
	EventID        string    `json:"event_id"`
	UserID         int64     `json:"user_id"`
	OldEmail       string    `json:"old_email"`
	NewEmail       string    `json:"new_email"`
	Name           string    `json:"name"`
	VerificationURL string `json:"verification_url"`
	ExpiresAt      time.Time `json:"expires_at"`
	OccurredAt     time.Time `json:"occurred_at"`
}
