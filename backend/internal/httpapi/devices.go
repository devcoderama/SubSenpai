package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"subsenpai/backend/internal/auth"
)

type deviceOut struct {
	DeviceID   string    `json:"device_id"`
	DeviceName *string   `json:"device_name,omitempty"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleDevicesList: perangkat aktif akun (halaman keamanan akun).
func (a *API) handleDevicesList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	devices := []deviceOut{}
	rows, err := a.pool.Query(r.Context(), `
		select device_id, device_name, last_seen_at, created_at
		from user_devices
		where user_id = $1::uuid and revoked_at is null
		order by last_seen_at desc`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d deviceOut
			if err := rows.Scan(&d.DeviceID, &d.DeviceName, &d.LastSeenAt, &d.CreatedAt); err == nil {
				devices = append(devices, d)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

// handleDeviceRevoke mencabut sesi satu perangkat milik akun sendiri.
func (a *API) handleDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceID == "" {
		writeErr(w, http.StatusBadRequest, "device_id wajib diisi")
		return
	}
	_, err := a.pool.Exec(r.Context(), `
		update user_devices set revoked_at = now()
		where user_id = $1::uuid and device_id = $2 and revoked_at is null`,
		p.ID, req.DeviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revoked": true})
}

// deviceLimit: batas perangkat aktif per akun — 2 untuk premium,
// 1 untuk gratis (permintaan Rams 27 Nov 2026).
func deviceLimit(premium bool) int {
	if premium {
		return 2
	}
	return 1
}

// registerDevice mencatat perangkat saat login dan menegakkan batas
// perangkat: perangkat baru yang melebihi batas MENCABUT perangkat
// aktif yang paling lama tidak aktif (last_seen_at tertua), lalu
// login baru diloloskan — bukan ditolak (perilaku lama: 409
// "batas_perangkat", dihapus 27 Nov 2026 atas permintaan Rams:
// "jika lebih akan logout salah satunya").
// Mengembalikan perangkat yang dicabut (nil bila tidak ada) dan
// false bila login harus ditolak — dalam hal ini respons error
// sudah ditulis. Dua login bersamaan dari perangkat baru yang
// berbeda diserialkan dengan advisory lock per akun supaya
// hitungan batas tidak lolos ganda.
func (a *API) registerDevice(w http.ResponseWriter, r *http.Request, userID, deviceID, deviceName string) (*deviceOut, bool) {
	if deviceID == "" {
		return nil, true
	}
	ctx := r.Context()
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mencatat perangkat")
		return nil, false
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext($1))`, "user_devices:"+userID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mencatat perangkat")
		return nil, false
	}
	var known bool
	_ = tx.QueryRow(ctx, `
		select exists(select 1 from user_devices
			where user_id = $1::uuid and device_id = $2 and revoked_at is null)`,
		userID, deviceID).Scan(&known)
	var kicked *deviceOut
	if !known {
		limit := deviceLimit(a.isPremiumAktif(userID))
		var count int
		_ = tx.QueryRow(ctx, `
			select count(*) from user_devices
			where user_id = $1::uuid and revoked_at is null`, userID).Scan(&count)
		// Biasanya menendang 1; perulangan menangani akun yang sudah
		// terlanjur kelebihan (mis. premium habis sebelum sweeper jalan).
		for count >= limit {
			var d deviceOut
			err := tx.QueryRow(ctx, `
				update user_devices set revoked_at = now()
				where (user_id, device_id) = (
					select user_id, device_id from user_devices
					where user_id = $1::uuid and revoked_at is null
					order by last_seen_at asc, created_at asc limit 1
				)
				returning device_id, device_name, last_seen_at, created_at`,
				userID).Scan(&d.DeviceID, &d.DeviceName, &d.LastSeenAt, &d.CreatedAt)
			if err != nil {
				break
			}
			if kicked == nil {
				kicked = &d
			}
			count--
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into user_devices (user_id, device_id, device_name, last_seen_at)
		values ($1::uuid, $2, $3, now())
		on conflict (user_id, device_id)
		do update set device_name = excluded.device_name,
		              last_seen_at = now(), revoked_at = null`,
		userID, deviceID, nullStr(deviceName)); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mencatat perangkat")
		return nil, false
	}
	if err := tx.Commit(ctx); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mencatat perangkat")
		return nil, false
	}
	return kicked, true
}
