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
	// GoogleClientID adalah OAuth client ID (web) untuk verifikasi
	// audiens token ID Google. Kosong = login Google nonaktif.
	GoogleClientID string
	// GoogleCertsURL adalah alamat JWKS Google (bisa diganti untuk uji).
	GoogleCertsURL string
	// JWTTTLHours adalah masa berlaku JWT sesi backend (bawaan 30 hari).
	JWTTTLHours int
}

func Load() Config {
	ttl, err := strconv.Atoi(getenv("SESSION_TTL_HOURS", "24"))
	if err != nil || ttl <= 0 {
		ttl = 24
	}
	jwtTTL, err := strconv.Atoi(getenv("JWT_TTL_HOURS", "720"))
	if err != nil || jwtTTL <= 0 {
		jwtTTL = 720
	}
	return Config{
		Port:            getenv("PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		SessionTTLHours: ttl,
		JWTSecret:       os.Getenv("JWT_SECRET"),
		GoogleClientID:  os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleCertsURL:  getenv("GOOGLE_CERTS_URL", "https://www.googleapis.com/oauth2/v3/certs"),
		JWTTTLHours:     jwtTTL,
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
