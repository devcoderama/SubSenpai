package httpapi

// Jalur on-demand (permintaan Rams 2026-11-22): bila user membuka
// judul yang detailnya belum dimuat worker, API menarik langsung
// dari sumber saat itu juga (lewat fungsi per-judul paket jobs),
// menyimpan ke DB, dan mengembalikan detail lengkap — tanpa
// menunggu siklus worker. Pencarian yang kosong di DB juga jatuh
// ke pencarian live sumber. Pengaman: kunci per judul di memori
// (permintaan bersamaan hanya memicu satu tarikan), timeout pendek
// untuk percobaan di depan user, penyelesaian latar belakang bila
// percobaan depan gagal/timeout, dan cooldown percobaan gagal agar
// sumber yang sedang down tidak dihajar berulang. Semua state
// in-memory — tanpa migrasi skema. Judul mode device (Kuronime/
// komik) TIDAK disentuh jalur ini: server memang tidak bisa
// menarik dari sumber ber-Cloudflare itu.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"subsenpai/backend/internal/jobs"
	"subsenpai/backend/internal/sources"
)

const (
	// onDemandTimeout membatasi percobaan pengisian yang dilakukan
	// selagi user menunggu respons detail.
	onDemandTimeout = 10 * time.Second
	// onDemandBackgroundTimeout membatasi penyelesaian latar
	// belakang setelah percobaan depan gagal/timeout.
	onDemandBackgroundTimeout = 2 * time.Minute
	// onDemandCooldown adalah jeda sebelum judul (atau kata
	// pencarian) yang tarikannya gagal boleh dicoba lagi.
	onDemandCooldown = 10 * time.Minute
)

// fillGuard adalah kunci per kunci + cooldown percobaan gagal,
// seluruhnya di memori proses API.
type fillGuard struct {
	mu            sync.Mutex
	inflight      map[string]bool
	cooldownUntil map[string]time.Time
	cooldownDur   time.Duration
	now           func() time.Time
}

func newFillGuard(cooldown time.Duration) *fillGuard {
	return &fillGuard{
		inflight:      make(map[string]bool),
		cooldownUntil: make(map[string]time.Time),
		cooldownDur:   cooldown,
		now:           time.Now,
	}
}

// tryStart mengklaim hak menarik untuk kunci ini. False bila ada
// tarikan lain yang sedang berjalan untuk kunci yang sama, atau
// kunci sedang dalam masa cooldown sesudah kegagalan.
func (g *fillGuard) tryStart(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inflight[key] {
		return false
	}
	if until, ok := g.cooldownUntil[key]; ok && g.now().Before(until) {
		return false
	}
	g.inflight[key] = true
	return true
}

// finish melepas klaim; failed=true memasang cooldown.
func (g *fillGuard) finish(key string, failed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.inflight, key)
	if failed {
		g.cooldownUntil[key] = g.now().Add(g.cooldownDur)
	} else {
		delete(g.cooldownUntil, key)
	}
}

// needsOnDemand menentukan aspek detail judul yang masih kosong
// dan layak diisi on-demand. Hanya judul video mode server: judul
// device tidak punya pemetaan sumber server, dan jenis komik
// dibaca di perangkat (F39). Sinopsis boilerplate iklan unduhan
// (data lama dari halaman salah alamat) dianggap kosong agar
// detail yang dibuka user langsung disembuhkan on-demand.
func needsOnDemand(fetchMode, typ, synopsis string, episodeCount int) (needMeta, needEps bool) {
	if fetchMode != "server" {
		return false, false
	}
	switch typ {
	case "anime", "donghua", "film":
	default:
		return false, false
	}
	needMeta = synopsis == "" || sources.IsBoilerplateSynopsis(synopsis)
	needEps = episodeCount == 0
	return needMeta, needEps
}

var errNoProgress = errors.New("on-demand: sumber tidak memberi kemajuan")

// fillTitle menjalankan pengisian satu judul sesuai aspek yang
// dibutuhkan. Nil hanya bila ada kemajuan nyata (metadata tertulis
// atau episode kini ada); selain itu error agar pemanggil bisa
// memasang cooldown / mencoba lagi di latar belakang.
func (a *API) fillTitle(ctx context.Context, titleID string, needMeta, needEps bool) error {
	var name, typ string
	if err := a.pool.QueryRow(ctx, `
		select title, type from titles where id = $1::uuid`, titleID).Scan(&name, &typ); err != nil {
		return err
	}
	progress := false
	var lastErr error
	if needMeta {
		updated, err := jobs.EnrichOneTitle(ctx, a.pool, titleID)
		if err != nil {
			lastErr = err
		} else if updated {
			progress = true
		}
	}
	if needEps {
		if err := jobs.SyncOneTitle(ctx, a.pool, titleID, name, typ); err != nil {
			lastErr = err
		}
		var n int
		if err := a.pool.QueryRow(ctx, `
			select count(*) from episodes where title_id = $1::uuid`, titleID).Scan(&n); err == nil && n > 0 {
			progress = true
		}
	}
	if progress {
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return errNoProgress
}

// fillOnDemand mencoba mengisi judul selagi user menunggu (timeout
// pendek). Mengembalikan true bila pemanggil sebaiknya membaca
// ulang detail dari DB (ada kemungkinan kemajuan). Bila percobaan
// depan gagal/timeout, penyelesaian dilanjutkan di goroutine latar
// dengan konteks sendiri — tetap di bawah kunci per judul yang
// sama, dan cooldown dipasang bila itu pun gagal.
func (a *API) fillOnDemand(r *http.Request, titleID string, needMeta, needEps bool) bool {
	if !a.guard.tryStart(titleID) {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), onDemandTimeout)
	err := a.fillTitle(ctx, titleID, needMeta, needEps)
	cancel()
	if err == nil {
		a.guard.finish(titleID, false)
		return true
	}
	go func() {
		bctx, bcancel := context.WithTimeout(context.Background(), onDemandBackgroundTimeout)
		defer bcancel()
		berr := a.fillTitle(bctx, titleID, needMeta, needEps)
		a.guard.finish(titleID, berr != nil)
	}()
	return true
}

// liveSearch menarik pencarian live dari sumber server aktif dan
// menyimpan kandidat baru ke katalog (jobs.LiveSearchStore). typ
// kosong berarti semua jenis video. Kunci guard per jenis+kata
// memakai cooldown yang sama: kata yang barusan gagal/tidak
// menghasilkan apa-apa tidak menghajar sumber berulang kali.
func (a *API) liveSearch(ctx context.Context, typ, q string) {
	types := []string{typ}
	if typ == "" {
		types = []string{"anime", "donghua", "film"}
	}
	for _, t := range types {
		key := "search|" + t + "|" + strings.ToLower(strings.TrimSpace(q))
		if !a.guard.tryStart(key) {
			continue
		}
		n, err := jobs.LiveSearchStore(ctx, a.pool, t, q)
		a.guard.finish(key, err != nil || n == 0)
	}
}
