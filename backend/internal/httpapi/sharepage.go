package httpapi

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Halaman bagikan publik: GET /share/{id}
//
// Dirender di SERVER (bukan oleh JavaScript) supaya perayap
// WhatsApp / Telegram / X — yang tidak menjalankan JS — melihat tag
// Open Graph berisi poster + sinopsis judul ini saat tautannya
// dibagikan. Netlify mem-proxy domain resmi
// https://subsenpai.web.id/judul/{id} ke endpoint ini (rewrite
// status 200), jadi URL publiknya tetap domain resmi sementara isi
// halamannya datang dari sini. Halaman yang sama juga yang dibuka
// manusia di peramban: poster, info, dan tombol membuka aplikasi.

var shareIDRe = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

type sharePageData struct {
	Title       string
	SEOTitle    string // judul lengkap <title>/og:title: nama + tahun + tipe + "Subtitle Indonesia"
	OGType      string // video.movie (film) / video.tv_show (anime/donghua)
	JSONLD      template.JS // data terstruktur schema.org — template.JS agar tidak di-escape JS di dalam <script>

	Description string
	PageURL     string
	ImageURL    string
	PosterURL   string
	MetaLine    string
	GenresLine  string
	Synopsis    string
	IntentURL   template.URL
	SchemeURL   template.JS
	ID          string
	Found       bool
}

var shareTpl = template.Must(template.New("share").Parse(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.SEOTitle}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.PageURL}}">
<meta property="og:type" content="{{.OGType}}">
<meta property="og:site_name" content="SubSenpai">
<meta property="og:title" content="{{.SEOTitle}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.PageURL}}">
{{if .ImageURL}}<meta property="og:image" content="{{.ImageURL}}">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:image" content="{{.ImageURL}}">{{end}}
<meta name="twitter:title" content="{{.SEOTitle}}">
<meta name="twitter:description" content="{{.Description}}">
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90' fill='%23E50914' font-weight='900'>S</text></svg>">
{{if .JSONLD}}<script type="application/ld+json">{{.JSONLD}}</script>{{end}}
<style>
  body{background:#141414;color:#fff;font-family:system-ui,'Segoe UI',sans-serif;line-height:1.6;margin:0;min-height:100vh}
  .wrap{max-width:880px;margin:0 auto;padding:0 4%}
  header{background:#0b0b0b;border-bottom:1px solid #2d2d2d}
  .bar{padding:14px 0}
  .wordmark{color:#E50914;font-weight:900;font-size:1.5rem;letter-spacing:.06em;text-transform:uppercase;text-decoration:none}
  main{padding:38px 0 60px}
  .detail{display:flex;gap:26px;align-items:flex-start}
  @media (max-width:640px){.detail{flex-direction:column;align-items:center;text-align:center}}
  .poster{flex:0 0 auto;width:220px;max-width:62vw;aspect-ratio:2/3;border-radius:10px;overflow:hidden;background:#232323;box-shadow:0 18px 44px rgba(0,0,0,.55);position:relative;cursor:pointer}
  .poster img{width:100%;height:100%;object-fit:cover;display:block}
  .play{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;background:rgba(0,0,0,.22)}
  .play span{width:64px;height:64px;border-radius:50%;background:rgba(229,9,20,.92);display:flex;align-items:center;justify-content:center;font-size:26px;padding-left:5px;box-shadow:0 8px 24px rgba(0,0,0,.5)}
  .info{flex:1;min-width:0}
  .info h1{font-size:clamp(1.6rem,4.6vw,2.4rem);line-height:1.2;font-weight:900;margin:0}
  .meta{color:#b3b3b3;margin:10px 0 4px}
  .genres{color:#e5e5e5;font-size:.95rem}
  .syn{color:#d7d7d7;margin:16px 0 26px;display:-webkit-box;-webkit-line-clamp:4;-webkit-box-orient:vertical;overflow:hidden}
  .stickybar{display:none}
  @media (max-width:640px){
    .stickybar{display:flex;gap:10px;position:fixed;left:0;right:0;bottom:0;padding:12px 4% calc(12px + env(safe-area-inset-bottom));background:rgba(11,11,11,.97);border-top:1px solid #2d2d2d;z-index:10}
    .stickybar .btn{flex:1;text-align:center;padding:13px 8px;margin:0}
    main{padding-bottom:110px}
  }
  .btn{display:inline-block;background:#E50914;color:#fff;font-weight:700;padding:13px 28px;border-radius:6px;text-decoration:none}
  .btn.ghost{background:rgba(255,255,255,.14);margin-left:8px}
  .note{color:#b3b3b3;font-size:.88rem;margin-top:14px}
  .err{background:#1f1f1f;border-radius:10px;padding:28px;text-align:center;color:#b3b3b3}
  .err b{color:#fff;display:block;font-size:1.15rem;margin-bottom:6px}
</style>
</head>
<body>
<header><div class="wrap bar"><a class="wordmark" href="https://subsenpai.web.id/">SubSenpai</a></div></header>
<main class="wrap">
{{if .Found}}
  <div class="detail">
    {{if .PosterURL}}<div class="poster" id="playBtn" role="button" tabindex="0" title="Buka di aplikasi"><img src="{{.PosterURL}}" alt="{{.Title}}"><div class="play"><span>▶</span></div></div>{{end}}
    <div class="info">
      <h1>{{.Title}}</h1>
      {{if .MetaLine}}<div class="meta">{{.MetaLine}}</div>{{end}}
      {{if .GenresLine}}<div class="genres">{{.GenresLine}}</div>{{end}}
      {{if .Synopsis}}<p class="syn">{{.Synopsis}}</p>{{end}}
      <a class="btn" id="openBtn" href="{{.IntentURL}}">▶ Buka di Aplikasi SubSenpai</a>
      <a class="btn ghost" href="https://subsenpai.web.id/">Download Aplikasi</a>
      <div class="note">Ketuk tombol merah atau poster untuk membuka episode ini di aplikasi. Belum punya aplikasinya? Download dulu — gratis.</div>
    </div>
  </div>
  <div class="stickybar">
    <a class="btn" id="openBtn2" href="{{.IntentURL}}">▶ Buka di Aplikasi</a>
    <a class="btn ghost" href="https://subsenpai.web.id/">Unduh Aplikasi</a>
  </div>
  <script>
  // Buka aplikasi HANYA saat diketuk (tombol merah, bar bawah,
  // atau poster) — TIDAK otomatis saat halaman dibuka. Auto-open
  // dimatikan (permintaan Rams 2026-11-25): intent:// yang
  // ditembak otomatis 0,6 detik setelah halaman dibuka tidak
  // membuka aplikasi dari Chrome, melainkan mengikuti
  // browser_fallback_url ke #download — preview-nya cuma numpang
  // lewat sebelum sempat dibaca. Saat diketuk, urutannya sama:
  // intent:// dulu (Chrome), lalu skema subsenpai:// sebagai
  // cadangan; tanpa aplikasi terpasang, fallback mengarah ke
  // halaman download — persis alur TikTok.
  (function () {
    var intent = "{{.IntentURL}}";
    var scheme = {{.SchemeURL}};
    function go(u) { try { location.href = u; } catch (e) {} }
    function bindOpen(id) {
      var el = document.getElementById(id);
      if (!el) return;
      el.addEventListener("click", function (ev) {
        ev.preventDefault();
        go(intent);
        setTimeout(function () { if (!document.hidden) go(scheme); }, 500);
      });
    }
    bindOpen("openBtn");
    bindOpen("openBtn2");
    bindOpen("playBtn");
  })();
  </script>
{{else}}
  <div class="err"><b>Judul tidak ditemukan</b>Tautan ini mungkin sudah tidak berlaku. Jelajahi katalog lengkapnya di halaman utama.<div style="margin-top:16px"><a class="btn" href="https://subsenpai.web.id/">Ke Halaman Utama</a></div></div>
{{end}}
</main>
</body>
</html>`))

// trimDesc memotong teks untuk meta description (~200 karakter,
// sadar rune agar tidak memotong UTF-8 di tengah).
func trimDesc(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}

// typeLabelID: label Indonesia untuk tipe konten.
func typeLabelID(typ string) string {
	switch typ {
	case "anime":
		return "Anime"
	case "donghua":
		return "Donghua"
	case "film":
		return "Film"
	}
	return ""
}

// seoTitle: judul yang tampil di hasil pencarian Google — nama +
// tahun + tipe + kata kunci "Subtitle Indonesia" (permintaan Rams
// 28 Nov 2026). epNo > 0 → halaman episode.
func seoTitle(title string, year *int, typ string, epNo int) string {
	s := title
	if epNo > 0 {
		s = fmt.Sprintf("%s — Episode %d", title, epNo)
	}
	if year != nil {
		s += " (" + strconv.Itoa(*year) + ")"
	}
	if lbl := typeLabelID(typ); lbl != "" {
		s += " — " + lbl + " Subtitle Indonesia"
	}
	return s + " | SubSenpai"
}

// seoDescription: sinopsis bila ada; bila kosong, bangun dari
// metadata agar halaman tetap punya deskripsi unik.
func seoDescription(title string, year *int, typ string, genres []string, synopsis *string) string {
	if synopsis != nil && strings.TrimSpace(*synopsis) != "" {
		return trimDesc(*synopsis, 200)
	}
	s := "Nonton " + title
	if year != nil {
		s += " (" + strconv.Itoa(*year) + ")"
	}
	if lbl := typeLabelID(typ); lbl != "" {
		s += " " + lbl
	}
	s += " subtitle Indonesia di SubSenpai."
	if len(genres) > 0 {
		s += " " + strings.Join(genres, ", ") + "."
	}
	return s
}

// shareJSONLD: data terstruktur schema.org (Movie / TVSeries /
// TVEpisode) untuk rich result Google.
func shareJSONLD(name, pageURL, image, desc string, year *int, genres []string, typ string, epNo int, seriesName, seriesURL string) template.JS {
	v := map[string]any{
		"@context":   "https://schema.org",
		"@type":      "Movie",
		"name":       name,
		"url":        pageURL,
		"inLanguage": "id",
	}
	if typ != "film" {
		v["@type"] = "TVSeries"
	}
	if epNo > 0 {
		v["@type"] = "TVEpisode"
		v["episodeNumber"] = epNo
		if seriesName != "" {
			v["partOfSeries"] = map[string]any{"@type": "TVSeries", "name": seriesName, "url": seriesURL}
		}
	}
	if image != "" {
		v["image"] = image
	}
	if desc != "" {
		v["description"] = desc
	}
	if year != nil {
		v["datePublished"] = strconv.Itoa(*year)
	}
	if len(genres) > 0 {
		v["genre"] = genres
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

func (a *API) handleSharePage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	data := sharePageData{
		Title:       "SubSenpai",
		Description: "Nonton anime, donghua & film subtitle Indonesia di aplikasi SubSenpai.",
		PageURL:     "https://subsenpai.web.id/judul/" + id,
		ID:          id,
	}
	status := http.StatusOK
	if shareIDRe.MatchString(id) {
		var title, typ string
		var poster, synopsis *string
		var year *int
		var genres []string
		err := a.pool.QueryRow(r.Context(), `
			select title, poster_url, year, type, genres, synopsis
			from titles where id = $1::uuid and not is_hidden`, id).
			Scan(&title, &poster, &year, &typ, &genres, &synopsis)
		if err == nil {
			data.Found = true
			data.Title = title
			if synopsis != nil && strings.TrimSpace(*synopsis) != "" {
				data.Synopsis = *synopsis
			}
			meta := []string{}
			if year != nil {
				meta = append(meta, strconv.Itoa(*year))
			}
			switch typ {
			case "anime":
				meta = append(meta, "Anime")
			case "donghua":
				meta = append(meta, "Donghua")
			case "film":
				meta = append(meta, "Film")
			}
			data.MetaLine = strings.Join(meta, " · ")
			data.GenresLine = strings.Join(genres, " • ")
			if poster != nil && strings.TrimSpace(*poster) != "" {
				proxy := "https://" + r.Host + "/api/v1/img-proxy?u=" + url.QueryEscape(*poster)
				data.ImageURL = proxy
				data.PosterURL = proxy
			}
			data.IntentURL = template.URL("intent://title/" + id +
				"#Intent;scheme=subsenpai;package=com.subsenpai;S.browser_fallback_url=" +
				url.QueryEscape("https://subsenpai.web.id/#download") + ";end")
			data.SchemeURL = template.JS(strconv.Quote("subsenpai://title/" + id))
			data.SEOTitle = seoTitle(title, year, typ, 0)
			data.Description = seoDescription(title, year, typ, genres, synopsis)
			data.OGType = "video.tv_show"
			if typ == "film" {
				data.OGType = "video.movie"
			}
			data.JSONLD = shareJSONLD(title, data.PageURL, data.ImageURL, data.Description, year, genres, typ, 0, "", "")
		} else {
			status = http.StatusNotFound
		}
	} else {
		status = http.StatusNotFound
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(status)
	_ = shareTpl.Execute(w, data)
}
