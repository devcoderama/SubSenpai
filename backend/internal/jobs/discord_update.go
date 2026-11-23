package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/discord"
	"subsenpai/backend/internal/telegram"
)

// Notifikasi update APK OTOMATIS ke Discord (permintaan Rams
// 2026-11-23 malam): worker memeriksa rilis GitHub terbaru repo
// SubSenpai pada tiap siklus; versi yang belum pernah diumumkan
// diposting ke channel update-apk. Penanda versi terakhir yang
// diumumkan disimpan di app_settings agar tidak dobel. Cara lama
// (posting saat admin menaikkan min_version_name di dashboard)
// dicabut — sumber kebenarannya kini rilis yang benar-benar
// terbit, tanpa langkah manual apa pun.

// GitHubAPIBase adalah var agar tes bisa mengarahkannya ke server
// tiruan.
var GitHubAPIBase = "https://api.github.com"

const subsenpaiRepo = "devcoderama/SubSenpai"
const discordUpdateNotifiedKey = "discord_update_notified"
const telegramUpdateNotifiedKey = "telegram_update_notified"

type ghRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Assets  []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

// latestReleaseVersion mengambil versi (tag tanpa awalan "v")
// dari rilis terbaru yang punya aset .apk — pola yang sama dengan
// halaman unduhan resmi. Semua rilis SubSenpai adalah pre-release
// sehingga flag itu TIDAK disaring; draf disaring.
func latestReleaseVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		GitHubAPIBase+"/repos/"+subsenpaiRepo+"/releases?per_page=10", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "SubSenpai-Worker")
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github releases: HTTP %d", resp.StatusCode)
	}
	var rels []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return "", err
	}
	for _, r := range rels {
		if r.Draft {
			continue
		}
		hasAPK := false
		for _, a := range r.Assets {
			if strings.HasSuffix(strings.ToLower(a.Name), ".apk") {
				hasAPK = true
				break
			}
		}
		if !hasAPK {
			continue
		}
		if v := strings.TrimPrefix(r.TagName, "v"); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("tidak ada rilis ber-APK")
}

// compareVersions membandingkan dua nama versi bertitik secara
// numerik per segmen: hasil <0 / 0 / >0.
func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var na, nb int
		if i < len(pa) {
			na, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			nb, _ = strconv.Atoi(pb[i])
		}
		if na != nb {
			if na < nb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// PostUpdateToDiscordIfNew mengumumkan rilis APK terbaru ke
// Discord bila versinya lebih baru dari penanda terakhir. No-op
// bila token/channel kosong. Kegagalan hanya dicatat di log —
// siklus worker tidak boleh terganggu.
func PostUpdateToDiscordIfNew(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) {
	discordOK := cfg.DiscordBotToken != "" && cfg.DiscordChannelUpdateAPK != ""
	telegramOK := cfg.TelegramBotToken != "" && cfg.TelegramGroupID != "" && cfg.TelegramTopicUpdateAPK != ""
	if !discordOK && !telegramOK {
		return
	}
	latest, err := latestReleaseVersion(ctx)
	if err != nil {
		log.Printf("discord update: cek rilis: %v", err)
		return
	}
	// Penanda per kanal (perbaikan 2026-11-24): dulu satu penanda
	// bersama — kanal yang dikonfigurasi belakangan (Telegram)
	// tidak pernah menerima versi yang sudah terlanjur diumumkan
	// di kanal lain (Discord), karena penandanya sudah terangkat.
	readMarker := func(key string) string {
		var v string
		_ = pool.QueryRow(ctx,
			`select value from app_settings where key = $1`, key).Scan(&v)
		return v
	}
	writeMarker := func(key string) {
		if _, err := pool.Exec(ctx, `
			insert into app_settings (key, value) values ($1, $2)
			on conflict (key) do update set value = excluded.value, updated_at = now()`,
			key, latest); err != nil {
			log.Printf("update notif: simpan penanda %s: %v", key, err)
		}
	}
	due := func(key string) bool {
		n := readMarker(key)
		return n == "" || compareVersions(latest, n) > 0
	}
	msg := "🚀 **Update APK SubSenpai v" + latest + "**\n\n" +
		"Versi terbaru sudah tersedia. Unduh di https://www.subsenpai.web.id"
	if discordOK && due(discordUpdateNotifiedKey) {
		if err := discord.New(cfg.DiscordBotToken).Send(ctx, cfg.DiscordChannelUpdateAPK, msg); err != nil {
			log.Printf("discord update: kirim gagal: %v", err)
		} else {
			writeMarker(discordUpdateNotifiedKey)
		}
	}
	if telegramOK && due(telegramUpdateNotifiedKey) {
		if err := telegram.New(cfg.TelegramBotToken).Send(ctx, cfg.TelegramGroupID,
			cfg.TelegramTopicUpdateAPK, telegram.ToHTML(msg)); err != nil {
			log.Printf("telegram update: kirim gagal: %v", err)
		} else {
			writeMarker(telegramUpdateNotifiedKey)
		}
	}
}
