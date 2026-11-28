package jobs

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/discord"
	"subsenpai/backend/internal/sources"
	"subsenpai/backend/internal/telegram"
)

// Pengiriman jadwal rilis per judul per slot waktu (perbaikan
// 2026-11-28, permintaan Rams): tiap judul dikirim SATU pesan,
// tepat sesudah jam rilisnya (WIB) — bukan lagi digest sehari penuh
// sekali jalan. Penanda per judul per kanal di tabel jadwal_notified
// (migrasi 0027): kirim yang gagal tidak menandai apa pun, jadi
// siklus berikutnya hanya mengulang judul yang gagal itu, bukan
// sehari penuh dari awal. Header harian memakai slug "__header__"
// di tabel yang sama.
const jadwalHeaderSlug = "__header__"

const (
	jadwalKanalDiscord   = "discord"
	jadwalKanalTelegram  = "telegram"
	jadwalKanalTelegram2 = "telegram2"
)

// Jeda antar pesan ke kanal yang sama — Telegram Bot API membatasi
// kirim ke satu grup (~1 pesan/detik); tanpa jeda, burst header +
// puluhan judul langsung kena 429 (kejadian 2026-11-28: grup 1
// gagal 2 hari berturut-turut karena tiap siklus mengulang dari
// awal). Discord ikut diberi jeda yang sama.
const jadwalJedaAntarPesan = 1200 * time.Millisecond

// Kunci app_settings untuk id pesan header jadwal yang sedang
// tersemat di tiap grup Telegram (permintaan Rams 2026-11-25:
// header hari ini di-pin, header kemarin dilepas).
const telegramJadwalPinKey = "telegram_jadwal_pin_id"
const telegramJadwalPinKey2 = "telegram_jadwal_pin_id_2"

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

// jadwalJamRe mengenali "05:00" / "5:00" / "16:26".
var jadwalJamRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)

// parseJadwalJam mengubah Time jadwal ("05:00") menjadi jam+menit.
// "Sudah Rilis", "??", "" dan format lain -> ok=false (sudah
// terbit / jam tak diketahui: dikirim di batch pagi, bukan
// dijadwalkan per slot).
func parseJadwalJam(s string) (int, int, bool) {
	m := jadwalJamRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, 0, false
	}
	hh, _ := strconv.Atoi(m[1])
	mm, _ := strconv.Atoi(m[2])
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, false
	}
	return hh, mm, true
}

// jadwalItem adalah satu judul hari ini yang siap dikirim.
// jam = waktu rilis hari ini (WIB); nol = sudah terbit / jam tak
// diketahui, dikirim di batch pagi pertama sesudah 05:00.
type jadwalItem struct {
	slug  string
	pesan string
	jam   time.Time
}

// susunJadwalHariIni mengubah grup jadwal mentah menjadi daftar
// item hari ini, diurutkan berdasarkan jam rilis (yang tanpa jam
// dulu), siap dipilah mana yang sudah tiba waktunya. enrich
// dipisah sebagai parameter agar logika urut/pilah bisa diuji
// tanpa database.
func susunJadwalHariIni(day string, now time.Time, loc *time.Location,
	enrich func(string) (jadwalMeta, bool),
	anime, donghua []sources.ScheduleGroup) []jadwalItem {
	kumpul := func(label string, groups []sources.ScheduleGroup) []jadwalItem {
		var out []jadwalItem
		for _, g := range groups {
			if g.Day != day {
				continue
			}
			for _, it := range g.Items {
				var m *jadwalMeta
				if meta, found := enrich(it.Slug); found {
					m = &meta
				}
				ji := jadwalItem{
					slug:  it.Slug,
					pesan: "**" + label + "**\n\n" + jadwalBlock(it, m),
				}
				if hh, mm, ok := parseJadwalJam(it.Time); ok {
					ji.jam = time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, loc)
				}
				out = append(out, ji)
			}
			break // hanya grup hari ini
		}
		return out
	}
	items := append(kumpul("🎌 Anime", anime), kumpul("🐉 Donghua", donghua)...)
	sort.Slice(items, func(i, j int) bool {
		iz, jz := items[i].jam.IsZero(), items[j].jam.IsZero()
		if iz != jz {
			return iz
		}
		if !items[i].jam.Equal(items[j].jam) {
			return items[i].jam.Before(items[j].jam)
		}
		return items[i].slug < items[j].slug
	})
	return items
}

// jadwalDue mengembalikan item yang jam rilisnya sudah tiba
// (atau tanpa jam — sudah terbit / tak diketahui).
func jadwalDue(items []jadwalItem, now time.Time) []jadwalItem {
	var out []jadwalItem
	for _, it := range items {
		if it.jam.IsZero() || !now.Before(it.jam) {
			out = append(out, it)
		}
	}
	return out
}

// jadwalSudahTerkirim memeriksa penanda per judul per kanal.
func jadwalSudahTerkirim(ctx context.Context, pool *pgxpool.Pool, kanal, hari, slug string) bool {
	var ok bool
	_ = pool.QueryRow(ctx, `
		select exists(select 1 from jadwal_notified
		              where channel = $1 and day = $2 and slug = $3)`,
		kanal, hari, slug).Scan(&ok)
	return ok
}

// jadwalTandaiTerkirim mencatat satu judul (atau header) sebagai
// sudah terkirim ke satu kanal. Dipanggil hanya sesudah kirim
// berhasil — gagal kirim tidak menandai apa pun.
func jadwalTandaiTerkirim(ctx context.Context, pool *pgxpool.Pool, kanal, hari, slug string) {
	if _, err := pool.Exec(ctx, `
		insert into jadwal_notified (channel, day, slug) values ($1, $2, $3)
		on conflict do nothing`, kanal, hari, slug); err != nil {
		log.Printf("jadwal: simpan penanda %s %s: %v", kanal, slug, err)
	}
}

// jadwalRetryAfterRe membaca "retry_after":N dari badan error 429
// (Telegram: {"retry_after":42}; Discord: {"retry_after":1.5}).
var jadwalRetryAfterRe = regexp.MustCompile(`"retry_after"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)

// retryAfterDariError mengembalikan lama tunggu dari error 429,
// atau 0 bila errornya bukan 429 / tak membawa angka.
func retryAfterDariError(err error) time.Duration {
	if err == nil {
		return 0
	}
	m := jadwalRetryAfterRe.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	f, ferr := strconv.ParseFloat(m[1], 64)
	if ferr != nil || f <= 0 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

// jedaJadwal menunggu antar pesan ke kanal yang sama, batal bila
// konteks dibatalkan.
func jedaJadwal(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(jadwalJedaAntarPesan):
	}
}

// kirimJadwalSekali mengirim satu pesan: gagal (mis. 429) -> tunggu
// retry_after (atau 10 dtk bila tak diketahui) lalu coba sekali
// lagi untuk pesan itu saja. Pesan yang ditolak 429 tidak pernah
// terkirim, jadi ulang sekali aman dari dobel. Gagal kedua ->
// error; pemanggil menghentikan kanal itu di siklus ini (judulnya
// tetap tak tertandai, dicoba lagi siklus berikutnya).
func kirimJadwalSekali(ctx context.Context, kirim func(context.Context) error) error {
	if err := kirim(ctx); err == nil {
		return nil
	} else {
		tunggu := retryAfterDariError(err)
		if tunggu <= 0 {
			tunggu = 10 * time.Second
		}
		log.Printf("jadwal: kirim gagal, tunggu %s lalu coba lagi: %v", tunggu, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(tunggu):
		}
		return kirim(ctx)
	}
}

// kirimJadwalJudulTelegram mengirim satu judul sebagai satu pesan
// dengan pratinjau menyala, agar posternya tampil sebagai kartu.
func kirimJadwalJudulTelegram(ctx context.Context, tc *telegram.Client, groupID, topicID, pesan string) error {
	return tc.SendPreview(ctx, groupID, topicID, telegram.ToHTML(pesan))
}

// kirimJadwalJudulDiscord mengirim satu judul sebagai satu pesan.
func kirimJadwalJudulDiscord(ctx context.Context, dc *discord.Client, channelID, pesan string) error {
	return dc.Send(ctx, channelID, pesan)
}

// jadwalTujuan adalah satu kanal tujuan (Discord / Telegram grup 1 /
// Telegram grup 2) dengan cara kirimnya masing-masing.
type jadwalTujuan struct {
	kanal       string
	kirimHeader func(context.Context, string) (int64, error) // id untuk pin (Telegram); Discord 0
	kirimJudul  func(context.Context, string) error
	pin         func(context.Context, int64) // nil bila kanal tak di-pin
}

// PostJadwalRilis mengirim jadwal rilis HARI INI (anime dari
// Kuronime + donghua dari Anichin) satu pesan per judul, tepat
// sesudah jam rilisnya (WIB). Dipanggil berkala (ticker 5 menit
// worker); tiap siklus hanya mengirim judul yang jamnya sudah tiba
// dan belum tertandai di jadwal_notified. Header harian dikirim
// sekali sesudah pukul 05:00 WIB dan di-pin di Telegram.
func PostJadwalRilis(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) {
	discordOK := cfg.DiscordBotToken != "" && cfg.DiscordChannelJadwal != ""
	telegramOK := cfg.TelegramBotToken != "" && cfg.TelegramGroupID != "" && cfg.TelegramTopicJadwal != ""
	telegram2OK := cfg.TelegramBotToken != "" && cfg.TelegramGroupID2 != "" && cfg.TelegramTopicJadwal2 != ""
	if !discordOK && !telegramOK && !telegram2OK {
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
	// Bersihkan penanda lama (hemat tabel; 7 hari cukup untuk
	// menahan kirim ganda bila worker lama mati).
	if _, err := pool.Exec(ctx, `delete from jadwal_notified where day < $1::date - 7`, today); err != nil {
		log.Printf("jadwal: bersihkan penanda lama: %v", err)
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
	enrich := func(slug string) (jadwalMeta, bool) {
		return enrichJadwalItem(ctx, pool, slug)
	}
	var animeGroups, donghuaGroups []sources.ScheduleGroup
	if errA == nil {
		animeGroups = anime
	}
	if errD == nil {
		donghuaGroups = donghua
	}
	items := susunJadwalHariIni(day, now, loc, enrich, animeGroups, donghuaGroups)
	due := jadwalDue(items, now)

	info := fmt.Sprintf("Hari ini ada %d judul — tiap judul dikirim sendiri sesudah jam rilisnya.", len(items))
	if len(items) == 0 {
		info = "Belum ada judul terjadwal hari ini."
	}
	header := fmt.Sprintf("🗓️ **Jadwal Rilis — %s, %s**", day, now.Format("2 January 2006"))
	footer := "Selamat menonton di SubSenpai! 🍿 https://www.subsenpai.web.id"
	headerPesan := header + "\n" + info + "\n\n" + footer

	var tujuan []jadwalTujuan
	if discordOK {
		dc := discord.New(cfg.DiscordBotToken)
		tujuan = append(tujuan, jadwalTujuan{
			kanal: jadwalKanalDiscord,
			kirimHeader: func(ctx context.Context, pesan string) (int64, error) {
				return 0, dc.Send(ctx, cfg.DiscordChannelJadwal, pesan)
			},
			kirimJudul: func(ctx context.Context, pesan string) error {
				return kirimJadwalJudulDiscord(ctx, dc, cfg.DiscordChannelJadwal, pesan)
			},
		})
	}
	if telegramOK {
		tc := telegram.New(cfg.TelegramBotToken)
		tujuan = append(tujuan, jadwalTujuan{
			kanal: jadwalKanalTelegram,
			kirimHeader: func(ctx context.Context, pesan string) (int64, error) {
				return tc.SendID(ctx, cfg.TelegramGroupID, cfg.TelegramTopicJadwal, telegram.ToHTML(pesan))
			},
			kirimJudul: func(ctx context.Context, pesan string) error {
				return kirimJadwalJudulTelegram(ctx, tc, cfg.TelegramGroupID, cfg.TelegramTopicJadwal, pesan)
			},
			pin: func(ctx context.Context, id int64) {
				pinJadwalHeader(ctx, pool, tc, cfg.TelegramGroupID, telegramJadwalPinKey, id)
			},
		})
	}
	if telegram2OK {
		// Grup Telegram kedua (permintaan Rams 2026-11-25): bot
		// yang sama, grup + topik berbeda, isi pesan sama persis.
		// Penandanya sendiri — gagal di grup kedua tidak menahan
		// grup pertama.
		tc2 := telegram.New(cfg.TelegramBotToken)
		tujuan = append(tujuan, jadwalTujuan{
			kanal: jadwalKanalTelegram2,
			kirimHeader: func(ctx context.Context, pesan string) (int64, error) {
				return tc2.SendID(ctx, cfg.TelegramGroupID2, cfg.TelegramTopicJadwal2, telegram.ToHTML(pesan))
			},
			kirimJudul: func(ctx context.Context, pesan string) error {
				return kirimJadwalJudulTelegram(ctx, tc2, cfg.TelegramGroupID2, cfg.TelegramTopicJadwal2, pesan)
			},
			pin: func(ctx context.Context, id int64) {
				pinJadwalHeader(ctx, pool, tc2, cfg.TelegramGroupID2, telegramJadwalPinKey2, id)
			},
		})
	}

	for _, t := range tujuan {
		if !jadwalSudahTerkirim(ctx, pool, t.kanal, today, jadwalHeaderSlug) {
			var headerID int64
			err := kirimJadwalSekali(ctx, func(ctx context.Context) error {
				id, err := t.kirimHeader(ctx, headerPesan)
				if err == nil {
					headerID = id
				}
				return err
			})
			if err != nil {
				log.Printf("jadwal %s: header gagal: %v", t.kanal, err)
			} else {
				jadwalTandaiTerkirim(ctx, pool, t.kanal, today, jadwalHeaderSlug)
				if t.pin != nil {
					t.pin(ctx, headerID)
				}
			}
		}
		for _, it := range due {
			if jadwalSudahTerkirim(ctx, pool, t.kanal, today, it.slug) {
				continue
			}
			if err := kirimJadwalSekali(ctx, func(ctx context.Context) error {
				return t.kirimJudul(ctx, it.pesan)
			}); err != nil {
				log.Printf("jadwal %s: %s gagal: %v", t.kanal, it.slug, err)
				break // berhenti untuk kanal ini di siklus ini
			}
			jadwalTandaiTerkirim(ctx, pool, t.kanal, today, it.slug)
			jedaJadwal(ctx)
		}
	}
}

// pinJadwalHeader menyematkan header jadwal hari ini di topik
// Telegram dan melepas sematan header kemarin (id-nya tersimpan di
// app_settings per grup). Pin hanya hiasan: gagal pin/unpin
// dicatat di log, tidak menggagalkan penanda hari. Bot harus admin
// grup dengan hak pin; bila haknya kurang, pinChatMessage ditolak
// Bot API dan jadwal tetap terkirim seperti biasa.
func pinJadwalHeader(ctx context.Context, pool *pgxpool.Pool, tc *telegram.Client,
	chatID, pinKey string, headerID int64) {
	if headerID <= 0 {
		return
	}
	var prev string
	_ = pool.QueryRow(ctx,
		`select value from app_settings where key = $1`, pinKey).Scan(&prev)
	if prev != "" && prev != strconv.FormatInt(headerID, 10) {
		if id, err := strconv.ParseInt(strings.TrimSpace(prev), 10, 64); err == nil && id > 0 {
			if err := tc.Unpin(ctx, chatID, id); err != nil {
				log.Printf("telegram jadwal: lepas pin lama gagal: %v", err)
			}
		}
	}
	if err := tc.Pin(ctx, chatID, headerID); err != nil {
		log.Printf("telegram jadwal: pin gagal: %v", err)
		return
	}
	if _, err := pool.Exec(ctx, `
		insert into app_settings (key, value) values ($1, $2)
		on conflict (key) do update set value = excluded.value, updated_at = now()`,
		pinKey, strconv.FormatInt(headerID, 10)); err != nil {
		log.Printf("telegram jadwal: simpan id pin %s: %v", pinKey, err)
	}
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
