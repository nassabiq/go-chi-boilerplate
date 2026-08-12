package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Http      HTTPConfig
	DB        DatabaseConfig
	Redis     RedisConfig
	NATS      NATSConfig
	S3        S3Config
	SMTP      SMTPConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
	CORS      CORSConfig
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	app, http, err := loadApp()
	if err != nil {
		return &Config{}, err
	}

	database, err := loadDB()
	if err != nil {
		return &Config{}, err
	}

	redis, err := loadRedis()
	if err != nil {
		return &Config{}, err
	}

	nats, err := loadNATS()
	if err != nil {
		return &Config{}, err
	}

	s3, err := loadS3()
	if err != nil {
		return &Config{}, err
	}

	smtp, err := loadSMTP()
	if err != nil {
		return &Config{}, err
	}

	auth, err := loadAuth()
	if err != nil {
		return &Config{}, err
	}

	rateLimit, err := loadRateLimit()
	if err != nil {
		return &Config{}, err
	}

	cors := loadCORS()

	config := &Config{
		App:       app,
		Http:      http,
		DB:        database,
		Redis:     redis,
		NATS:      nats,
		S3:        s3,
		SMTP:      smtp,
		Auth:      auth,
		RateLimit: rateLimit,
		CORS:      cors,
	}

	return config, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return strings.Trim(strings.TrimSpace(value), "\"'")
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := getEnv(key, ""); value != "" {
		if isInt, err := strconv.Atoi(value); err == nil {
			return isInt
		}
	}

	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value := getEnv(key, ""); value != "" {
		if timeValue, err := time.ParseDuration(value); err == nil {
			return timeValue
		}
	}

	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value := getEnv(key, ""); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}

	return fallback
}

func getEnvList(key, fallback string) []string {
	raw := getEnv(key, fallback)

	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if string := strings.TrimSpace(part); string != "" {
			result = append(result, string)
		}
	}

	return result
}
