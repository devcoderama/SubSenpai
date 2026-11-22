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
	// Pembayaran (Fase 4). Kosong = provider bersangkutan nonaktif.
	MidtransServerKey  string
	MidtransProduction bool
	XenditSecretKey    string
	XenditCallbackTok  string
	// GoPay Merchant (API pihak ketiga Api-Gopar-Merch): alamat layanan
	// + kredensial akun merchant. Kosong = QRIS otomatis & polling mati;
	// sesi nominal unik tetap dibuat.
	GoPayBaseURL  string
	GoPayUsername string
	GoPayPassword string
	// Tokopay — gateway utama (dipilih Rams 2026-11-22, fee lebih murah
	// dari Midtrans/Xendit). Kosong = checkout Tokopay nonaktif (503).
	TokopayMerchantID string
	TokopaySecret     string
	TokopayAPIURL     string // bawaan https://api.tokopay.id
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
		MidtransServerKey:  os.Getenv("MIDTRANS_SERVER_KEY"),
		MidtransProduction: os.Getenv("MIDTRANS_PRODUCTION") == "true",
		XenditSecretKey:    os.Getenv("XENDIT_SECRET_KEY"),
		XenditCallbackTok:  os.Getenv("XENDIT_CALLBACK_TOKEN"),
		GoPayBaseURL:       os.Getenv("GOPAY_MERCHANT_URL"),
		GoPayUsername:      os.Getenv("GOPAY_USERNAME"),
		GoPayPassword:      os.Getenv("GOPAY_PASSWORD"),
		TokopayMerchantID: os.Getenv("TOKOPAY_MERCHANT_ID"),
		TokopaySecret:     os.Getenv("TOKOPAY_SECRET_KEY"),
		TokopayAPIURL:     os.Getenv("TOKOPAY_API_URL"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
