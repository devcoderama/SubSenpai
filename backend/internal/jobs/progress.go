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
}

const workerStatusKey = "worker_status"

var progressMu sync.Mutex
var progressLast time.Time

// ReportProgress mencatat kemajuan worker. Dibatasi maks 1 tulis
// per 2 detik (kecuali penyelesaian fase) agar tidak membebani
// database; kegagalan mencatat tidak pernah mengganggu worker.
func ReportProgress(ctx context.Context, pool *pgxpool.Pool, phase string, done, total int, current string) {
	progressMu.Lock()
	if done < total && time.Since(progressLast) < 2*time.Second {
		progressMu.Unlock()
		return
	}
	progressLast = time.Now()
	progressMu.Unlock()
	raw, err := json.Marshal(WorkerStatus{
		Phase: phase, Done: done, Total: total,
		Current: current, UpdatedAt: time.Now(),
	})
	if err != nil {
		return
	}
	_, _ = pool.Exec(ctx, `
		insert into app_settings (key, value) values ($1, $2)
		on conflict (key) do update set value = excluded.value, updated_at = now()`,
		workerStatusKey, string(raw))
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
