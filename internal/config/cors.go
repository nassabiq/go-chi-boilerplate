package config

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

func loadCORS() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   getEnvList("CORS_ALLOWED_ORIGINS", "*"),
		AllowedMethods:   getEnvList("CORS_ALLOWED_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS"),
		AllowedHeaders:   getEnvList("CORS_ALLOWED_HEADERS", "Accept,Authorization,Content-Type,X-Request-ID"),
		AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", false),
		MaxAge:           getEnvInt("CORS_MAX_AGE", 300),
	}
}
