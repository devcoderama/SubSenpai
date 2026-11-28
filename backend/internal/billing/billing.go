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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// dbTX adalah permukaan database minimal yang dipakai logika billing:
// *pgxpool.Pool dan pgx.Tx sama-sama memenuhinya, sehingga aktivasi
// bisa berjalan di dalam transaksi tanpa mengubah pemanggil — dan
// tanpa mengambil koneksi kedua dari pool (yang akan deadlock saat
// beberapa aktivasi berjalan bersamaan).
type dbTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Plan adalah paket premium yang dijual (harga rupiah).
// Price adalah harga yang DITAGIH; OriginalPrice + DiscountPercent
// hanya pajangan (harga coret + badge -X%). Keduanya 0 = tanpa diskon.
type Plan struct {
	ID              string `json:"id"`
	Days            int    `json:"days"`
	Price           int    `json:"price"`
	Label           string `json:"label"`
	OriginalPrice   int    `json:"original_price"`
	DiscountPercent int    `json:"discount_percent"`
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
func LoadPlans(ctx context.Context, db dbTX) []Plan {
	rows, err := db.Query(ctx, `
		select id, days, price, label, coalesce(original_price, 0), coalesce(discount_percent, 0)
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
		if err := rows.Scan(&p.ID, &p.Days, &p.Price, &p.Label, &p.OriginalPrice, &p.DiscountPercent); err != nil {
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
func PlanByIDCtx(ctx context.Context, db dbTX, id string) (Plan, bool) {
	for _, p := range LoadPlans(ctx, db) {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// ExtendPremium menambah masa premium (menumpuk dari premium_until yang
// masih berlaku) dan mencatat baris subscriptions.
func ExtendPremium(ctx context.Context, pool *pgxpool.Pool, userID string, days int, provider, ref string) (time.Time, error) {
	return ExtendPremiumTx(ctx, pool, userID, days, provider, ref)
}

// ExtendPremiumTx sama seperti ExtendPremium, tapi di atas transaksi
// pemanggil — dipakai ActivateSession agar klaim sesi + perpanjangan
// premium berhasil atau gagal bersama-sama (bila perpanjangan gagal,
// klaim dibatalkan sehingga webhook/scheduler berikutnya bisa mencoba
// lagi; tanpa ini sesi bisa tertandai lunas tanpa premium).
func ExtendPremiumTx(ctx context.Context, tx dbTX, userID string, days int, provider, ref string) (time.Time, error) {
	var until time.Time
	err := tx.QueryRow(ctx, `
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
	_, err = tx.Exec(ctx, `
		insert into subscriptions (user_id, plan, status, current_period_end, provider, provider_ref)
		values ($1::uuid, $2, 'active', $3, $4, $5)`,
		userID, plan, until, provider, nullStr(ref))
	return until, err
}

// ActivateSession menandai sesi pembayaran sebagai lunas dan menjalankan
// efeknya: langganan diperpanjang untuk purpose 'subscription'; untuk
// purpose 'voucher' tidak ada efek langsung (voucher yang payment_ref-nya
// menunjuk sesi ini menjadi bisa ditukar). Aman dipanggil berulang.
//
// ANTI-DOBEL (permintaan Rams 2026-11-26): seluruh aktivasi berjalan
// dalam SATU transaksi dengan baris sesi dikunci (SELECT ... FOR
// UPDATE). Webhook callback dan job rekonsiliasi terjadwal yang tiba
// bersamaan saling antre pada kunci itu; yang kedua melihat status
// sudah 'paid' dan berhenti — premium tidak pernah diperpanjang dua
// kali untuk satu pembayaran, dan voucher tidak pernah terhitung dua
// kali. Bila perpanjangan gagal, transaksi dibatalkan sehingga sesi
// tetap 'pending' dan percobaan berikutnya (webhook ulang / scheduler)
// masih bisa mengaktifkannya.
func ActivateSession(ctx context.Context, pool *pgxpool.Pool, sessionID, matchedRef string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID, purpose, planID, provider, status string
	err = tx.QueryRow(ctx, `
		select user_id, purpose, coalesce(plan,''), provider, status
		from payment_sessions where id = $1::uuid for update`, sessionID,
	).Scan(&userID, &purpose, &planID, &provider, &status)
	if err != nil {
		return fmt.Errorf("sesi tidak ditemukan: %w", err)
	}
	if status == "paid" {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		update payment_sessions
		set status = 'paid', matched_ref = $2
		where id = $1::uuid`, sessionID, nullStr(matchedRef)); err != nil {
		return err
	}
	if purpose == "subscription" {
		plan, ok := PlanByIDCtx(ctx, tx, planID)
		if !ok {
			return fmt.Errorf("paket sesi tidak dikenal: %q", planID)
		}
		if _, err := ExtendPremiumTx(ctx, tx, userID, plan.Days, provider, sessionID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
