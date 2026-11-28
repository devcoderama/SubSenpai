package jobs

// Health-check kanal TV (permintaan Rams 2026-11-28): tiap kanal
// di-probe langsung ke stream URL-nya — sukses = HTTP 2xx + badan
// berawalan #EXTM3U (playlist HLS). 3x gagal beruntun kanal
// disembunyikan otomatis (is_active=false); begitu hidup lagi ia
// aktif kembali sendiri. Probe jalan dari VPS (IP Indonesia), jadi
// kanal yang geo-block di luar negeri tetap terukur benar di sini.
// Referer / User-Agent per kanal dipakai bila ada (mis. ANTV,
// CNN/CNBC, BeritaSatu butuh header khusus).

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var tvProbeTimeout = 12 * time.Second

// probeStream menembak satu URL stream: true bila 2xx + playlist HLS.
func probeStream(ctx context.Context, streamURL, referer, ua string) bool {
	ctx, cancel := context.WithTimeout(ctx, tvProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", uaOrDefault(ua))
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	head, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(head)), "#EXTM3U")
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
