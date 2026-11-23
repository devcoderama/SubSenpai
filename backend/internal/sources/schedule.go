package sources

import (
	"context"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// Jadwal rilis (permintaan Rams 2026-11-23): halaman Lainnya aplikasi
// menampilkan jadwal tayang per kategori — anime dari halaman jadwal
// Kuronime, donghua dari halaman schedule Anichin. Keduanya memakai
// format kartu yang mirip: per hari, tiap entri punya jam rilis,
// nomor episode terakhir, judul, dan poster.
//
// ScheduleItem adalah satu entri jadwal. Time berisi "16:26" untuk
// jam tayang, "Sudah Rilis" bila episode hari itu sudah terbit, dan
// "??" untuk bagian Update Acak Kuronime.
type ScheduleItem struct {
	Title   string
	Slug    string
	Time    string
	Episode int
	Poster  string
}

// ScheduleGroup adalah daftar entri untuk satu hari ("Senin" ..
// "Minggu") atau bagian khusus ("Update Acak").
type ScheduleGroup struct {
	Day   string
	Items []ScheduleItem
}

// schedDays adalah urutan hari kanonik (label tanpa apostrof agar
// mudah dicocokkan aplikasi: "Jum'at" → "Jumat").
var schedDays = []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}

func schedCanonDay(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "'", "")
	for _, d := range schedDays {
		if strings.EqualFold(s, d) || strings.HasPrefix(strings.ToLower(s), strings.ToLower(d)) {
			return d
		}
	}
	return ""
}

// sortSchedGroups mengurutkan grup: Senin..Minggu lalu bagian lain
// (Update Acak) di akhir, mengikuti urutan kemunculan aslinya.
func sortSchedGroups(groups []ScheduleGroup) []ScheduleGroup {
	rank := func(day string) int {
		for i, d := range schedDays {
			if day == d {
				return i
			}
		}
		return len(schedDays)
	}
	for i := 0; i < len(groups); i++ {
		for j := i + 1; j < len(groups); j++ {
			if rank(groups[j].Day) < rank(groups[i].Day) {
				groups[i], groups[j] = groups[j], groups[i]
			}
		}
	}
	return groups
}

var (
	schedTagRe     = regexp.MustCompile(`(?is)<[^>]+>`)
	schedAnchorRe  = regexp.MustCompile(`(?is)<a\s[^>]*>.*?</a>`)
	schedH3Re      = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>`)
	schedImgRe     = regexp.MustCompile(`(?is)<img\s[^>]*>`)
	schedTimeEpRe  = regexp.MustCompile(`(?i)(jam\s+(\d{1,2}:\d{2})|sudah rilis|\?\?)\s+(\d+)\b`)
	schedAttrDQ    = regexp.MustCompile(`(?i)([a-zA-Z-]+)\s*=\s*"([^"]*)"`)
	schedAttrSQ    = regexp.MustCompile(`(?i)([a-zA-Z-]+)\s*=\s*'([^']*)'`)
	schedAttrUQ    = regexp.MustCompile(`(?i)([a-zA-Z-]+)\s*=\s*([^\s>]+)`)
	schedSVIPRe    = regexp.MustCompile(`^\[[^\]]+\]\s*`)
	schedAnichinBx = regexp.MustCompile(`(?i)schedulepage\s+sch_(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`)
	schedEpxRe     = regexp.MustCompile(`(?is)<span[^>]*class\s*=\s*["'][^"']*epx[^"']*["'][^>]*>(.*?)</span>`)
	schedSbRe      = regexp.MustCompile(`(?is)<span[^>]*class\s*=\s*["'][^"']*\bsb\b[^"']*["'][^>]*>\s*(\d+)\s*</span>`)
)

func schedStripTags(s string) string {
	return html.UnescapeString(strings.Join(strings.Fields(schedTagRe.ReplaceAllString(s, " ")), " "))
}

// schedAttr membaca atribut dari tag pembuka (petik ganda, petik
// tunggal, atau tanpa petik — HTML Kuronime kadang tanpa petik).
func schedAttr(tag, name string) string {
	for _, re := range []*regexp.Regexp{schedAttrDQ, schedAttrSQ, schedAttrUQ} {
		for _, m := range re.FindAllStringSubmatch(tag, -1) {
			if strings.EqualFold(m[1], name) {
				return html.UnescapeString(m[2])
			}
		}
	}
	return ""
}

// schedPosterURL menilai satu nilai atribut kandidat: tampak
// seperti foto poster sungguhan bila berupa URL http(s)/relatif
// dan BUKAN placeholder (data:), ikon SVG tema, atau gambar
// tombol play tema (controls-play.svg).
func schedPosterURL(v string) bool {
	if v == "" || strings.HasPrefix(v, "data:") {
		return false
	}
	lv := strings.ToLower(v)
	if strings.HasSuffix(lv, ".svg") || strings.Contains(lv, "controls-play") {
		return false
	}
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "/")
}

// schedPoster mengambil URL poster dari blok kartu: memeriksa
// SEMUA <img> di blok (kartu Kuronime kadang memuat gambar
// overlay tema/play lebih dulu) dan untuk tiap gambar mencoba
// data-lazy-src, data-src, srcset (URL pertama), lalu src —
// menerima hanya URL yang tampak seperti foto poster. Alasan:
// 2026-11-23 produksi mengembalikan controls-play.svg (ikon
// tombol play tema animenonton) sebagai poster jadwal anime,
// padahal data-lazy-src kartu memuat poster .jpg yang benar.
func schedPoster(block string) string {
	for _, img := range schedImgRe.FindAllString(block, -1) {
		for _, name := range []string{"data-lazy-src", "data-src"} {
			if v := schedAttr(img, name); schedPosterURL(v) {
				return v
			}
		}
		if ss := schedAttr(img, "srcset"); ss != "" {
			if v := strings.Fields(ss)[0]; schedPosterURL(v) {
				return v
			}
		}
		if v := schedAttr(img, "src"); schedPosterURL(v) {
			return v
		}
	}
	return ""
}

// schedOpenTag memotong tag pembuka <a ...> dari blok anchor.
func schedOpenTag(block string) string {
	if i := strings.Index(block, ">"); i >= 0 {
		return block[:i+1]
	}
	return block
}

// ParseKuronimeSchedule mem-parsing halaman /jadwal-rilis/ Kuronime:
// bagian-bagian dipisah heading <h3> berisi nama hari atau
// "Update Acak"; tiap entri adalah kartu <a href="/anime/<slug>">
// berisi poster + teks "Jam HH:MM <ep> <Judul>" / "Sudah Rilis
// <ep> <Judul>" / "?? <ep> <Judul>". Terverifikasi dari halaman
// aslinya 2026-11-23 (struktur h3 + kartu anchor bersaudara).
func ParseKuronimeSchedule(page string) []ScheduleGroup {
	heads := schedH3Re.FindAllStringSubmatchIndex(page, -1)
	var groups []ScheduleGroup
	for hi, h := range heads {
		label := schedStripTags(page[h[2]:h[3]])
		day := schedCanonDay(label)
		isAcak := strings.HasPrefix(strings.ToLower(label), "update acak")
		if day == "" && !isAcak {
			continue
		}
		if isAcak {
			day = "Update Acak"
		}
		start := h[1]
		end := len(page)
		if hi+1 < len(heads) {
			end = heads[hi+1][0]
		}
		section := page[start:end]
		g := ScheduleGroup{Day: day}
		for _, ab := range schedAnchorRe.FindAllString(section, -1) {
			open := schedOpenTag(ab)
			href := schedAttr(open, "href")
			ai := strings.Index(href, "/anime/")
			if ai < 0 {
				continue
			}
			slug := strings.TrimSuffix(href[ai+len("/anime/"):], "/")
			if slug == "" || strings.Contains(slug, "/") {
				continue
			}
			m := schedTimeEpRe.FindStringSubmatch(schedStripTags(ab))
			if m == nil {
				continue
			}
			tm := "??"
			if m[2] != "" {
				tm = m[2]
			} else if strings.HasPrefix(strings.ToLower(m[1]), "sudah") {
				tm = "Sudah Rilis"
			}
			ep, _ := strconv.Atoi(m[3])
			title := schedAttr(open, "title")
			if title == "" {
				continue
			}
			g.Items = append(g.Items, ScheduleItem{
				Title:   title,
				Slug:    slug,
				Time:    tm,
				Episode: ep,
				Poster:  schedPoster(ab),
			})
		}
		if len(g.Items) > 0 {
			groups = append(groups, g)
		}
	}
	return sortSchedGroups(groups)
}

var schedAnichinDayID = map[string]string{
	"monday": "Senin", "tuesday": "Selasa", "wednesday": "Rabu",
	"thursday": "Kamis", "friday": "Jumat", "saturday": "Sabtu",
	"sunday": "Minggu",
}

// ParseAnichinSchedule mem-parsing anichin.moe/schedule/: kotak per
// hari berclass "schedulepage sch_<day>"; entri kartu .bsx berisi
// span.epx ("Jam HH:MM"), span.sb (nomor episode), poster, dan
// tautan /<slug>/ dengan atribut title. Terverifikasi dari halaman
// aslinya 2026-11-23.
func ParseAnichinSchedule(page string) []ScheduleGroup {
	boxes := schedAnichinBx.FindAllStringSubmatchIndex(page, -1)
	var groups []ScheduleGroup
	for bi, b := range boxes {
		day := schedAnichinDayID[strings.ToLower(page[b[2]:b[3]])]
		start := b[0]
		end := len(page)
		if bi+1 < len(boxes) {
			end = boxes[bi+1][0]
		}
		section := page[start:end]
		g := ScheduleGroup{Day: day}
		for _, ab := range schedAnchorRe.FindAllString(section, -1) {
			open := schedOpenTag(ab)
			href := schedAttr(open, "href")
			slug := strings.Trim(href, "/")
			if slug == "" || strings.Contains(slug, "/") || strings.Contains(slug, ".") {
				continue
			}
			em := schedEpxRe.FindStringSubmatch(ab)
			if em == nil {
				continue
			}
			tm := schedStripTags(em[1])
			if strings.HasPrefix(strings.ToLower(tm), "jam ") {
				tm = strings.TrimSpace(tm[4:])
			}
			ep := 0
			if sm := schedSbRe.FindStringSubmatch(ab); sm != nil {
				ep, _ = strconv.Atoi(sm[1])
			}
			title := schedSVIPRe.ReplaceAllString(schedAttr(open, "title"), "")
			if title == "" {
				continue
			}
			poster := schedPoster(ab)
			if poster != "" && strings.HasPrefix(poster, "/") {
				poster = "https://anichin.moe" + poster
			}
			g.Items = append(g.Items, ScheduleItem{
				Title:   title,
				Slug:    slug,
				Time:    tm,
				Episode: ep,
				Poster:  poster,
			})
			if len(g.Items) >= 80 {
				break
			}
		}
		if len(g.Items) > 0 {
			groups = append(groups, g)
		}
	}
	return sortSchedGroups(groups)
}

// FetchKuronimeSchedule mengambil + mem-parsing jadwal anime.
func FetchKuronimeSchedule(ctx context.Context) ([]ScheduleGroup, error) {
	page, err := Fetch(ctx, "https://kuronime.sbs/jadwal-rilis/")
	if err != nil {
		return nil, err
	}
	return ParseKuronimeSchedule(page), nil
}

// FetchAnichinSchedule mengambil + mem-parsing jadwal donghua.
func FetchAnichinSchedule(ctx context.Context) ([]ScheduleGroup, error) {
	page, err := Fetch(ctx, "https://anichin.moe/schedule/")
	if err != nil {
		return nil, err
	}
	return ParseAnichinSchedule(page), nil
}
