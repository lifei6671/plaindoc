package config

import "os"

type Config struct {
	Env       string
	Addr      string
	WebOrigin string
}

func Load() Config {
	return Config{
		Env:       getenv("APP_ENV", "development"),
		Addr:      getenv("APP_ADDR", ":8080"),
		WebOrigin: getenv("WEB_ORIGIN", "http://localhost:5173"),
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
