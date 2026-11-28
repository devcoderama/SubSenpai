package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/db"
)

// TestTVAPILive: endpoint TV lawan DB uji lokal — /tv/channels
// mengembalikan kanal seed (label bagus + tidak stabil), filter
// ?category=Berita dan ?stability=tidak stabil bekerja, detail per
// slug mengembalikan stream_url + labelnya. Tamu boleh (tanpa auth).
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestTVAPILive(t *testing.T) {
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

	get := func(path string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		a.handleTVChannels(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	code, body := get("/api/v1/tv/channels")
	if code != http.StatusOK {
		t.Fatalf("list: status %d body %v", code, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 38 {
		t.Fatalf("list: %d kanal, mau 38", len(items))
	}
	seen := map[string]int{}
	webOnly := 0
	for _, it := range items {
		m, _ := it.(map[string]any)
		seen[m["stability"].(string)]++
		if m["web_ok"] == false {
			webOnly++
		}
		if m["stream_url"] == "" || m["name"] == "" {
			t.Errorf("kanal tanpa nama/stream: %v", m)
		}
	}
	if seen["bagus"] != 18 || seen["tidak stabil"] != 20 {
		t.Fatalf("label: bagus=%d tidak stabil=%d, mau 18/20", seen["bagus"], seen["tidak stabil"])
	}
	// 16 kanal hanya-aplikasi (5 http + 11 DASH .mpd); sisanya web_ok=true.
	if webOnly != 16 {
		t.Fatalf("web_ok=false: %d kanal, mau 16", webOnly)
	}

	code, body = get("/api/v1/tv/channels?category=Berita")
	items, _ = body["items"].([]any)
	if code != http.StatusOK || len(items) != 7 {
		t.Fatalf("filter Berita: status %d, %d kanal, mau 7", code, len(items))
	}
	code, body = get("/api/v1/tv/channels?stability=tidak%20stabil")
	items, _ = body["items"].([]any)
	if code != http.StatusOK || len(items) != 20 {
		t.Fatalf("filter tidak stabil: status %d, %d kanal, mau 20", code, len(items))
	}

	// Detail per slug (chi RouteContext disuntik manual).
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tv/channels/transtv", nil)
	req = withChiParam(req, "id", "transtv")
	a.handleTVChannel(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail transtv: status %d", rec.Code)
	}
	var detail map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	ch, _ := detail["channel"].(map[string]any)
	if ch["name"] != "Trans TV" || ch["stability"] != "bagus" {
		t.Fatalf("detail transtv: %v", ch)
	}
	if ch["stream_url"] != "https://live.cnbcindonesia.com/transtv/smil:transtv.smil/playlist.m3u8" {
		t.Fatalf("detail transtv stream salah: %v", ch["stream_url"])
	}

	// Slug tidak dikenal → 404.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tv/channels/tidak-ada", nil)
	req = withChiParam(req, "id", "tidak-ada")
	a.handleTVChannel(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("detail tidak-ada: status %d, mau 404", rec.Code)
	}
}
