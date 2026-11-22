// Package jobs berisi pekerjaan latar worker.
package jobs

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RecomputeTrending menghitung skor tren (O10) dari aktivitas 24 jam &
// 7 hari terakhir, lalu menyimpannya ke trending_cache agar endpoint
// beranda tinggal membaca hasil jadi.
//
// Skor sederhana Fase 1: tontonan + bacaan diberi bobot 1, rating rata-rata
// judul (dari title_ratings) menjadi pengali kecil agar judul yang disukai
// naik. Rumus bisa diperhalus tanpa mengubah kontrak tabel.
func RecomputeTrending(ctx context.Context, pool *pgxpool.Pool) error {
	for _, w := range []struct {
		window   string
		interval string
	}{
		{"day", "24 hours"},
		{"week", "7 days"},
	} {
		_, err := pool.Exec(ctx, `
			insert into trending_cache (title_id, score, "window", computed_at)
			select t.id,
			       (count(distinct wh.user_id) + count(distinct rh.user_id)) *
			       (1 + coalesce(avg(tr.score), 0) / 20.0) as score,
			       $1, now()
			from titles t
			left join episodes e on e.title_id = t.id
			left join chapters c on c.title_id = t.id
			left join watch_history wh on wh.episode_id = e.id
				and wh.updated_at > now() - $2::interval
			left join read_history rh on rh.chapter_id = c.id
				and rh.updated_at > now() - $2::interval
			left join title_ratings tr on tr.title_id = t.id
			group by t.id
			having count(distinct wh.user_id) + count(distinct rh.user_id) > 0
			on conflict (title_id, "window") do update
				set score = excluded.score, computed_at = excluded.computed_at`,
			w.window, w.interval)
		if err != nil {
			return err
		}
	}
	log.Println("skor tren diperbarui")
	return nil
}
