// Package config membaca konfigurasi backend dari environment.
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	SessionTTLHours int
	JWTSecret       string
}

func Load() Config {
	ttl, err := strconv.Atoi(getenv("SESSION_TTL_HOURS", "24"))
	if err != nil || ttl <= 0 {
		ttl = 24
	}
	return Config{
		Port:            getenv("PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		SessionTTLHours: ttl,
		JWTSecret:       os.Getenv("JWT_SECRET"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
