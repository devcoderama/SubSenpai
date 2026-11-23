package sources

// Uji telusur genre langsung: URL halaman genre yang dibangun
// benar, kartu terbaca oleh pengurai yang sama dengan arsip, dan
// Hashflix memetakan nama genre ke id TMDB. Server tiruan lokal.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowseGenreKuronime(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`<article class="bs"><div class="bsx">
			<a href="https://kuronime.sbs/anime/one-piece-op/" title="One Piece">
			<div class="limit"><img src="https://i1.wp.com/kuronime.sbs/wp-content/uploads/op.webp"></div>
			<div class="tt"><h2>One Piece</h2></div></a></div></article>`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewKuronime().(GenreBrowser).BrowseGenre(context.Background(), src, "Action", 2)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/genres/action/page/2/" {
		t.Fatalf("path=%q", gotPath)
	}
	if len(out) != 1 || out[0].Slug != "one-piece-op" || out[0].Title != "One Piece" {
		t.Fatalf("hasil=%+v", out)
	}
	if out[0].URL != srv.URL+"/anime/one-piece-op/" {
		t.Fatalf("url=%q", out[0].URL)
	}
}

func TestBrowseGenreAnichin(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`<article class="bs"><div class="bsx"><a href="/36-cavalry/" title="36 Cavalry">
			<div class="limit"><img src="/wp-content/uploads/36.webp" class="ts-post-image"/></div>
			<div class="tt">36 Cavalry<h2>36 Cavalry</h2></div></a></div></article>`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewAnichin().(GenreBrowser).BrowseGenre(context.Background(), src, "Action", 1)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/genres/action/" {
		t.Fatalf("path=%q", gotPath)
	}
	if len(out) != 1 || out[0].Slug != "36-cavalry" || out[0].URL != srv.URL+"/36-cavalry/" {
		t.Fatalf("hasil=%+v", out)
	}
	if !strings.HasPrefix(out[0].Poster, srv.URL+"/wp-content/") {
		t.Fatalf("poster=%q", out[0].Poster)
	}
}

func TestBrowseGenreHashflix(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"page":1,"total_pages":5,"results":[
			{"id":1726,"title":"Iron Man","poster_path":"/im.jpg","release_date":"2008-04-30","genre_ids":[28,878]}
		]}`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewHashflix().BrowseGenre(context.Background(), src, "Action", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "with_genres=28") {
		t.Fatalf("query=%q", gotQuery)
	}
	if len(out) != 1 || out[0].Slug != "iron-man-1726" || out[0].Year != 2008 {
		t.Fatalf("hasil=%+v", out)
	}
	// Genre tak dikenal TMDB: kosong tanpa error.
	out, err = NewHashflix().BrowseGenre(context.Background(), src, "Ngawur", 1)
	if err != nil || len(out) != 0 {
		t.Fatalf("genre liar: out=%v err=%v", out, err)
	}
	// Genre kapitalisasi bebas.
	if _, err := NewHashflix().BrowseGenre(context.Background(), src, "science fiction", 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "with_genres=878") {
		t.Fatalf("science fiction query=%q", gotQuery)
	}
}

func TestBrowseGenreBerandaKuronime(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`<article class="bs"><div class="bsx">
			<a href="https://kuronime.sbs/anime/judul-beranda/" title="Judul Beranda">
			<div class="limit"><img src="https://i1.wp.com/kuronime.sbs/wp-content/uploads/jb.webp"></div>
			<div class="tt"><h2>Judul Beranda</h2></div></a></div></article>`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewKuronime().(GenreBrowser).BrowseGenre(context.Background(), src, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/" {
		t.Fatalf("path=%q, want /", gotPath)
	}
	if len(out) != 1 || out[0].Slug != "judul-beranda" || out[0].Poster == "" {
		t.Fatalf("hasil=%+v", out)
	}
	// Halaman > 1 pada mode beranda: kosong.
	out2, _ := NewKuronime().(GenreBrowser).BrowseGenre(context.Background(), src, "", 2)
	if len(out2) != 0 {
		t.Fatalf("page2=%+v", out2)
	}
}

func TestBrowseGenreBerandaAnichin(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`<article class="bs"><div class="bsx"><a href="/donghua-beranda/" title="Donghua Beranda">
			<div class="limit"><img src="/wp-content/uploads/db.webp" class="ts-post-image"/></div>
			<div class="tt">Donghua Beranda<h2>Donghua Beranda</h2></div></a></div></article>`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewAnichin().(GenreBrowser).BrowseGenre(context.Background(), src, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/" {
		t.Fatalf("path=%q, want /", gotPath)
	}
	if len(out) != 1 || out[0].Slug != "donghua-beranda" {
		t.Fatalf("hasil=%+v", out)
	}
}
