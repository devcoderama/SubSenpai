package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// probeStream: sukses hanya bila 2xx + badan playlist HLS.
func TestProbeStream(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n"))
	}))
	defer good.Close()
	badStatus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("nope"))
	}))
	defer badStatus.Close()
	notPlaylist := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!DOCTYPE html><html></html>"))
	}))
	defer notPlaylist.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("#EXTM3U\n"))
	}))
	defer slow.Close()

	ctx := context.Background()
	if !probeStream(ctx, good.URL, "", "") {
		t.Error("playlist bagus harus lolos")
	}
	if probeStream(ctx, badStatus.URL, "", "") {
		t.Error("403 harus gagal")
	}
	if probeStream(ctx, notPlaylist.URL, "", "") {
		t.Error("badan HTML harus gagal")
	}
	if probeStream(ctx, "http://127.0.0.1:1/tidak-ada", "", "") {
		t.Error("koneksi gagal harus gagal")
	}
	// Timeout: perkecil batas sementara supaya tes cepat.
	old := tvProbeTimeout
	tvProbeTimeout = 100 * time.Millisecond
	defer func() { tvProbeTimeout = old }()
	if probeStream(ctx, slow.URL, "", "") {
		t.Error("respons lambat harus kena timeout")
	}
	// Header Referer + UA diteruskan apa adanya.
	var gotRef, gotUA string
	hdr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRef, gotUA = r.Header.Get("Referer"), r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("#EXTM3U\n"))
	}))
	defer hdr.Close()
	if !probeStream(ctx, hdr.URL, "https://contoh.id/live", "UA-Uji/1.0") {
		t.Fatal("probe ber-header harus lolos")
	}
	if gotRef != "https://contoh.id/live" || gotUA != "UA-Uji/1.0" {
		t.Errorf("header tidak diteruskan: ref=%q ua=%q", gotRef, gotUA)
	}
	if uaOrDefault("") == "" || uaOrDefault("X") != "X" {
		t.Error("uaOrDefault salah")
	}
}
