package jobs

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/billing"
	"subsenpai/backend/internal/payments"
)

// PollGoPay mencocokkan sesi GoPay pending dengan transaksi masuk
// merchant (nominal unik + jendela waktu sesi), lalu mengaktifkannya
// lewat billing.ActivateSession. Tanpa kredensial merchant, job diam.
func PollGoPay(ctx context.Context, pool *pgxpool.Pool, client *payments.GoPayClient) {
	if client == nil || !client.Configured() {
		return
	}
	type sess struct {
		id      string
		amount  int
		created time.Time
		expires time.Time
	}
	rows, err := pool.Query(ctx, `
		select id, amount, created_at, expires_at from payment_sessions
		where provider = 'gopay' and status = 'pending'
		  and expires_at > now() - interval '5 minutes'`)
	if err != nil {
		log.Printf("polling GoPay: %v", err)
		return
	}
	var sessions []sess
	for rows.Next() {
		var s sess
		if err := rows.Scan(&s.id, &s.amount, &s.created, &s.expires); err == nil {
			sessions = append(sessions, s)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(sessions) == 0 {
		return
	}
	txs, err := client.Transactions(ctx)
	if err != nil {
		log.Printf("polling GoPay: ambil transaksi: %v", err)
		return
	}
	matched := 0
	for _, s := range sessions {
		if ref, ok := payments.Match(txs, s.amount, s.created, s.expires); ok {
			if err := billing.ActivateSession(ctx, pool, s.id, ref); err != nil {
				log.Printf("polling GoPay: aktivasi %s: %v", s.id, err)
				continue
			}
			matched++
		}
	}
	if matched > 0 {
		log.Printf("polling GoPay: %d sesi teraktivasi", matched)
	}
}

// activateIfPaid menanyakan status order ke Tokopay lalu mengaktifkan
// sesi bila gateway menyatakan lunas. Mengembalikan true bila sesi
// diaktifkan; false + err != nil bila pengecekan gagal (penelepon
// yang memutuskan: lewati dulu, atau anggap belum lunas). Aktivasi
// sendiri atomik (billing.ActivateSession) — dua penelepon yang
// bersamaan tidak bisa memperpanjang premium dua kali.
func activateIfPaid(ctx context.Context, pool *pgxpool.Pool, tokopay *payments.TokopayClient, id, trx string) (bool, error) {
	st, err := tokopay.CheckOrderStatus(ctx, trx, strings.ReplaceAll(id, "-", ""))
	if err != nil {
		return false, err
	}
	if !payments.TokopayPaid(st) {
		return false, nil
	}
	if err := billing.ActivateSession(ctx, pool, id, trx); err != nil {
		return false, err
	}
	return true, nil
}

// ReconcileTokopayPayments adalah jaring pengaman webhook (permintaan
// Rams 2026-11-26): tiap menit, sesi Tokopay yang masih pending tapi
// dibuat <24 jam lalu ditanyakan ke gateway; yang ternyata sudah lunas
// diaktifkan — mis. callback terlewat karena webhook masih menunjuk
// domain lama, atau request callback gagal di tengah jalan. Sesi yang
// lebih tua dari 24 jam ditangani ExpireStaleSessions, di sini tidak
// disentuh. Anti-dobel: bila webhook sudah mengaktifkan lebih dulu,
// sesi sudah 'paid' dan dilewati; bila job ini yang menang, webhook
// yang datang belakangan jadi no-op (lihat ActivateSession).
func ReconcileTokopayPayments(ctx context.Context, pool *pgxpool.Pool, tokopay *payments.TokopayClient) {
	if tokopay == nil || !tokopay.Configured() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	const batch = 20
	type sess struct {
		id  string
		trx string
	}
	rows, err := pool.Query(ctx, `
		select id, matched_ref
		from payment_sessions
		where provider = 'tokopay' and status = 'pending'
		  and matched_ref is not null
		  and created_at >= now() - interval '24 hours'
		order by created_at asc limit $1`, batch)
	if err != nil {
		log.Printf("rekonsiliasi Tokopay: %v", err)
		return
	}
	var sessions []sess
	for rows.Next() {
		var s sess
		if err := rows.Scan(&s.id, &s.trx); err == nil {
			sessions = append(sessions, s)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(sessions) == 0 {
		return
	}
	activated := 0
	for _, s := range sessions {
		ok, err := activateIfPaid(ctx, pool, tokopay, s.id, s.trx)
		if err != nil {
			log.Printf("rekonsiliasi Tokopay: cek %s: %v", s.id, err)
			continue
		}
		if ok {
			activated++
		}
	}
	if activated > 0 {
		log.Printf("rekonsiliasi Tokopay: %d sesi lunas teraktivasi", activated)
	}
}

// ExpireStaleSessions menandai sesi pembayaran yang sudah lebih dari
// 24 jam masih pending sebagai 'expired' agar tidak menumpuk
// selamanya di dashboard (permintaan Rams 2026-11-26).
//
// Tokopay TIDAK punya endpoint cancel — order mati sendiri setelah
// masa berlakunya habis — jadi yang dilakukan di sini adalah
// pencerminan di sisi kita, DENGAN verifikasi dulu via check-order:
// bila gateway ternyata sudah lunas (mis. callback terlewat karena
// webhook masih menunjuk domain lama), sesi justru DIAKTIFKAN lewat
// billing.ActivateSession. Bila pengecekan gagal (gateway tidak bisa
// dihubungi), sesi dilewati dan dicoba lagi di siklus berikutnya —
// jangan pernah meng-expire sesi yang status bayarnya belum pasti.
// GoPay tidak punya API cek status dan jendela pencocokannya
// (15 mnt + 5 mnt) sudah lama tertutup, jadi sesi GoPay kedaluwarsa
// langsung berdasarkan umur. Provider lain (midtrans/xendit)
// dilewati — tidak ada klien cek statusnya.
//
// Aman terhadap pembayaran telat: ActivateSession tidak mensyaratkan
// status 'pending' (hanya 'paid' yang no-op), jadi bila user membayar
// sesudah sesi ditandai expired, webhook tetap mengaktifkannya.
func ExpireStaleSessions(ctx context.Context, pool *pgxpool.Pool, tokopay *payments.TokopayClient) {
	const staleAfter = 24 * time.Hour
	const batch = 50
	type sess struct {
		id       string
		provider string
		trx      string
	}
	rows, err := pool.Query(ctx, `
		select id, provider, coalesce(matched_ref, '')
		from payment_sessions
		where status = 'pending' and created_at < now() - interval '24 hours'
		order by created_at asc limit $1`, batch)
	if err != nil {
		log.Printf("expire sesi basi: %v", err)
		return
	}
	var sessions []sess
	for rows.Next() {
		var s sess
		if err := rows.Scan(&s.id, &s.provider, &s.trx); err == nil {
			sessions = append(sessions, s)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(sessions) == 0 {
		return
	}
	expired, activated, skipped := 0, 0, 0
	expire := func(id string) {
		res, err := pool.Exec(ctx, `
			update payment_sessions set status = 'expired'
			where id = $1::uuid and status = 'pending'`, id)
		if err == nil && res.RowsAffected() > 0 {
			expired++
		}
	}
	for _, s := range sessions {
		switch s.provider {
		case "tokopay":
			if tokopay == nil || !tokopay.Configured() {
				skipped++
				continue
			}
			ok, err := activateIfPaid(ctx, pool, tokopay, s.id, s.trx)
			if err != nil {
				log.Printf("expire sesi basi: cek %s: %v", s.id, err)
				skipped++
				continue
			}
			if ok {
				activated++
				continue
			}
			expire(s.id)
		case "gopay":
			expire(s.id)
		default:
			skipped++
		}
	}
	if expired > 0 || activated > 0 {
		log.Printf("expire sesi basi: %d kedaluwarsa, %d teraktivasi (telat bayar), %d dilewati", expired, activated, skipped)
	}
}
