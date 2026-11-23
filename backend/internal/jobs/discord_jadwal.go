package jobs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/discord"
	"subsenpai/backend/internal/sources"
)

// hariIndo memetakan time.Weekday ke nama hari kanonik yang dipakai
// grup jadwal sumber ("Jum'at" sudah dinormalkan menjadi "Jumat").
var hariIndo = map[time.Weekday]string{
	time.Monday:    "Senin",
	time.Tuesday:   "Selasa",
	time.Wednesday: "Rabu",
	time.Thursday:  "Kamis",
	time.Friday:    "Jumat",
	time.Saturday:  "Sabtu",
	time.Sunday:    "Minggu",
}

// PostDailyScheduleToDiscord memposting jadwal rilis HARI INI
// (anime dari Kuronime + donghua dari Anichin) ke channel Discord
// #jadwal-rilis, sekali sehari sesudah pukul 05:00 WIB. Penanda
// tanggal terakhir disimpan di app_settings agar siklus worker
// 30-menit tidak memposting berulang. Kegagalan ambil jadwal tidak
// menandai hari itu — siklus berikutnya mencoba lagi.
func PostDailyScheduleToDiscord(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) {
	if cfg.DiscordBotToken == "" || cfg.DiscordChannelJadwal == "" {
		return
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	now := time.Now().In(loc)
	if now.Hour() < 5 {
		return
	}
	today := now.Format("2006-01-02")
	var last string
	_ = pool.QueryRow(ctx,
		`select value from app_settings where key = 'discord_jadwal_last_date'`).Scan(&last)
	if last == today {
		return
	}
	day := hariIndo[now.Weekday()]

	var b strings.Builder
	fmt.Fprintf(&b, "🗓️ **Jadwal Rilis — %s, %s**\n", day, now.Format("2 January 2006"))

	section := func(label string, groups []sources.ScheduleGroup, ferr error) bool {
		if ferr != nil {
			log.Printf("discord jadwal: ambil %s: %v", label, ferr)
			return false
		}
		for _, g := range groups {
			if g.Day != day {
				continue
			}
			if len(g.Items) == 0 {
				return true
			}
			fmt.Fprintf(&b, "\n**%s**\n", label)
			for _, it := range g.Items {
				line := "• " + it.Title
				if it.Time != "" {
					line += " — " + it.Time
				}
				if it.Episode > 0 {
					line += fmt.Sprintf(" (Ep %d)", it.Episode)
				}
				b.WriteString(line + "\n")
			}
			return true
		}
		return true
	}

	anime, errA := sources.FetchKuronimeSchedule(ctx)
	donghua, errD := sources.FetchAnichinSchedule(ctx)
	okA := section("🎌 Anime", anime, errA)
	okD := section("🐉 Donghua", donghua, errD)
	if !okA && !okD {
		return // dua-duanya gagal — coba lagi siklus berikutnya
	}
	b.WriteString("\nSelamat menonton di SubSenpai! 🍿 https://www.subsenpai.web.id")

	if err := discord.New(cfg.DiscordBotToken).Send(ctx, cfg.DiscordChannelJadwal, b.String()); err != nil {
		log.Printf("discord jadwal: kirim gagal: %v", err)
		return
	}
	if _, err := pool.Exec(ctx, `
		insert into app_settings (key, value) values ('discord_jadwal_last_date', $1)
		on conflict (key) do update set value = excluded.value, updated_at = now()`, today); err != nil {
		log.Printf("discord jadwal: simpan penanda: %v", err)
		return
	}
	log.Printf("discord jadwal: jadwal %s terposting", today)
}
