package config

import "strconv"

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	Name     string
	UseTLS   bool
}

func loadSMTP() (SMTPConfig, error) {
	useTLS, err := strconv.ParseBool(getEnv("SMTP_USE_TLS", "false"))
	if err != nil {
		return SMTPConfig{}, err
	}

	return SMTPConfig{
		Host:     getEnv("SMTP_HOST", ""),
		Port:     getEnvInt("SMTP_PORT", 1025),
		User:     getEnv("SMTP_USER", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", ""),
		Name:     getEnv("SMTP_FROM_NAME", "Boilerplate"),
		UseTLS:   useTLS,
	}, nil
}
