package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/db"
)

// TestSeriesAPILive: endpoint serial TV lawan DB uji lokal (judul +
// episode di-seed langsung): /titles?type=series menemukannya,
// detail mengembalikan season_no + locked=false untuk tamu,
// playback tamu lolos (gratis) dengan sumber embed TV,
// live-browse type=series mengembalikan judul TV dari Hashflix.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestSeriesAPILive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrasi: %v", err)
	}
	a := &API{pool: pool, cache: newTTLCache(time.Minute), guard: newFillGuard(time.Minute)}

	var titleID string
	err = pool.QueryRow(ctx, `
		insert into titles (slug, type, title, synopsis, year, status, genres)
		values ('uji-serial-tv-1396', 'series', 'Uji Serial TV', 'Sinopsis uji.', 2008, 'completed', array['Drama'])
		returning id`).Scan(&titleID)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from titles where id = $1::uuid`, titleID)
	epIDs := map[string]string{}
	for _, ep := range []struct {
		season, no int
	}{
		{1, 1}, {1, 2}, {2, 1},
	} {
		var epID string
		err := pool.QueryRow(ctx, `
			insert into episodes (title_id, season_no, episode_no, title, video_url)
			values ($1::uuid, $2, $3, $4, $5) returning id`,
			titleID, ep.season, ep.no, "Episode", "https://hashflix.top/tv/1396").Scan(&epID)
		if err != nil {
			t.Fatal(err)
		}
		epIDs[strconv.Itoa(ep.season)+"-"+strconv.Itoa(ep.no)] = epID
		if _, err := pool.Exec(ctx, `
			insert into episode_sources (episode_id, url, label) values
			($1::uuid, $2, 'Main Server'), ($1::uuid, $3, 'Backup I')`,
			epID,
			"https://vidstuck.cc/embed/tv/1396/"+strconv.Itoa(ep.season)+"/"+strconv.Itoa(ep.no),
			"https://zxcstream.cc/player/tv/1396/"+strconv.Itoa(ep.season)+"/"+strconv.Itoa(ep.no)); err != nil {
			t.Fatal(err)
		}
	}

	// 1) Daftar: type=series menemukannya, type=film tidak.
	body := callTitles(t, a, "/api/v1/titles?type=series&limit=50")
	if !strings.Contains(body, "Uji Serial TV") {
		t.Fatalf("titles?type=series tidak memuat judul uji: %s", trunc(body, 400))
	}
	body = callTitles(t, a, "/api/v1/titles?type=film&limit=50")
	if strings.Contains(body, "Uji Serial TV") {
		t.Fatalf("titles?type=film ikut memuat judul series")
	}

	// 2) Detail: season_no + locked=false untuk tamu.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/titles/"+titleID, nil)
	req = withChiParam(req, "id", titleID)
	a.handleTitleDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: status %d: %s", rec.Code, trunc(rec.Body.String(), 300))
	}
	var detail struct {
		Episodes []struct {
			ID        string `json:"id"`
			SeasonNo  int    `json:"season_no"`
			EpisodeNo int    `json:"episode_no"`
			Locked    bool   `json:"locked"`
		} `json:"episodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Episodes) != 3 {
		t.Fatalf("detail mau 3 episode, dapat %d", len(detail.Episodes))
	}
	if detail.Episodes[0].SeasonNo != 1 || detail.Episodes[2].SeasonNo != 2 || detail.Episodes[2].EpisodeNo != 1 {
		t.Fatalf("urutan season salah: %+v", detail.Episodes)
	}
	for _, e := range detail.Episodes {
		if e.Locked {
			t.Fatalf("episode series harus gratis (locked=false) untuk tamu: %+v", e)
		}
	}

	// 3) Playback tamu S2E1: lolos + sumber embed TV + season_no.
	s2e1 := epIDs["2-1"]
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/episodes/"+s2e1+"/playback", nil)
	req = withChiParam(req, "id", s2e1)
	a.handlePlayback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("playback tamu S2E1: status %d: %s", rec.Code, trunc(rec.Body.String(), 300))
	}
	var play struct {
		Episode struct {
			SeasonNo  int `json:"season_no"`
			EpisodeNo int `json:"episode_no"`
		} `json:"episode"`
		Sources []struct {
			URL string `json:"url"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &play); err != nil {
		t.Fatal(err)
	}
	if play.Episode.SeasonNo != 2 || play.Episode.EpisodeNo != 1 {
		t.Fatalf("episode playback salah: %+v", play.Episode)
	}
	if len(play.Sources) == 0 || play.Sources[0].URL != "https://vidstuck.cc/embed/tv/1396/2/1" {
		t.Fatalf("sumber playback salah: %+v", play.Sources)
	}

	// 4) Live-browse type=series: judul TV dari Hashflix.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/live-browse?type=series&page=1", nil)
	a.handleLiveBrowse(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live-browse series: status %d: %s", rec.Code, trunc(rec.Body.String(), 300))
	}
	var lb struct {
		Items []struct {
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lb); err != nil {
		t.Fatal(err)
	}
	if len(lb.Items) == 0 {
		t.Fatal("live-browse series kosong")
	}
	t.Logf("live-browse series: %d judul, pertama %q", len(lb.Items), lb.Items[0].Title)
}

func callTitles(t *testing.T, a *API, url string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	a.handleTitles(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", url, rec.Code, trunc(rec.Body.String(), 300))
	}
	return rec.Body.String()
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
