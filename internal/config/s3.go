package config

import "time"

type S3Config struct {
	Endpoint            string
	Region              string
	AccessKey           string
	SecretKey           string
	Bucket              string
	UseSSL              bool
	UsePathStyle        bool
	PublicBaseURL       string
	PresignExpiry       time.Duration
	TempCleanupInterval time.Duration
	TempMaxAge          time.Duration
}

func loadS3() (S3Config, error) {
	return S3Config{
		Endpoint:            getEnv("STORAGE_ENDPOINT", "localhost"),
		Region:              getEnv("STORAGE_REGION", "us-east-1"),
		AccessKey:           getEnv("STORAGE_ACCESS_KEY", ""),
		SecretKey:           getEnv("STORAGE_SECRET_KEY", ""),
		Bucket:              getEnv("STORAGE_BUCKET", "minio"),
		UseSSL:              getEnvBool("STORAGE_USE_SSL", false),
		UsePathStyle:        getEnvBool("STORAGE_USE_PATH_STYLE", true),
		PublicBaseURL:       getEnv("STORAGE_PUBLIC_BASE_URL", ""),
		PresignExpiry:       getEnvDuration("STORAGE_PRESIGN_EXPIRY", 15*time.Minute),
		TempCleanupInterval: getEnvDuration("STORAGE_TEMP_CLEANUP_INTERVAL", 6*time.Hour),
		TempMaxAge:          getEnvDuration("STORAGE_TEMP_MAX_AGE", 24*time.Hour),
	}, nil
}
