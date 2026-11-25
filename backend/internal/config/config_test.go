package config

import (
	"os"
	"testing"
)

// Pemetaan env grup Telegram kedua untuk jadwal rilis: bot yang
// sama mengirim jadwal ke grup + topik berbeda (permintaan Rams
// 2026-11-25, grup sistem topup -1002084839985 topik 65332).
func TestLoadTelegramJadwalGrup2(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token-uji")
	t.Setenv("TELEGRAM_GROUP_ID2", "-1002084839985")
	t.Setenv("TELEGRAM_TOPIC_JADWAL2", "65332")
	cfg := Load()
	if cfg.TelegramGroupID2 != "-1002084839985" || cfg.TelegramTopicJadwal2 != "65332" {
		t.Fatalf("grup jadwal kedua tidak terpetakan: grup=%q topik=%q",
			cfg.TelegramGroupID2, cfg.TelegramTopicJadwal2)
	}
	// Tanpa env grup kedua: kosong, dan grup utama tidak ikut berubah.
	os.Unsetenv("TELEGRAM_GROUP_ID2")
	os.Unsetenv("TELEGRAM_TOPIC_JADWAL2")
	cfg2 := Load()
	if cfg2.TelegramGroupID2 != "" || cfg2.TelegramTopicJadwal2 != "" {
		t.Fatalf("grup kedua harus kosong tanpa env: %q %q",
			cfg2.TelegramGroupID2, cfg2.TelegramTopicJadwal2)
	}
}
