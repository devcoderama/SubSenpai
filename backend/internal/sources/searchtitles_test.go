package sources

// Uji pencarian judul langsung: URL halaman pencarian yang dibangun
// benar dan hasilnya terbaca oleh pengurai kartu/JSON yang sama
// dengan arsip & trending. Server tiruan lokal.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchTitlesKuronime(t *testing.T) {
	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`<article class="bs"><div class="bsx">
			<a href="https://kuronime.sbs/anime/fairy-tail-ova-2016/" title="Fairy Tail OVA (2016)">
			<div class="limit"><img src="https://i1.wp.com/kuronime.sbs/wp-content/uploads/ftova.webp"></div>
			<div class="tt"><h2>Fairy Tail OVA (2016)</h2></div></a></div></article>`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewKuronime().(TitleSearcher).SearchTitles(context.Background(), src, "fairy tail")
	if err != nil {
		t.Fatal(err)
	}
	if gotRawQuery != "s=fairy+tail" {
		t.Fatalf("query=%q", gotRawQuery)
	}
	if len(out) != 1 || out[0].Slug != "fairy-tail-ova-2016" || out[0].Title != "Fairy Tail OVA (2016)" {
		t.Fatalf("hasil=%+v", out)
	}
	if out[0].URL != srv.URL+"/anime/fairy-tail-ova-2016/" {
		t.Fatalf("url=%q", out[0].URL)
	}
	if out[0].Poster == "" {
		t.Fatalf("poster kosong: %+v", out[0])
	}
}

func TestSearchTitlesAnichin(t *testing.T) {
	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`<article class="bs"><div class="bsx"><a href="/i-am-the-blade/" title="I Am The Blade">
			<div class="limit"><img src="/wp-content/uploads/blade.webp" class="ts-post-image"/></div>
			<div class="tt">I Am The Blade<h2>I Am The Blade</h2></div></a></div></article>`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewAnichin().(TitleSearcher).SearchTitles(context.Background(), src, "blade")
	if err != nil {
		t.Fatal(err)
	}
	if gotRawQuery != "s=blade" {
		t.Fatalf("query=%q", gotRawQuery)
	}
	if len(out) != 1 || out[0].Slug != "i-am-the-blade" || out[0].URL != srv.URL+"/i-am-the-blade/" {
		t.Fatalf("hasil=%+v", out)
	}
	if !strings.HasPrefix(out[0].Poster, srv.URL+"/wp-content/") {
		t.Fatalf("poster=%q", out[0].Poster)
	}
}

func TestSearchTitlesHashflix(t *testing.T) {
	var gotPath, gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"page":1,"total_pages":1,"results":[
			{"id":607833,"title":"The Intern","poster_path":"/intern.jpg","release_date":"2026-01-01","genre_ids":[18,35]}
		]}`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewHashflix().SearchTitles(context.Background(), src, "the intern")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/database/search/movie" || !strings.Contains(gotRawQuery, "query=the+intern") {
		t.Fatalf("path=%q query=%q", gotPath, gotRawQuery)
	}
	if len(out) != 1 || out[0].Slug != "the-intern-607833" || out[0].Year != 2026 {
		t.Fatalf("hasil=%+v", out)
	}
	if out[0].Poster != "https://image.tmdb.org/t/p/w500/intern.jpg" {
		t.Fatalf("poster=%q", out[0].Poster)
	}
	// Kata kosong: tanpa panggilan, tanpa error.
	out, err = NewHashflix().SearchTitles(context.Background(), src, "  ")
	if err != nil || len(out) != 0 {
		t.Fatalf("kata kosong: out=%v err=%v", out, err)
	}
}
