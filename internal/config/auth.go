package config

import "time"

type AuthConfig struct {
	JWT               JWTConfig
	BasicAuth         BasicAuthConfig
	LoginThrottle     LoginThrottleConfig
	PasswordReset     PasswordResetConfig
	EmailVerification EmailVerificationConfig
	EmailChange       EmailChangeConfig
	Password          PasswordConfig
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	Issuer        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type BasicAuthConfig struct {
	Username string
	Password string
}

type LoginThrottleConfig struct {
	FailureWindow time.Duration
	FirstLimit    int
	FirstBlock    time.Duration

	SecondLimit int
	SecondBlock time.Duration

	MaxLimit int
	MaxBlock time.Duration
}

type PasswordResetConfig struct {
	TokenTTL time.Duration
	URL      string
}

type EmailVerificationConfig struct {
	TokenTTL time.Duration
	URL      string
}

type EmailChangeConfig struct {
	TokenTTL time.Duration
	URL      string
}

type PasswordConfig struct {
	BcryptCost int
}

func loadAuth() (AuthConfig, error) {
	return AuthConfig{
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", ""),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", ""),
			Issuer:        getEnv("JWT_ISSUER", "go-chi"),
			AccessExpiry:  getEnvDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshExpiry: getEnvDuration("JWT_REFRESH_EXPIRY", 168*time.Hour),
		},

		BasicAuth: BasicAuthConfig{
			Username: getEnv("BASIC_AUTH_USERNAME", ""),
			Password: getEnv("BASIC_AUTH_PASSWORD", ""),
		},

		LoginThrottle: LoginThrottleConfig{
			FailureWindow: getEnvDuration("LOGIN_THROTTLE_FAILURE_WINDOW", time.Hour),

			FirstLimit: getEnvInt("LOGIN_THROTTLE_FIRST_LIMIT", 5),
			FirstBlock: getEnvDuration(
				"LOGIN_THROTTLE_FIRST_BLOCK",
				30*time.Second,
			),

			SecondLimit: getEnvInt("LOGIN_THROTTLE_SECOND_LIMIT", 7),
			SecondBlock: getEnvDuration(
				"LOGIN_THROTTLE_SECOND_BLOCK",
				5*time.Minute,
			),

			MaxLimit: getEnvInt("LOGIN_THROTTLE_MAX_LIMIT", 10),
			MaxBlock: getEnvDuration(
				"LOGIN_THROTTLE_MAX_BLOCK",
				15*time.Minute,
			),
		},

		PasswordReset: PasswordResetConfig{
			TokenTTL: getEnvDuration("PASSWORD_RESET_TOKEN_TTL", 15*time.Minute),
			URL:      getEnv("PASSWORD_RESET_URL", "http://localhost:8080/auth/reset-password"),
		},

		EmailVerification: EmailVerificationConfig{
			TokenTTL: getEnvDuration("EMAIL_VERIFICATION_TOKEN_TTL", 24*time.Hour),
			URL:      getEnv("EMAIL_VERIFICATION_URL", "http://localhost:8080/auth/verify-email"),
		},

		EmailChange: EmailChangeConfig{
			TokenTTL: getEnvDuration("EMAIL_CHANGE_TOKEN_TTL", 24*time.Hour),
			URL:      getEnv("EMAIL_CHANGE_URL", "http://localhost:8080/auth/verify-email-change"),
		},

		Password: PasswordConfig{
			BcryptCost: getEnvInt("PASSWORD_BCRYPT_COST", 12),
		},
	}, nil
}
