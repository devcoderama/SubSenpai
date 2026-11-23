package httpapi

// Uji proxy gambar: CDN tiruan yang hanya melayani bila Referer =
// asal gambar itu sendiri (pola perlindungan hotlink sumber
// poster), penolakan SSRF untuk host privat, dan penolakan konten
// bukan gambar. Tanpa database & tanpa jaringan luar.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// PNG 1x1 transparan.
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

func TestImgProxyReferer(t *testing.T) {
	// CDN tiruan: 403 tanpa Referer yang benar.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "http://" + r.Host + "/"
		if r.Header.Get("Referer") != want {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	}))
	defer upstream.Close()

	// Izinkan host pengujian (loopback) hanya selama uji ini.
	old := imgHostAllowed
	imgHostAllowed = func(string) bool { return true }
	defer func() { imgHostAllowed = old }()

	a := &API{}
	u := upstream.URL + "/poster.png"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/img-proxy?u="+url.QueryEscape(u), nil)
	a.handleImgProxy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("proxy status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("content-type=%q", ct)
	}
	if rec.Body.Len() != len(tinyPNG) {
		t.Fatalf("ukuran body=%d, want %d", rec.Body.Len(), len(tinyPNG))
	}
}

func TestImgProxyGuards(t *testing.T) {
	a := &API{}
	for _, u := range []string{
		"http://127.0.0.1:8080/x.png",   // loopback
		"http://10.0.0.5/x.png",         // privat
		"http://192.168.1.1/x.png",      // privat
		"http://[::1]/x.png",            // loopback v6
		"ftp://example.com/x.png",       // skema liar
		"http://169.254.169.254/latest", // link-local (metadata)
		"bukan-url",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/img-proxy?u="+url.QueryEscape(u), nil)
		a.handleImgProxy(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("u=%q status=%d, want 400", u, rec.Code)
		}
	}
	if imgHostAllowed("8.8.8.8") != true {
		t.Fatal("IP publik literal harus diizinkan")
	}
	if imgHostAllowed("localhost") != false {
		t.Fatal("localhost harus ditolak")
	}
}

func TestImgProxyRejectsNonImage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>halo</html>"))
	}))
	defer upstream.Close()
	old := imgHostAllowed
	imgHostAllowed = func(string) bool { return true }
	defer func() { imgHostAllowed = old }()

	a := &API{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/img-proxy?u="+url.QueryEscape(upstream.URL+"/x.png"), nil)
	a.handleImgProxy(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("konten bukan gambar: status=%d, want 502", rec.Code)
	}
}
