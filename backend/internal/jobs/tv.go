package jobs

// Health-check kanal TV (permintaan Rams 2026-11-28): tiap kanal
// di-probe langsung ke stream URL-nya. Sukses = HTTP 2xx + playlist
// HLS yang BENAR-BENAR berisi segmen (minimal satu #EXTINF), atau
// master playlist yang variannya berisi segmen; DASH .mpd sukses bila
// manifest valid dan tidak terenkripsi (tanpa ContentProtection/cenc).
//
// PELAJARAN 28 Nov 2026 (v0.24.40): cek lama hanya memastikan 2xx +
// "#EXTM3U" di 2048 byte pertama — playlist KOSONG (0 segmen, mis.
// 8 dari 10 link dens.tv) lolos sebagai "hidup" padahal tidak pernah
// bisa diputar. Sejak v2, segmen wajib ada. Go http.Transport
// men-dekompresi gzip otomatis (Accept-Encoding ditambahkan sendiri),
// jadi playlist ter-gzip bukan masalah di sini (beda dengan probe
// Python yang harus gunzip manual).
// Referer / User-Agent per kanal dipakai bila ada (mis. ANTV,
// CNN/CNBC, BeritaSatu butuh header khusus).

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var tvProbeTimeout = 12 * time.Second

// probeStream menembak satu URL stream: true bila 2xx + playlist HLS
// berisi segmen (atau master yang variannya berisi segmen), atau
// manifest DASH valid yang tidak terenkripsi.
func probeStream(ctx context.Context, streamURL, referer, ua string) bool {
	body, ok := fetchProbe(ctx, streamURL, referer, ua)
	if !ok {
		return false
	}
	if isDashURL(streamURL) {
		return strings.Contains(body, "<MPD") &&
			!strings.Contains(body, "ContentProtection") &&
			!strings.Contains(strings.ToLower(streamURL), "cenc")
	}
	if strings.Contains(body, "#EXTINF") {
		return true
	}
	if !strings.Contains(body, "#EXT-X-STREAM-INF") {
		return false
	}
	// Master playlist: ikuti varian bandwidth tertinggi, cek segmennya.
	variant := pickVariant(body)
	if variant == "" {
		return false
	}
	variantURL := resolveProbeURL(streamURL, variant)
	vbody, ok := fetchProbe(ctx, variantURL, referer, ua)
	return ok && strings.Contains(vbody, "#EXTINF")
}

// fetchProbe mengunduh satu URL probe (maks 64KB) dan mengembalikan
// badannya sebagai string; false bila status bukan 2xx / gagal.
func fetchProbe(ctx context.Context, streamURL, referer, ua string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, tvProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", uaOrDefault(ua))
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	head, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", false
	}
	return string(head), true
}

func isDashURL(u string) bool {
	lower := strings.ToLower(u)
	return strings.Contains(lower, ".mpd")
}

// pickVariant mengambil URI varian dengan BANDWIDTH tertinggi dari
// sebuah master playlist HLS.
func pickVariant(master string) string {
	best := ""
	bestBW := -1
	lines := strings.Split(master, "\n")
	for i, ln := range lines {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "#EXT-X-STREAM-INF:") || i+1 >= len(lines) {
			continue
		}
		bw := -1
		for _, kv := range strings.Split(strings.TrimPrefix(ln, "#EXT-X-STREAM-INF:"), ",") {
			if k, v, ok := strings.Cut(kv, "="); ok && strings.TrimSpace(k) == "BANDWIDTH" {
				var n int
				if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err == nil {
					bw = n
				}
			}
		}
		uri := strings.TrimSpace(lines[i+1])
		if uri != "" && !strings.HasPrefix(uri, "#") && bw >= bestBW {
			best, bestBW = uri, bw
		}
	}
	return best
}

// resolveProbeURL menyelesaikan URI relatif varian terhadap URL master.
func resolveProbeURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	if u, err := url.Parse(base); err == nil {
		if v, err := u.Parse(ref); err == nil {
			return v.String()
		}
	}
	if i := strings.LastIndex(base, "/"); i >= 0 {
		return base[:i+1] + ref
	}
	return ref
}

func uaOrDefault(ua string) string {
	if strings.TrimSpace(ua) != "" {
		return ua
	}
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
}

// CheckTVChannels mem-probe semua kanal tv_channels (aktif maupun
// yang sedang sembunyi — supaya bisa aktif lagi) dan memperbarui
// last_checked_at / last_ok_at / fail_count / is_active.
func CheckTVChannels(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		select id, stream_url, coalesce(http_referer, ''), coalesce(http_user_agent, ''),
		       is_active, fail_count
		from tv_channels order by sort_order`)
	if err != nil {
		return err
	}
	type ch struct {
		id, url, referer, ua string
		active               bool
		fails                int
	}
	var chans []ch
	for rows.Next() {
		var c ch
		if err := rows.Scan(&c.id, &c.url, &c.referer, &c.ua, &c.active, &c.fails); err == nil {
			chans = append(chans, c)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	alive, hidden, shown := 0, 0, 0
	for _, c := range chans {
		ok := probeStream(ctx, c.url, c.referer, c.ua)
		if ok {
			alive++
			if !c.active {
				shown++
			}
			_, err = pool.Exec(ctx, `
				update tv_channels
				   set last_checked_at = now(), last_ok_at = now(),
				       fail_count = 0, is_active = true, updated_at = now()
				 where id = $1::uuid`, c.id)
		} else {
			fails := c.fails + 1
			if c.active && fails >= 3 {
				hidden++
			}
			_, err = pool.Exec(ctx, `
				update tv_channels
				   set last_checked_at = now(), fail_count = $2,
				       is_active = ($2 < 3), updated_at = now()
				 where id = $1::uuid`, c.id, fails)
		}
		if err != nil {
			log.Printf("tv health: %s: %v", c.id, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Printf("tv health: %d kanal dicek, %d hidup, %d disembunyikan, %d aktif lagi",
		len(chans), alive, hidden, shown)
	return nil
}
