package jobs

// Notifikasi rilis (permintaan Rams 2026-11-23): pengguna yang
// memfavoritkan judul (Daftar Saya / watchlist) menerima notifikasi
// kind 'release' saat episode BARU judul itu tersinkron — tampil
// di halaman Notifikasi aplikasi bersama pengumuman admin dan
// aktivitas postingan.

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NotifyRelease menyebarkan SATU notifikasi per pengguna pemfavorit
// untuk sekumpulan episode baru dari satu judul. newEps = nomor
// episode yang baru saja disisipkan pada sinkronisasi ini.
func NotifyRelease(ctx context.Context, pool *pgxpool.Pool, titleID, titleName string, newEps []int) error {
	if len(newEps) == 0 {
		return nil
	}
	latest := newEps[0]
	for _, n := range newEps {
		if n > latest {
			latest = n
		}
	}
	var body string
	if len(newEps) == 1 {
		body = fmt.Sprintf("Episode %d baru saja rilis. Selamat menonton!", latest)
	} else {
		body = fmt.Sprintf("%d episode baru ditambahkan (terbaru: Episode %d). Selamat menonton!", len(newEps), latest)
	}
	_, err := pool.Exec(ctx, `
		insert into notifications (user_id, kind, title, body, ref_title_id)
		select distinct user_id, 'release', $2, $3, $1::uuid
		from watchlist where title_id = $1::uuid`,
		titleID, "Episode baru: "+titleName, body)
	return err
}
