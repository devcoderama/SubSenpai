package jobs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TrimDevices mencabut perangkat aktif yang melebihi batas per akun
// (2 premium, 1 gratis) — dipakai sweeper tiap jam untuk merapikan
// akun yang premiumnya habis (2 → 1) tanpa menunggu login berikutnya.
// Yang dipertahankan: perangkat yang paling baru terlihat
// (last_seen_at terbaru). Kriteria premium sama seperti
// isPremiumAktif: langganan aktif atau premium_until di masa depan.
// Mengembalikan jumlah perangkat yang dicabut.
func TrimDevices(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	cmd, err := pool.Exec(ctx, `
		with ranked as (
			select d.user_id, d.device_id,
				row_number() over (
					partition by d.user_id
					order by d.last_seen_at desc, d.created_at desc
				) as rn,
				case when exists(
					select 1 from subscriptions s
					where s.user_id = d.user_id and s.status = 'active'
					  and (s.current_period_end is null or s.current_period_end > now())
				) or exists(
					select 1 from profiles p
					where p.id = d.user_id
					  and p.premium_until is not null and p.premium_until > now()
				) then 2 else 1 end as lim
			from user_devices d
			where d.revoked_at is null
		)
		update user_devices d set revoked_at = now()
		from ranked r
		where d.user_id = r.user_id and d.device_id = r.device_id
		  and r.rn > r.lim`)
	if err != nil {
		return 0, err
	}
	return cmd.RowsAffected(), nil
}
