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
	"subsenpai/backend/internal/telegram"
)

const discordJadwalLastDateKey = "discord_jadwal_last_date"
const telegramJadwalLastDateKey = "telegram_jadwal_last_date"

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
// berkala tidak memposting berulang. Kegagalan ambil jadwal tidak
// menandai hari itu — siklus berikutnya mencoba lagi.
func PostDailyScheduleToDiscord(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) {
	discordOK := cfg.DiscordBotToken != "" && cfg.DiscordChannelJadwal != ""
	telegramOK := cfg.TelegramBotToken != "" && cfg.TelegramGroupID != "" && cfg.TelegramTopicJadwal != ""
	if !discordOK && !telegramOK {
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
	// Penanda harian per kanal (perbaikan 2026-11-24, pola sama
	// dengan notifikasi update APK): kanal yang dikonfigurasi
	// belakangan tetap menerima jadwal hari ini walau kanal lain
	// sudah mempostingnya lebih dulu.
	readDay := func(key string) string {
		var v string
		_ = pool.QueryRow(ctx,
			`select value from app_settings where key = $1`, key).Scan(&v)
		return v
	}
	discordDue := discordOK && readDay(discordJadwalLastDateKey) != today
	telegramDue := telegramOK && readDay(telegramJadwalLastDateKey) != today
	if !discordDue && !telegramDue {
		return
	}
	day := hariIndo[now.Weekday()]

	anime, errA := sources.FetchKuronimeSchedule(ctx)
	donghua, errD := sources.FetchAnichinSchedule(ctx)
	if errA != nil {
		log.Printf("jadwal: ambil anime: %v", errA)
	}
	if errD != nil {
		log.Printf("jadwal: ambil donghua: %v", errD)
	}
	if errA != nil && errD != nil {
		return // dua-duanya gagal — coba lagi siklus berikutnya
	}

	// Susun blok kaya per judul (permintaan Rams 2026-11-24):
	// judul, judul lain, genre, studio, musim, sinopsis ringkas,
	// dan tautan nonton langsung ke pemutar episode terbaru —
	// metadata dari katalog, dipetakan lewat slug.
	collect := func(label string, groups []sources.ScheduleGroup) []string {
		var blocks []string
		for _, g := range groups {
			if g.Day != day {
				continue
			}
			for i, it := range g.Items {
				var m *jadwalMeta
				if meta, found := enrichJadwalItem(ctx, pool, it.Slug); found {
					m = &meta
				}
				blk := jadwalBlock(it, m)
				if i == 0 {
					blk = "**" + label + "**\n\n" + blk
				}
				blocks = append(blocks, blk)
			}
			break // hanya grup hari ini
		}
		return blocks
	}
	var blocks []string
	if errA == nil {
		blocks = append(blocks, collect("🎌 Anime", anime)...)
	}
	if errD == nil {
		blocks = append(blocks, collect("🐉 Donghua", donghua)...)
	}
	header := fmt.Sprintf("🗓️ **Jadwal Rilis — %s, %s**", day, now.Format("2 January 2006"))
	footer := "Selamat menonton di SubSenpai! 🍿 https://www.subsenpai.web.id"
	msgs := packJadwal(header, blocks, footer, 1700)

	markDay := func(key string) {
		if _, err := pool.Exec(ctx, `
			insert into app_settings (key, value) values ($1, $2)
			on conflict (key) do update set value = excluded.value, updated_at = now()`,
			key, today); err != nil {
			log.Printf("jadwal: simpan penanda %s: %v", key, err)
		}
	}
	sendAll := func(send func(string) error) error {
		for _, m := range msgs {
			if err := send(m); err != nil {
				return err
			}
		}
		return nil
	}
	if discordDue {
		dc := discord.New(cfg.DiscordBotToken)
		if err := sendAll(func(m string) error {
			return dc.Send(ctx, cfg.DiscordChannelJadwal, m)
		}); err != nil {
			log.Printf("discord jadwal: kirim gagal: %v", err)
		} else {
			markDay(discordJadwalLastDateKey)
			log.Printf("discord jadwal: jadwal %s terposting (%d pesan)", today, len(msgs))
		}
	}
	if telegramDue {
		// Telegram menerima bentuk berbeda dari Discord (permintaan
		// Rams 2026-11-24): bukan kemasan ringkasan, melainkan 1
		// pesan per judul dengan pratinjau tautan menyala, agar
		// poster tiap judul tampil sebagai kartu WebPage.
		tc := telegram.New(cfg.TelegramBotToken)
		if err := kirimJadwalTelegram(ctx, tc, cfg.TelegramGroupID,
			cfg.TelegramTopicJadwal, header, blocks, footer); err != nil {
			log.Printf("telegram jadwal: kirim gagal: %v", err)
		} else {
			markDay(telegramJadwalLastDateKey)
			log.Printf("telegram jadwal: jadwal %s terposting (%d pesan)", today, len(blocks)+2)
		}
	}
}

// kirimJadwalTelegram mengirim jadwal ke topik Telegram sebagai
// rangkaian pesan: header, lalu 1 pesan per judul dengan pratinjau
// menyala (halaman /nonton/ membawa og:image poster, jadi tiap
// judul tampil bergambar), lalu footer. Satu kegagalan membatalkan
// sisanya dan mengembalikan error — pemanggil tidak menandai hari
// itu, jadi siklus worker berikutnya mengulang dari awal (judul
// awal mungkin terkirim dua kali; diterima, sama seperti kemasan
// lama).
func kirimJadwalTelegram(ctx context.Context, tc *telegram.Client,
	groupID, topicID, header string, blocks []string, footer string) error {
	if err := tc.Send(ctx, groupID, topicID, telegram.ToHTML(header)); err != nil {
		return err
	}
	for _, b := range blocks {
		if err := tc.SendPreview(ctx, groupID, topicID, telegram.ToHTML(b)); err != nil {
			return err
		}
	}
	if footer != "" {
		if err := tc.SendPreview(ctx, groupID, topicID, telegram.ToHTML(footer)); err != nil {
			return err
		}
	}
	return nil
}

// jadwalMeta adalah metadata katalog untuk satu judul jadwal.
type jadwalMeta struct {
	TitleAlt  string
	Genres    []string
	Studio    string
	Season    string
	Year      int
	Synopsis  string
	EpisodeID string
	LatestNo  int
}

// enrichJadwalItem memetakan slug jadwal ke judul katalog:
// metadata lengkap + episode terbaru yang sudah tersedia (untuk
// tautan nonton langsung ke pemutar). Judul tersembunyi dan yang
// belum terpetakan dianggap tidak ditemukan.
func enrichJadwalItem(ctx context.Context, pool *pgxpool.Pool, slug string) (jadwalMeta, bool) {
	var m jadwalMeta
	var year *int
	err := pool.QueryRow(ctx, `
		select coalesce(t.title_alt, ''), coalesce(t.genres, '{}'),
		       coalesce(t.studio, ''), coalesce(t.season, ''),
		       t.year, coalesce(t.synopsis, ''),
		       coalesce((select e.id::text from episodes e
		                  where e.title_id = t.id and not e.is_hidden
		                  order by e.episode_no desc limit 1), ''),
		       coalesce((select max(e.episode_no) from episodes e
		                  where e.title_id = t.id and not e.is_hidden), 0)
		from titles t where t.slug = $1 and not t.is_hidden`, slug).
		Scan(&m.TitleAlt, &m.Genres, &m.Studio, &m.Season, &year,
			&m.Synopsis, &m.EpisodeID, &m.LatestNo)
	if err != nil {
		return jadwalMeta{}, false
	}
	if year != nil {
		m.Year = *year
	}
	return m, true
}

// jadwalBlock menyusun blok teks kaya untuk satu judul jadwal.
// Baris yang datanya kosong dilewati; judul di luar katalog
// (m == nil) tampil sebagai baris ringkas saja.
func jadwalBlock(it sources.ScheduleItem, m *jadwalMeta) string {
	var b strings.Builder
	head := "• **" + it.Title + "**"
	if it.Episode > 0 {
		head += fmt.Sprintf(" — Ep %d", it.Episode)
	}
	if it.Time != "" {
		head += " • " + it.Time
	}
	b.WriteString(head)
	if m == nil {
		return b.String()
	}
	if m.TitleAlt != "" {
		fmt.Fprintf(&b, "\nJudul lain: %s", m.TitleAlt)
	}
	if len(m.Genres) > 0 {
		fmt.Fprintf(&b, "\nGenre: %s", strings.Join(m.Genres, ", "))
	}
	var meta []string
	if m.Studio != "" {
		meta = append(meta, "Studio: "+m.Studio)
	}
	switch {
	case m.Season != "" && m.Year > 0:
		meta = append(meta, fmt.Sprintf("Musim: %s %d", kapital(m.Season), m.Year))
	case m.Season != "":
		meta = append(meta, "Musim: "+kapital(m.Season))
	case m.Year > 0:
		meta = append(meta, fmt.Sprintf("Tahun: %d", m.Year))
	}
	if len(meta) > 0 {
		b.WriteString("\n" + strings.Join(meta, " • "))
	}
	if m.Synopsis != "" {
		b.WriteString("\n" + potongRunes(m.Synopsis, 180))
	}
	if m.EpisodeID != "" {
		fmt.Fprintf(&b, "\n▶️ Nonton Ep %d: https://www.subsenpai.web.id/nonton/%s",
			m.LatestNo, m.EpisodeID)
	}
	return b.String()
}

// packJadwal mengemas blok-blok menjadi pesan-pesan yang tidak
// melebihi maxLen karakter (Discord menolak pesan > 2000).
// Header selalu membuka pesan pertama; footer menutup pesan
// terakhir bila muat, kalau tidak jadi pesan sendiri.
func packJadwal(header string, blocks []string, footer string, maxLen int) []string {
	parts := append([]string{header}, blocks...)
	var msgs []string
	cur := ""
	flush := func() {
		if cur != "" {
			msgs = append(msgs, cur)
			cur = ""
		}
	}
	for _, p := range parts {
		if cur == "" {
			cur = p
			continue
		}
		if len([]rune(cur))+2+len([]rune(p)) > maxLen {
			flush()
			cur = p
		} else {
			cur += "\n\n" + p
		}
	}
	if footer != "" {
		if cur != "" && len([]rune(cur))+2+len([]rune(footer)) <= maxLen {
			cur += "\n\n" + footer
		} else {
			flush()
			cur = footer
		}
	}
	flush()
	return msgs
}

// potongRunes memotong teks ke maksimal n rune + elipsis.
func potongRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

// kapital membuat huruf pertama string menjadi huruf besar
// (nilai musim ASCII: winter/spring/summer/fall).
func kapital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
