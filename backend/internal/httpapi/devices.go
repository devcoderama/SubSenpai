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

// registerDevice mencatat perangkat saat login dan menegakkan batas
// 2 perangkat untuk akun premium. Mengembalikan false bila login harus
// ditolak (perangkat ketiga) — respons 409 sudah ditulis.
func (a *API) registerDevice(w http.ResponseWriter, r *http.Request, userID, deviceID, deviceName string) bool {
	if deviceID == "" {
		return true
	}
	ctx := r.Context()
	var known bool
	_ = a.pool.QueryRow(ctx, `
		select exists(select 1 from user_devices
			where user_id = $1::uuid and device_id = $2 and revoked_at is null)`,
		userID, deviceID).Scan(&known)
	if !known && a.isPremiumAktif(userID) {
		var count int
		_ = a.pool.QueryRow(ctx, `
			select count(*) from user_devices
			where user_id = $1::uuid and revoked_at is null`, userID).Scan(&count)
		if count >= 2 {
			devices := []deviceOut{}
			rows, err := a.pool.Query(ctx, `
				select device_id, device_name, last_seen_at, created_at
				from user_devices
				where user_id = $1::uuid and revoked_at is null
				order by last_seen_at desc`, userID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var d deviceOut
					if err := rows.Scan(&d.DeviceID, &d.DeviceName, &d.LastSeenAt, &d.CreatedAt); err == nil {
						devices = append(devices, d)
					}
				}
			}
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":   "batas_perangkat: akun premium aktif di maks. 2 perangkat — cabut salah satu dulu",
				"devices": devices,
			})
			return false
		}
	}
	if _, err := a.pool.Exec(ctx, `
		insert into user_devices (user_id, device_id, device_name, last_seen_at)
		values ($1::uuid, $2, $3, now())
		on conflict (user_id, device_id)
		do update set device_name = excluded.device_name,
		              last_seen_at = now(), revoked_at = null`,
		userID, deviceID, nullStr(deviceName)); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mencatat perangkat")
		return false
	}
	return true
}
