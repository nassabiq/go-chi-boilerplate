package config

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Host string
	Port int
}

func loadApp() (AppConfig, HTTPConfig, error) {
	app := AppConfig{
		Name: getEnv("APP_NAME", "Go Backend Chi"),
		Env:  getEnv("APP_ENV", "Development"),
	}

	http := HTTPConfig{
		Host: getEnv("HTTP_HOST", "localhost"),
		Port: getEnvInt("HTTP_PORT", 8080),
	}

	return app, http, nil
}
