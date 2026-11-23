package httpapi

// Proxy gambar publik (permintaan Rams 2026-11-23): poster katalog
// di dashboard web tidak tampil karena CDN sumber (Kuronime,
// Anichin, dll.) menolak hotlink dari domain asing. Proxy ini
// mengambil gambar dari server dengan header Referer = asal
// gambar itu sendiri (pola yang sama dengan unduhan poster di
// aplikasi), lalu meneruskannya ke peramban.
//
// Pengaman SSRF: hanya http/https; host harus publik (IP privat,
// loopback, dan link-local ditolak — termasuk sesudah redirect);
// hanya konten yang terdeteksi sebagai gambar; maks 6 MB.

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// imgHostAllowed dapat diganti di pengujian.
var imgHostAllowed = func(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return isPublicIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return false
		}
	}
	return true
}

func isPublicIP(ip net.IP) bool {
	return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsUnspecified()
}

var imgProxyClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		if !imgHostAllowed(req.URL.Hostname()) {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func fetchImage(target *url.URL, referer string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, target.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,*/*")
	resp, err := imgProxyClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20+1))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func (a *API) handleImgProxy(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("u")
	target, err := url.Parse(raw)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" {
		http.Error(w, "url tidak valid", http.StatusBadRequest)
		return
	}
	if !imgHostAllowed(target.Hostname()) {
		http.Error(w, "host tidak diizinkan", http.StatusBadRequest)
		return
	}
	// Percobaan pertama dengan Referer asal gambar; sebagian CDN
	// justru hanya menerima tanpa Referer — coba lagi polos.
	status, body, ferr := fetchImage(target, target.Scheme+"://"+target.Host+"/")
	if ferr == nil && status == http.StatusForbidden {
		status, body, ferr = fetchImage(target, "")
	}
	if ferr != nil || status != http.StatusOK {
		http.Error(w, "gambar tidak dapat diambil", http.StatusBadGateway)
		return
	}
	if len(body) > 6<<20 {
		http.Error(w, "gambar terlalu besar", http.StatusBadGateway)
		return
	}
	ctype := http.DetectContentType(body)
	if !strings.HasPrefix(ctype, "image/") || strings.Contains(ctype, "svg") {
		http.Error(w, "bukan gambar", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}
