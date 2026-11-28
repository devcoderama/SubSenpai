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
		"avatar-fire-and-ash-83533":         "83533",
		"83533":                             "83533",
		"https://hashflix.top/movie/":       "",
		"bukan-id":                          "",
		"":                                  "",
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
		got, ok := hfDiscoverURL(base, "movie", c.f, c.page)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got (%q, %v), mau (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

// Uji URL discover TV: genre + tahun + urutan memakai parameter
// discover/tv (first_air_date, name), bukan parameter film.
func TestHFDiscoverURLTV(t *testing.T) {
	base := "https://hashflix.top"
	cases := []struct {
		name string
		f    BrowseFilter
		page int
		want string
		ok   bool
	}{
		{"kosong", BrowseFilter{}, 1, base + "/database/discover/tv?page=1", true},
		{"satu genre", BrowseFilter{Genres: []string{"Drama"}}, 1, base + "/database/discover/tv?page=1&with_genres=18", true},
		{"genre TV khusus", BrowseFilter{Genres: []string{"Sci-Fi & Fantasy"}}, 1, base + "/database/discover/tv?page=1&with_genres=10765", true},
		{"genre film tak dikenal di TV", BrowseFilter{Genres: []string{"Western"}}, 1, base + "/database/discover/tv?page=1&with_genres=37", true},
		{"genre tak dikenal", BrowseFilter{Genres: []string{"BukanGenre"}}, 1, "", false},
		{"tahun+negara+urutan", BrowseFilter{Genres: []string{"Drama"}, Year: 2008, Country: "us", Sort: "latest"}, 1, base + "/database/discover/tv?page=1&with_genres=18&first_air_date_year=2008&with_origin_country=US&sort_by=first_air_date.desc", true},
		{"urutan nama A-Z", BrowseFilter{Sort: "name_asc"}, 1, base + "/database/discover/tv?page=1&sort_by=name.asc", true},
	}
	for _, c := range cases {
		got, ok := hfDiscoverURL(base, "tv", c.f, c.page)
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

// Uji pengurai tren TV: judul dari `name`, tahun dari first_air_date,
// genre TV, URL /tv/{id}, dan Completed harus false (status tamat
// tidak diketahui dari daftar tren).
func TestParseHFTrendingTV(t *testing.T) {
	body := `{"page":1,"results":[
		{"id":1396,"name":"Breaking Bad","poster_path":"/gg.jpg","first_air_date":"2008-01-20","genre_ids":[18,80],"adult":false},
		{"id":1399,"name":"Game of Thrones","poster_path":"/u.jpg","first_air_date":"2011-04-17","genre_ids":[10765,18],"adult":false},
		{"id":1,"name":"","first_air_date":"2020-01-01","genre_ids":[],"adult":false},
		{"id":2,"name":"Dewasa","first_air_date":"2020-01-01","genre_ids":[],"adult":true}
	],"total_pages":1}`
	out, pages := parseHFTrending(body, "https://hashflix.top", "tv")
	if pages != 1 || len(out) != 2 {
		t.Fatalf("mau 2 judul, dapat %d (pages=%d)", len(out), pages)
	}
	bb := out[0]
	if bb.Slug != "breaking-bad-1396" || bb.URL != "https://hashflix.top/tv/1396" || bb.Year != 2008 {
		t.Errorf("Breaking Bad salah: %+v", bb)
	}
	if bb.Completed {
		t.Error("serial TV dari tren harus Completed=false")
	}
	if len(bb.Genres) != 2 || bb.Genres[0] != "Drama" || bb.Genres[1] != "Crime" {
		t.Errorf("genre TV salah: %v", bb.Genres)
	}
	got := out[1]
	if len(got.Genres) != 2 || got.Genres[0] != "Sci-Fi & Fantasy" || got.Genres[1] != "Drama" {
		t.Errorf("genre GoT salah: %v", got.Genres)
	}
}

// Uji pengurai detail TV: judul dari `name`, tahun dari first_air_date,
// durasi dari episode_run_time pertama, studio + negara.
func TestParseHFDetailsTV(t *testing.T) {
	body := `{"name":"Breaking Bad","original_name":"Breaking Bad",` +
		`"overview":"Guru kimia menjadi produsen sabu.",` +
		`"episode_run_time":[49],"first_air_date":"2008-01-20","status":"Ended",` +
		`"number_of_seasons":5,"number_of_episodes":62,` +
		`"seasons":[{"season_number":0,"episode_count":9},{"season_number":1,"episode_count":7},{"season_number":5,"episode_count":16}],` +
		`"genres":[{"id":18,"name":"Drama"},{"id":80,"name":"Crime"}],` +
		`"production_companies":[{"name":"Sony Pictures Television"}],` +
		`"production_countries":[{"iso_3166_1":"US"}]}`
	meta, err := parseHFDetailsTV(body)
	if err != nil {
		t.Fatalf("parseHFDetailsTV: %v", err)
	}
	if meta.Synopsis == "" || meta.DurationMin != 49 || meta.Year != 2008 {
		t.Errorf("meta inti salah: %+v", meta)
	}
	if len(meta.Genres) != 2 || meta.Studio != "Sony Pictures Television" {
		t.Errorf("meta pelengkap salah: %+v", meta)
	}
	if _, err := parseHFDetailsTV(`{"name":"X"}`); err == nil {
		t.Error("detail kosong harus error")
	}
}

// Uji Episodes + Resolve serial TV lewat server tiruan: season 0
// dilewati, episode S1E1..S1E7 + S5E1..S5E16, URL embed /tv/{id}/{s}/{e}.
func TestHashflixEpisodesResolveTV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/database/details/tv/1396" {
			t.Errorf("path=%q, mau /database/details/tv/1396", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"name":"Breaking Bad","overview":"x","episode_run_time":[49],` +
			`"first_air_date":"2008-01-20","seasons":[{"season_number":0,"episode_count":9},` +
			`{"season_number":1,"episode_count":7},{"season_number":5,"episode_count":16}],` +
			`"genres":[{"id":18,"name":"Drama"}]}`))
	}))
	defer srv.Close()
	h := NewHashflix()
	src := Source{ContentType: "series", BaseURL: srv.URL}
	eps, err := h.Episodes(context.Background(), src, srv.URL+"/tv/1396")
	if err != nil {
		t.Fatalf("Episodes TV: %v", err)
	}
	if len(eps) != 23 { // 7 + 16 (season 0 dilewati)
		t.Fatalf("mau 23 episode, dapat %d", len(eps))
	}
	if eps[0].Season != 1 || eps[0].Number != 1 || eps[0].Title != "Episode 1" || eps[0].Ref != "1396" {
		t.Errorf("episode pertama salah: %+v", eps[0])
	}
	if eps[7].Season != 5 || eps[7].Number != 1 {
		t.Errorf("episode S5E1 salah: %+v", eps[7])
	}
	if eps[22].Season != 5 || eps[22].Number != 16 {
		t.Errorf("episode terakhir salah: %+v", eps[22])
	}
	res, err := h.Resolve(context.Background(), src, eps[7]) // S5E1
	if err != nil {
		t.Fatalf("Resolve TV: %v", err)
	}
	if len(res.Streams) != 4 {
		t.Fatalf("mau 4 aliran, dapat %d", len(res.Streams))
	}
	want := []string{
		"https://vidstuck.cc/embed/tv/1396/5/1",
		"https://zxcstream.cc/player/tv/1396/5/1",
		"https://vidbolt.xyz/tv/1396/5/1",
		"https://vidfast.vc/tv/1396/5/1",
	}
	for i, w := range want {
		if res.Streams[i].URL != w {
			t.Errorf("aliran %d = %q, mau %q", i, res.Streams[i].URL, w)
		}
	}
}

// Uji asap TV ke situs asli — hanya jalan bila SUBSENPAI_LIVE_TEST=1.
func TestHashflixLiveTV(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	h := NewHashflix()
	src := Source{Name: "Hashflix", ContentType: "series", Lang: "multi", BaseURL: "https://hashflix.top", Resolver: "hashflix"}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	meta, err := h.SeriesInfo(ctx, src, "https://hashflix.top/tv/1396")
	if err != nil {
		t.Fatalf("SeriesInfo TV: %v", err)
	}
	t.Logf("sinopsis %dB, durasi %d, tahun %d, genre %v, studio %q",
		len(meta.Synopsis), meta.DurationMin, meta.Year, meta.Genres, meta.Studio)
	if meta.Synopsis == "" || meta.Year != 2008 || len(meta.Genres) == 0 {
		t.Fatal("metadata TV live kosong")
	}
	eps, err := h.Episodes(ctx, src, "https://hashflix.top/tv/1396")
	if err != nil {
		t.Fatalf("Episodes TV: %v", err)
	}
	t.Logf("episode TV: %d (pertama S%dE%d, terakhir S%dE%d)",
		len(eps), eps[0].Season, eps[0].Number, eps[len(eps)-1].Season, eps[len(eps)-1].Number)
	if len(eps) != 62 {
		t.Fatalf("Breaking Bad mau 62 episode, dapat %d", len(eps))
	}
	res, err := h.Resolve(ctx, src, eps[0])
	if err != nil {
		t.Fatalf("Resolve TV: %v", err)
	}
	if len(res.Streams) != 4 || !strings.HasPrefix(res.Streams[0].URL, "https://vidstuck.cc/embed/tv/1396/1/1") {
		t.Fatalf("aliran TV salah: %+v", res.Streams)
	}
	found, err := h.SearchTitles(ctx, src, "breaking bad")
	if err != nil {
		t.Fatalf("SearchTitles TV: %v", err)
	}
	if len(found) == 0 || found[0].Slug != "breaking-bad-1396" {
		t.Fatalf("cari TV salah: %+v", found)
	}
	disc, err := h.Discover(ctx, src, 25)
	if err != nil {
		t.Fatalf("Discover TV: %v", err)
	}
	if len(disc) == 0 {
		t.Fatal("discover TV kosong")
	}
	t.Logf("discover TV: %d judul, pertama %q (%d)", len(disc), disc[0].Title, disc[0].Year)
}
