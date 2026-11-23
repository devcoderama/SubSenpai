package httpapi

import (
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
// https://www.subsenpai.web.id/judul/{id} ke endpoint ini (rewrite
// status 200), jadi URL publiknya tetap domain resmi sementara isi
// halamannya datang dari sini. Halaman yang sama juga yang dibuka
// manusia di peramban: poster, info, dan tombol membuka aplikasi.

var shareIDRe = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

type sharePageData struct {
	Title       string
	Description string
	PageURL     string
	ImageURL    string
	PosterURL   string
	MetaLine    string
	GenresLine  string
	Synopsis    string
	IntentURL   template.URL
	ID          string
	Found       bool
}

var shareTpl = template.Must(template.New("share").Parse(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}} — SubSenpai</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.PageURL}}">
<meta property="og:type" content="website">
<meta property="og:site_name" content="SubSenpai">
<meta property="og:title" content="{{.Title}} — SubSenpai">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.PageURL}}">
{{if .ImageURL}}<meta property="og:image" content="{{.ImageURL}}">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:image" content="{{.ImageURL}}">{{end}}
<meta name="twitter:title" content="{{.Title}} — SubSenpai">
<meta name="twitter:description" content="{{.Description}}">
<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90' fill='%23E50914' font-weight='900'>S</text></svg>">
<style>
  body{background:#141414;color:#fff;font-family:system-ui,'Segoe UI',sans-serif;line-height:1.6;margin:0;min-height:100vh}
  .wrap{max-width:880px;margin:0 auto;padding:0 4%}
  header{background:#0b0b0b;border-bottom:1px solid #2d2d2d}
  .bar{padding:14px 0}
  .wordmark{color:#E50914;font-weight:900;font-size:1.5rem;letter-spacing:.06em;text-transform:uppercase;text-decoration:none}
  main{padding:38px 0 60px}
  .detail{display:flex;gap:26px;align-items:flex-start}
  @media (max-width:640px){.detail{flex-direction:column;align-items:center;text-align:center}}
  .poster{flex:0 0 auto;width:220px;max-width:62vw;aspect-ratio:2/3;border-radius:10px;overflow:hidden;background:#232323;box-shadow:0 18px 44px rgba(0,0,0,.55)}
  .poster img{width:100%;height:100%;object-fit:cover;display:block}
  .info{flex:1;min-width:0}
  .info h1{font-size:clamp(1.6rem,4.6vw,2.4rem);line-height:1.2;font-weight:900;margin:0}
  .meta{color:#b3b3b3;margin:10px 0 4px}
  .genres{color:#e5e5e5;font-size:.95rem}
  .syn{color:#d7d7d7;margin:16px 0 26px}
  .btn{display:inline-block;background:#E50914;color:#fff;font-weight:700;padding:13px 28px;border-radius:6px;text-decoration:none}
  .btn.ghost{background:rgba(255,255,255,.14);margin-left:8px}
  .note{color:#b3b3b3;font-size:.88rem;margin-top:14px}
  .err{background:#1f1f1f;border-radius:10px;padding:28px;text-align:center;color:#b3b3b3}
  .err b{color:#fff;display:block;font-size:1.15rem;margin-bottom:6px}
</style>
</head>
<body>
<header><div class="wrap bar"><a class="wordmark" href="https://www.subsenpai.web.id/">SubSenpai</a></div></header>
<main class="wrap">
{{if .Found}}
  <div class="detail">
    {{if .PosterURL}}<div class="poster"><img src="{{.PosterURL}}" alt="{{.Title}}"></div>{{end}}
    <div class="info">
      <h1>{{.Title}}</h1>
      {{if .MetaLine}}<div class="meta">{{.MetaLine}}</div>{{end}}
      {{if .GenresLine}}<div class="genres">{{.GenresLine}}</div>{{end}}
      {{if .Synopsis}}<p class="syn">{{.Synopsis}}</p>{{end}}
      <a class="btn" id="openBtn" href="{{.IntentURL}}">▶ Buka di Aplikasi SubSenpai</a>
      <a class="btn ghost" href="https://www.subsenpai.web.id/">Download Aplikasi</a>
      <div class="note">Kalau aplikasi tidak terbuka otomatis, ketuk tombol merah di atas. Belum punya aplikasinya? Download dulu — gratis.</div>
    </div>
  </div>
  <script>
  // Buka aplikasi: coba intent:// (Chrome), lalu skema subsenpai://
  // sebagai cadangan — peramban di dalam aplikasi chat (Telegram/
  // WhatsApp) sering menelan intent:// tapi meneruskan skema khusus
  // ke sistem. Tombol merah memakai urutan yang sama saat diketuk.
  (function () {
    var intent = "{{.IntentURL}}";
    var scheme = "subsenpai://title/{{.ID}}";
    function go(u) { try { location.href = u; } catch (e) {} }
    setTimeout(function () { go(intent); }, 600);
    setTimeout(function () { if (!document.hidden) go(scheme); }, 1700);
    var btn = document.getElementById("openBtn");
    if (btn) btn.addEventListener("click", function (ev) {
      ev.preventDefault();
      go(intent);
      setTimeout(function () { if (!document.hidden) go(scheme); }, 500);
    });
  })();
  </script>
{{else}}
  <div class="err"><b>Judul tidak ditemukan</b>Tautan ini mungkin sudah tidak berlaku. Jelajahi katalog lengkapnya di halaman utama.<div style="margin-top:16px"><a class="btn" href="https://www.subsenpai.web.id/">Ke Halaman Utama</a></div></div>
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

func (a *API) handleSharePage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	data := sharePageData{
		Title:       "SubSenpai",
		Description: "Nonton anime, donghua & film subtitle Indonesia di aplikasi SubSenpai.",
		PageURL:     "https://www.subsenpai.web.id/judul/" + id,
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
				data.Description = trimDesc(*synopsis, 200)
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
				url.QueryEscape("https://www.subsenpai.web.id/#download") + ";end")
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
