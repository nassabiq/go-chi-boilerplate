package config

import "time"

type RateLimitConfig struct {
	Enabled bool
	Default RateLimitRule
	Login   RateLimitRule
	Resend  RateLimitRule
	Forgot  RateLimitRule
}

type RateLimitRule struct {
	Requests int
	Window   time.Duration
	Burst    int
}

func loadRateLimit() (RateLimitConfig, error) {
	return RateLimitConfig{
		Enabled: getEnvBool("RATE_LIMIT_ENABLED", false),
		Default: RateLimitRule{
			Requests: getEnvInt("RATE_LIMIT_REQUESTS", 100),
			Window:   getEnvDuration("RATE_LIMIT_WINDOW", time.Hour),
			Burst:    getEnvInt("RATE_LIMIT_BURST", 20),
		},
		Login: RateLimitRule{
			Requests: getEnvInt("RATE_LIMIT_LOGIN_REQUESTS", 10),
			Window:   getEnvDuration("RATE_LIMIT_LOGIN_WINDOW", time.Minute),
			Burst:    getEnvInt("RATE_LIMIT_LOGIN_BURST", 0),
		},
		Resend: RateLimitRule{
			Requests: getEnvInt("RATE_LIMIT_RESEND_REQUESTS", 3),
			Window:   getEnvDuration("RATE_LIMIT_RESEND_WINDOW", time.Hour),
			Burst:    getEnvInt("RATE_LIMIT_RESEND_BURST", 0),
		},
		Forgot: RateLimitRule{
			Requests: getEnvInt("RATE_LIMIT_FORGOT_REQUESTS", 5),
			Window:   getEnvDuration("RATE_LIMIT_FORGOT_WINDOW", time.Hour),
			Burst:    getEnvInt("RATE_LIMIT_FORGOT_BURST", 0),
		},
	}, nil
}
