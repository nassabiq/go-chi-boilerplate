package config

import "fmt"

type DatabaseConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	Name        string
	MaxConns    int32
	MinConns    int32
	MaxIdleConn int32
	MinIdleConn int32
	SSLMode     bool
}

func loadDB() (DatabaseConfig, error) {
	return DatabaseConfig{
		Host:        getEnv("DB_HOST", "localhost"),
		Port:        getEnvInt("DB_PORT", 5432),
		User:        getEnv("DB_USER", "postgres"),
		Password:    getEnv("DB_PASSWORD", "postgres"),
		Name:        getEnv("DB_NAME", "postgres"),
		MaxConns:    int32(getEnvInt("DB_MAX_CONNECTION", 25)),
		MinConns:    int32(getEnvInt("DB_MIN_CONNECTION", 25)),
		MaxIdleConn: int32(getEnvInt("DB_MAX_IDLE_CONNECTION", 25)),
		MinIdleConn: int32(getEnvInt("DB_MIN_IDLE_CONNECTION", 25)),
		SSLMode:     getEnvBool("DB_USE_SSL_MODE", false),
	}, nil
}

func (cfg *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.Name, cfg.DB.User, cfg.DB.Password,
	)
}
