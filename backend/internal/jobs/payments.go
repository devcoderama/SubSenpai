package jobs

import (
	"context"
	"log"
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
