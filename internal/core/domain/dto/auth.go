package dto

import "time"

type Auth struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type Session struct {
	ID        string    `json:"id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Current   bool      `json:"current"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Message struct {
	Message string `json:"message"`
}
