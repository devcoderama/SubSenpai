// Package billing berisi logika premium bersama: daftar paket, aktivasi
// sesi pembayaran, dan perpanjangan masa premium. Dipakai handler API
// (checkout/webhook/voucher/trial) dan worker (polling GoPay) agar semua
// jalur aktivasi konsisten: premium selalu dicatat di subscriptions dan
// dicerminkan ke profiles.premium_until/is_premium.
package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Plan adalah paket premium yang dijual (harga rupiah).
type Plan struct {
	ID    string `json:"id"`
	Days  int    `json:"days"`
	Price int    `json:"price"`
	Label string `json:"label"`
}

// Plans adalah daftar bawaan yang dipakai HANYA bila tabel
// subscription_plans belum ada/kosong (mis. migrasi 0014 belum
// berjalan). Sumber kebenaran harga adalah database.
var Plans = []Plan{
	{ID: "premium-30", Days: 30, Price: 8000, Label: "Premium 1 Bulan"},
	{ID: "premium-90", Days: 90, Price: 21000, Label: "Premium 3 Bulan"},
	{ID: "premium-180", Days: 180, Price: 39000, Label: "Premium 6 Bulan"},
	{ID: "premium-365", Days: 365, Price: 72000, Label: "Premium 12 Bulan"},
	{ID: "premium-1095", Days: 1095, Price: 180000, Label: "Premium 3 Tahun"},
}

func PlanByID(id string) (Plan, bool) {
	for _, p := range Plans {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// LoadPlans membaca katalog paket aktif dari tabel
// subscription_plans; jatuh ke daftar bawaan bila tabel belum
// ada atau masih kosong.
func LoadPlans(ctx context.Context, pool *pgxpool.Pool) []Plan {
	rows, err := pool.Query(ctx, `
		select id, days, price, label
		from subscription_plans
		where is_active
		order by sort_order, days`)
	if err != nil {
		return Plans
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Days, &p.Price, &p.Label); err != nil {
			return Plans
		}
		out = append(out, p)
	}
	if rows.Err() != nil || len(out) == 0 {
		return Plans
	}
	return out
}

// PlanByIDCtx mencari paket dari database (sumber kebenaran).
func PlanByIDCtx(ctx context.Context, pool *pgxpool.Pool, id string) (Plan, bool) {
	for _, p := range LoadPlans(ctx, pool) {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// ExtendPremium menambah masa premium (menumpuk dari premium_until yang
// masih berlaku) dan mencatat baris subscriptions.
func ExtendPremium(ctx context.Context, pool *pgxpool.Pool, userID string, days int, provider, ref string) (time.Time, error) {
	var until time.Time
	err := pool.QueryRow(ctx, `
		update profiles
		set premium_until = greatest(coalesce(premium_until, now()), now()) + make_interval(days => $2),
		    is_premium = true
		where id = $1::uuid
		returning premium_until`, userID, days).Scan(&until)
	if err != nil {
		return time.Time{}, err
	}
	plan := fmt.Sprintf("premium-%d", days)
	if provider == "trial" {
		plan = "trial-3"
	}
	_, err = pool.Exec(ctx, `
		insert into subscriptions (user_id, plan, status, current_period_end, provider, provider_ref)
		values ($1::uuid, $2, 'active', $3, $4, $5)`,
		userID, plan, until, provider, nullStr(ref))
	return until, err
}

// ActivateSession menandai sesi pembayaran sebagai lunas dan menjalankan
// efeknya: langganan diperpanjang untuk purpose 'subscription'; untuk
// purpose 'voucher' tidak ada efek langsung (voucher yang payment_ref-nya
// menunjuk sesi ini menjadi bisa ditukar). Aman dipanggil berulang.
func ActivateSession(ctx context.Context, pool *pgxpool.Pool, sessionID, matchedRef string) error {
	var userID, purpose, planID, provider, status string
	err := pool.QueryRow(ctx, `
		select user_id, purpose, coalesce(plan,''), provider, status
		from payment_sessions where id = $1::uuid`, sessionID,
	).Scan(&userID, &purpose, &planID, &provider, &status)
	if err != nil {
		return fmt.Errorf("sesi tidak ditemukan: %w", err)
	}
	if status == "paid" {
		return nil
	}
	if _, err := pool.Exec(ctx, `
		update payment_sessions
		set status = 'paid', matched_ref = $2
		where id = $1::uuid`, sessionID, nullStr(matchedRef)); err != nil {
		return err
	}
	if purpose == "subscription" {
		plan, ok := PlanByIDCtx(ctx, pool, planID)
		if !ok {
			return fmt.Errorf("paket sesi tidak dikenal: %q", planID)
		}
		_, err := ExtendPremium(ctx, pool, userID, plan.Days, provider, sessionID)
		return err
	}
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
