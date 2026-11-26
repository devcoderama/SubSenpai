package jobs

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status proses worker yang dibaca dashboard admin (kartu pantauan
// di halaman Kelola Judul — permintaan Rams 2026-11-23 malam:
// admin ingin melihat worker sedang memproses apa saat melengkapi
// metadata judul yang belum lengkap). Disimpan sebagai JSON di
// app_settings (kunci worker_status) karena worker dan API adalah
// proses terpisah — database satu-satunya tempat bersama mereka.

type WorkerStatus struct {
	Phase     string    `json:"phase"`
	Done      int       `json:"done"`
	Total     int       `json:"total"`
	Current   string    `json:"current"`
	UpdatedAt time.Time `json:"updated_at"`
	// Lanes: kemajuan per jenis saat satu fase berjalan 3 jalur
	// bersamaan (Melengkapi metadata / Mengisi episode — pemisahan
	// per jenis 2026-11-26). Kosong pada fase berurutan; dashboard
	// lama mengabaikan kolom ini.
	Lanes map[string]LaneProgress `json:"lanes,omitempty"`
}

// LaneProgress adalah kemajuan satu jalur (satu jenis konten) di
// dalam satu fase paralel.
type LaneProgress struct {
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Current string `json:"current"`
}

const workerStatusKey = "worker_status"

var progressMu sync.Mutex
var progressLast time.Time

// Keadaan jalur paralel: dijaga lanesMu agar 3 goroutine per jenis
// tidak saling menimpa. Tiap laporan memperbarui jalurnya sendiri,
// lalu satu tulis menyimpan SEMUA jalur sekaligus — menulis per
// jalur dari tiap goroutine akan saling menghapus.
var lanesMu sync.Mutex
var lanePhase string
var lanes = map[string]LaneProgress{}

// writeStatus menulis satu status worker (dibatasi maks 1 tulis per
// 2 detik, kecuali penyelesaian fase, agar tidak membebani
// database; kegagalan mencatat tidak pernah mengganggu worker).
func writeStatus(ctx context.Context, pool *pgxpool.Pool, st WorkerStatus) {
	progressMu.Lock()
	if st.Done < st.Total && time.Since(progressLast) < 2*time.Second {
		progressMu.Unlock()
		return
	}
	progressLast = time.Now()
	progressMu.Unlock()
	st.UpdatedAt = time.Now()
	raw, err := json.Marshal(st)
	if err != nil {
		return
	}
	_, _ = pool.Exec(ctx, `
		insert into app_settings (key, value) values ($1, $2)
		on conflict (key) do update set value = excluded.value, updated_at = now()`,
		workerStatusKey, string(raw))
}

// ReportProgress mencatat kemajuan worker untuk fase BERURUTAN.
// Fase berurutan tidak punya jalur: laporan ini menghapus jalur
// fase paralel sebelumnya agar kartu dashboard tidak menampilkan
// jalur basi.
func ReportProgress(ctx context.Context, pool *pgxpool.Pool, phase string, done, total int, current string) {
	lanesMu.Lock()
	lanePhase = phase
	lanes = map[string]LaneProgress{}
	lanesMu.Unlock()
	writeStatus(ctx, pool, WorkerStatus{
		Phase: phase, Done: done, Total: total, Current: current,
	})
}

// ReportLaneProgress mencatat kemajuan SATU jalur (satu jenis) di
// dalam fase paralel. Done/Total di status gabungan adalah jumlah
// ketiga jalur, sehingga kartu dashboard tetap menampilkan satu
// angka total yang koheren; rincian per jalur ada di Lanes.
func ReportLaneProgress(ctx context.Context, pool *pgxpool.Pool, phase, typ string, done, total int, current string) {
	lanesMu.Lock()
	if lanePhase != phase {
		lanePhase = phase
		lanes = map[string]LaneProgress{}
	}
	lanes[typ] = LaneProgress{Done: done, Total: total, Current: current}
	snap := make(map[string]LaneProgress, len(lanes))
	aggDone, aggTotal := 0, 0
	for k, v := range lanes {
		snap[k] = v
		aggDone += v.Done
		aggTotal += v.Total
	}
	lanesMu.Unlock()
	writeStatus(ctx, pool, WorkerStatus{
		Phase: phase, Done: aggDone, Total: aggTotal, Current: current, Lanes: snap,
	})
}

// ReadWorkerStatus membaca status terakhir (nil bila worker belum
// pernah melapor).
func ReadWorkerStatus(ctx context.Context, pool *pgxpool.Pool) *WorkerStatus {
	var raw string
	if err := pool.QueryRow(ctx,
		`select value from app_settings where key = $1`, workerStatusKey).Scan(&raw); err != nil {
		return nil
	}
	var st WorkerStatus
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil
	}
	return &st
}
