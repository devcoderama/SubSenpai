package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// probeStream: sukses hanya bila 2xx + playlist berisi segmen
// (atau master yang variannya berisi segmen), atau manifest DASH
// valid yang tidak terenkripsi. Playlist kosong (0 segmen) gagal —
// itu jebakan v0.24.40 (dens.tv 200 + #EXTM3U tanpa segmen).
func TestProbeStream(t *testing.T) {
	seg := "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:6\n#EXTINF:6.0,\nseg0.ts\n#EXTINF:6.0,\nseg1.ts\n"
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte(seg))
	}))
	defer good.Close()
	// Playlist kosong: 200 + #EXTM3U tapi 0 segmen -> gagal.
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n"))
	}))
	defer empty.Close()
	// Master playlist: varian bandwidth tertinggi berisi segmen.
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/variant.m3u8" {
			_, _ = w.Write([]byte(seg))
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720\nvariant.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=400000,RESOLUTION=854x480\nvariant.m3u8\n"))
	}))
	defer master.Close()
	// Master yang variannya kosong -> gagal.
	masterEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/variant.m3u8" {
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n"))
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000\nvariant.m3u8\n"))
	}))
	defer masterEmpty.Close()
	// DASH valid (tidak terenkripsi) -> lolos; terenkripsi -> gagal.
	dash := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cenc.mpd" {
			_, _ = w.Write([]byte(`<MPD><Period><AdaptationSet><ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011"/></AdaptationSet></Period></MPD>`))
			return
		}
		_, _ = w.Write([]byte(`<MPD><Period><AdaptationSet><SegmentTemplate/></AdaptationSet></Period></MPD>`))
	}))
	defer dash.Close()
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
		_, _ = w.Write([]byte(seg))
	}))
	defer slow.Close()

	ctx := context.Background()
	if !probeStream(ctx, good.URL, "", "") {
		t.Error("playlist berisi segmen harus lolos")
	}
	if probeStream(ctx, empty.URL, "", "") {
		t.Error("playlist kosong (0 segmen) harus gagal")
	}
	if !probeStream(ctx, master.URL, "", "") {
		t.Error("master yang variannya berisi segmen harus lolos")
	}
	if probeStream(ctx, masterEmpty.URL, "", "") {
		t.Error("master yang variannya kosong harus gagal")
	}
	if !probeStream(ctx, dash.URL+"/x.mpd", "", "") {
		t.Error("MPD valid harus lolos")
	}
	if probeStream(ctx, dash.URL+"/cenc.mpd", "", "") {
		t.Error("MPD terenkripsi harus gagal")
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
		_, _ = w.Write([]byte(seg))
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
