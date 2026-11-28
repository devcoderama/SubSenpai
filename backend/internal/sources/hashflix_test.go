package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTMDBIDFromRef(t *testing.T) {
	cases := map[string]string{
		"https://hashflix.top/movie/83533":  "83533",
		"https://hashflix.top/movie/83533/": "83533",
		"avatar-fire-and-ash-83533":          "83533",
		"83533":                              "83533",
		"https://hashflix.top/movie/":       "",
		"bukan-id":                           "",
		"":                                   "",
	}
	for in, want := range cases {
		if got := tmdbIDFromRef(in); got != want {
			t.Errorf("tmdbIDFromRef(%q) = %q, mau %q", in, got, want)
		}
	}
}

func TestParseHFDetails(t *testing.T) {
	body := `{"title":"Avatar: Fire and Ash","original_title":"Avatar: Fire and Ash",` +
		`"overview":"Jake Sully dan Neytiri menghadapi ancaman baru.",` +
		`"runtime":198,"release_date":"2025-12-17",` +
		`"genres":[{"id":878,"name":"Science Fiction"},{"id":12,"name":"Adventure"}],` +
		`"production_companies":[{"name":"Lightstorm Entertainment"}],` +
		`"production_countries":[{"iso_3166_1":"US"}]}`
	meta, err := parseHFDetails(body)
	if err != nil {
		t.Fatalf("parseHFDetails: %v", err)
	}
	if meta.Synopsis == "" || meta.DurationMin != 198 || meta.Year != 2025 {
		t.Errorf("meta inti salah: %+v", meta)
	}
	if len(meta.Genres) != 2 || meta.Studio != "Lightstorm Entertainment" || meta.Country != "US" {
		t.Errorf("meta pelengkap salah: %+v", meta)
	}
	if meta.TitleAlt != "" {
		t.Errorf("TitleAlt harus kosong bila sama dengan judul: %q", meta.TitleAlt)
	}
	if _, err := parseHFDetails(`{"title":"X"}`); err == nil {
		t.Error("detail kosong harus error")
	}
}

func TestHashflixEpisodesResolve(t *testing.T) {
	h := NewHashflix()
	eps, err := h.Episodes(context.Background(), Source{}, "https://hashflix.top/movie/83533")
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(eps) != 1 || eps[0].Number != 1 || eps[0].Ref != "83533" {
		t.Fatalf("episode salah: %+v", eps)
	}
	res, err := h.Resolve(context.Background(), Source{}, eps[0])
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(res.Streams) != 4 {
		t.Fatalf("mau 4 aliran, dapat %d", len(res.Streams))
	}
	want := []string{
		"https://vidstuck.cc/embed/movie/83533",
		"https://zxcstream.cc/player/movie/83533",
		"https://vidbolt.xyz/movie/83533",
		"https://vidfast.vc/movie/83533",
	}
	for i, w := range want {
		if res.Streams[i].URL != w {
			t.Errorf("aliran %d = %q, mau %q", i, res.Streams[i].URL, w)
		}
	}
}

// TestHashflixLive adalah uji asap ke situs asli — hanya jalan bila
// SUBSENPAI_LIVE_TEST=1.
func TestHashflixLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	h := NewHashflix()
	src := Source{Name: "Hashflix", ContentType: "film", Lang: "multi", BaseURL: "https://hashflix.top", Resolver: "hashflix"}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	meta, err := h.SeriesInfo(ctx, src, "https://hashflix.top/movie/83533")
	if err != nil {
		t.Fatalf("SeriesInfo: %v", err)
	}
	t.Logf("sinopsis %dB, durasi %d, tahun %d, genre %v, studio %q",
		len(meta.Synopsis), meta.DurationMin, meta.Year, meta.Genres, meta.Studio)
	if meta.Synopsis == "" || meta.DurationMin == 0 {
		t.Fatal("metadata live kosong")
	}
}

// Uji URL discover ala donkey.to: genre banyak = OR, tahun, negara,
// urutan — dan genre tak dikenal mengembalikan kosong.
func TestHFDiscoverURL(t *testing.T) {
	base := "https://hashflix.top"
	cases := []struct {
		name string
		f    BrowseFilter
		page int
		want string
		ok   bool
	}{
		{"kosong", BrowseFilter{}, 1, base + "/database/discover/movie?page=1", true},
		{"satu genre", BrowseFilter{Genres: []string{"Action"}}, 1, base + "/database/discover/movie?page=1&with_genres=28", true},
		{"multi genre OR", BrowseFilter{Genres: []string{"Action", "Horror"}}, 2, base + "/database/discover/movie?page=2&with_genres=28%7C27", true},
		{"genre tak dikenal", BrowseFilter{Genres: []string{"BukanGenre"}}, 1, "", false},
		{"tahun+negara+urutan", BrowseFilter{Genres: []string{"Action"}, Year: 2024, Country: "id", Sort: "latest"}, 1, base + "/database/discover/movie?page=1&with_genres=28&primary_release_year=2024&with_origin_country=ID&sort_by=primary_release_date.desc", true},
		{"urutan tak dikenal diabaikan", BrowseFilter{Sort: "acak"}, 1, base + "/database/discover/movie?page=1", true},
		{"urutan nama A-Z", BrowseFilter{Sort: "name_asc"}, 1, base + "/database/discover/movie?page=1&sort_by=title.asc", true},
		{"urutan paling banyak divote", BrowseFilter{Sort: "most_voted"}, 1, base + "/database/discover/movie?page=1&sort_by=vote_count.desc", true},
		{"page 0 jadi 1", BrowseFilter{}, 0, base + "/database/discover/movie?page=1", true},
	}
	for _, c := range cases {
		got, ok := hfDiscoverURL(base, c.f, c.page)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got (%q, %v), mau (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestHashflixBrowseFiltered(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"page":1,"results":[
			{"id":801688,"title":"Kalki 2898-AD","poster_path":"/x.jpg","release_date":"2024-06-27","genre_ids":[28,878],"adult":false}
		]}`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewHashflix().BrowseFiltered(context.Background(), src,
		BrowseFilter{Genres: []string{"Action", "Horror"}, Year: 2024, Country: "id", Sort: "latest"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"with_genres=28%7C27", "primary_release_year=2024", "with_origin_country=ID", "sort_by=primary_release_date.desc"} {
		if !strings.Contains(gotQuery, want) {
			t.Fatalf("query=%q, kurang %q", gotQuery, want)
		}
	}
	if len(out) != 1 || out[0].Slug != "kalki-2898-ad-801688" || out[0].Year != 2024 {
		t.Fatalf("hasil=%+v", out)
	}
}

func TestHashflixSearchTitles(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query().Get("query")
		_, _ = w.Write([]byte(`{"page":1,"results":[
			{"id":533535,"title":"Deadpool & Wolverine","poster_path":"/8cd.jpg","release_date":"2024-07-24","genre_ids":[28,35,878],"adult":false}
		]}`))
	}))
	defer srv.Close()
	src := Source{BaseURL: srv.URL}
	out, err := NewHashflix().SearchTitles(context.Background(), src, "deadpool")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/database/search/movie" || gotQuery != "deadpool" {
		t.Fatalf("path=%q query=%q", gotPath, gotQuery)
	}
	if len(out) != 1 || out[0].Slug != "deadpool-wolverine-533535" || out[0].URL != srv.URL+"/movie/533535" {
		t.Fatalf("hasil=%+v", out)
	}
}
