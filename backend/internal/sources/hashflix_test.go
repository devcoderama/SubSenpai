package sources

import (
	"context"
	"os"
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
