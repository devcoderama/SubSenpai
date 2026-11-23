// Package config membaca konfigurasi backend dari environment.
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Host            string // alamat dengar; kosong = semua antarmuka
	Port            string
	DatabaseURL     string
	SessionTTLHours int
	JWTSecret       string
	// WebOrigins: daftar origin web yang diizinkan CORS (pisah koma,
	// mis. dashboard admin di Netlify). Kosong = pola bawaan:
	// https://*.netlify.app + http://localhost (lihat middleware.CORS).
	WebOrigins string
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
	// FeedUploadDir: folder penyimpanan gambar postingan feed
	// komunitas (dilayani publik lewat /uploads/feed/{nama}).
	FeedUploadDir string
	// ImgBBAPIKey: kunci API ImgBB untuk menyimpan gambar feed
	// sebagai URL publik (permintaan Rams 2026-11-22). Kosong =
	// gambar disimpan lokal di FeedUploadDir.
	ImgBBAPIKey string
	// Discord — notifikasi komunitas lewat bot resmi SubSenpai
	// (permintaan Rams 2026-11-23). Token hanya dari env server;
	// kosong = semua notifikasi Discord nonaktif. Pengiriman cukup
	// lewat REST API Discord (bot tidak perlu gateway/online).
	DiscordBotToken          string
	DiscordGuildID           string
	DiscordChannelPengumuman string
	DiscordChannelUpdateAPK  string
	DiscordChannelJadwal     string
	// Telegram — cermin notifikasi komunitas ke topik grup
	// Telegram (permintaan Rams 2026-11-23 malam). Token hanya
	// dari env server; kosong = notifikasi Telegram nonaktif.
	TelegramBotToken        string
	TelegramGroupID         string
	TelegramTopicPengumuman string
	TelegramTopicUpdateAPK  string
	TelegramTopicJadwal     string
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
		Host:                     os.Getenv("HOST"),
		Port:                     getenv("PORT", "8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		SessionTTLHours:          ttl,
		JWTSecret:                os.Getenv("JWT_SECRET"),
		WebOrigins:               os.Getenv("WEB_ORIGINS"),
		GoogleClientID:           os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleCertsURL:           getenv("GOOGLE_CERTS_URL", "https://www.googleapis.com/oauth2/v3/certs"),
		JWTTTLHours:              jwtTTL,
		MidtransServerKey:        os.Getenv("MIDTRANS_SERVER_KEY"),
		MidtransProduction:       os.Getenv("MIDTRANS_PRODUCTION") == "true",
		XenditSecretKey:          os.Getenv("XENDIT_SECRET_KEY"),
		XenditCallbackTok:        os.Getenv("XENDIT_CALLBACK_TOKEN"),
		GoPayBaseURL:             os.Getenv("GOPAY_MERCHANT_URL"),
		GoPayUsername:            os.Getenv("GOPAY_USERNAME"),
		GoPayPassword:            os.Getenv("GOPAY_PASSWORD"),
		TokopayMerchantID:        os.Getenv("TOKOPAY_MERCHANT_ID"),
		TokopaySecret:            os.Getenv("TOKOPAY_SECRET_KEY"),
		TokopayAPIURL:            os.Getenv("TOKOPAY_API_URL"),
		FeedUploadDir:            getenv("FEED_UPLOAD_DIR", "/opt/subsenpai/uploads/feed"),
		ImgBBAPIKey:              os.Getenv("IMGBB_API_KEY"),
		DiscordBotToken:          os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordGuildID:           os.Getenv("DISCORD_GUILD_ID"),
		DiscordChannelPengumuman: os.Getenv("DISCORD_CHANNEL_PENGUMUMAN"),
		DiscordChannelUpdateAPK:  os.Getenv("DISCORD_CHANNEL_UPDATE_APK"),
		DiscordChannelJadwal:     os.Getenv("DISCORD_CHANNEL_JADWAL"),
		TelegramBotToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramGroupID:          os.Getenv("TELEGRAM_GROUP_ID"),
		TelegramTopicPengumuman:  os.Getenv("TELEGRAM_TOPIC_PENGUMUMAN"),
		TelegramTopicUpdateAPK:   os.Getenv("TELEGRAM_TOPIC_UPDATE_APK"),
		TelegramTopicJadwal:      os.Getenv("TELEGRAM_TOPIC_JADWAL"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
