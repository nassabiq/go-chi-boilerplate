package config

type NATSConfig struct {
	URL      string
	Username string
	Password string
}

func loadNATS() (NATSConfig, error) {
	return NATSConfig{
		URL:      getEnv("NATS_URL", "nats://localhost:4222"),
		Username: getEnv("NATS_USERNAME", ""),
		Password: getEnv("NATS_PASSWORD", ""),
	}, nil
}
