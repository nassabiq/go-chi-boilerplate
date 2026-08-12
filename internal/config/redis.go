package config

import "fmt"

type RedisConfig struct {
	Host     string
	Port     int
	DB       int
	Password string
}

func loadRedis() (RedisConfig, error) {
	return RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getEnvInt("REDIS_PORT", 6379),
		DB:       getEnvInt("REDIS_DB", 0),
		Password: getEnv("REDIS_PASSWORD", "postgres"),
	}, nil
}

func (cfg RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}
