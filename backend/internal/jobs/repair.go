package jobs

// Audit & perbaikan kelengkapan katalog (permintaan Rams
// 2026-11-23): worker membaca judul anime/donghua/film SATU PER
// SATU, memeriksa kelengkapannya — poster, sinopsis, tahun, genre,
// daftar episode, dan sumber putar (link embed/aliran) tiap
// episode — lalu otomatis mencari yang kurang dari sumber dan
// menyimpannya ulang ke database. Judul yang paling tidak lengkap
// dan paling banyak dicari pengguna didahulukan; judul yang baru
// dicoba masuk masa jeda agar sumber yang rusak tidak dihajar
// terus setiap siklus.

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// repairPerCycle membatasi judul yang diperbaiki per siklus worker.
const repairPerCycle = 12

type repairCandidate struct {
	id       string
	typ      string
	name     string
	score    int
	missing  []string
	epCount  int
	unsourced int
	attempts int
}

const auditColumns = `
	(case when coalesce(t.poster_url,'') = '' then 3 else 0 end
	 + case when coalesce(t.synopsis,'') = '' then 2 else 0 end
	 + case when t.year is null then 1 else 0 end
	 + case when coalesce(cardinality(t.genres),0) = 0 then 1 else 0 end
	 + case when ec.cnt = 0 then 5 else 0 end
	 + least(ec.unsourced, 10)) as score,
	coalesce(t.poster_url,'') = '' as no_poster,
	coalesce(t.synopsis,'') = '' as no_synopsis,
	t.year is null as no_year,
	coalesce(cardinality(t.genres),0) = 0 as no_genres,
	ec.cnt as ep_count,
	ec.unsourced as unsourced`

const auditFrom = `
	from titles t
	join lateral (
		select count(*) as cnt,
		       count(*) filter (
		           where not exists (select 1 from episode_sources es where es.episode_id = e.id)
		             and coalesce(e.video_url,'') = ''
		       ) as unsourced
		from episodes e where e.title_id = t.id
	) ec on true`

func (c *repairCandidate) setFlags(noPoster, noSynopsis, noYear, noGenres bool) {
	if noPoster {
		c.missing = append(c.missing, "poster")
	}
	if noSynopsis {
		c.missing = append(c.missing, "sinopsis")
	}
	if noYear {
		c.missing = append(c.missing, "tahun")
	}
	if noGenres {
		c.missing = append(c.missing, "genre")
	}
	if c.epCount == 0 {
		c.missing = append(c.missing, "episode kosong")
	} else if c.unsourced > 0 {
		c.missing = append(c.missing, fmt.Sprintf("%d episode tanpa sumber", c.unsourced))
	}
}

func scanCandidate(scan func(...any) error, c *repairCandidate) error {
	var noPoster, noSynopsis, noYear, noGenres bool
	if err := scan(&c.score, &noPoster, &noSynopsis, &noYear, &noGenres,
		&c.epCount, &c.unsourced, &c.id, &c.typ, &c.name, &c.attempts); err != nil {
		return err
	}
	c.setFlags(noPoster, noSynopsis, noYear, noGenres)
	return nil
}

// auditTitle menilai kelengkapan SATU judul (dipakai sebelum dan
// sesudah perbaikan untuk mengukur hasil).
func auditTitle(ctx context.Context, pool *pgxpool.Pool, titleID string) (repairCandidate, error) {
	var c repairCandidate
	row := pool.QueryRow(ctx, `select `+auditColumns+`, t.id, t.type, t.title, t.repair_attempts`+
		auditFrom+` where t.id = $1::uuid`, titleID)
	err := scanCandidate(row.Scan, &c)
	return c, err
}

// auditCatalog mengambil judul tidak lengkap yang paling perlu
// diperbaiki: skor kekurangan tertinggi, lalu permintaan pengguna
// (difavoritkan / ditonton) terbanyak. Judul dalam masa jeda
// perbaikan dilewati (6 jam; 7 hari bila sudah gagal 3x+).
func auditCatalog(ctx context.Context, pool *pgxpool.Pool, limit int) ([]repairCandidate, error) {
	rows, err := pool.Query(ctx, `
		select * from (
			select `+auditColumns+`, t.id, t.type, t.title, t.repair_attempts,
			       ((select count(*) from watchlist w where w.title_id = t.id)
			        + (select count(*) from watch_history wh
			           join episodes e2 on e2.id = wh.episode_id
			           where e2.title_id = t.id)) as demand
			`+auditFrom+`
			where t.type in ('anime','donghua','film')
			  and (t.last_repair_at is null
			       or (t.repair_attempts < 3 and t.last_repair_at < now() - interval '6 hours')
			       or (t.repair_attempts >= 3 and t.last_repair_at < now() - interval '7 days'))
		) s
		where s.score > 0
		order by s.score desc, s.demand desc, s.id
		limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []repairCandidate{}
	for rows.Next() {
		var c repairCandidate
		var noPoster, noSynopsis, noYear, noGenres bool
		var demand int
		if err := rows.Scan(&c.score, &noPoster, &noSynopsis, &noYear, &noGenres,
			&c.epCount, &c.unsourced, &c.id, &c.typ, &c.name, &c.attempts, &demand); err != nil {
			return nil, err
		}
		c.setFlags(noPoster, noSynopsis, noYear, noGenres)
		out = append(out, c)
	}
	return out, rows.Err()
}

// repairOneTitle menjalankan perbaikan satu judul: sinkron sumber
// (membuat pemetaan bila belum ada, mengisi episode yang hilang,
// menyelesaikan sumber episode yang kosong), lalu perkaya metadata
// dari pemetaan yang kini ada. Variabel agar dapat diganti di uji.
var repairOneTitle = func(ctx context.Context, pool *pgxpool.Pool, c repairCandidate) {
	_ = SyncOneTitle(ctx, pool, c.id, c.name, c.typ)
	_, _ = EnrichOneTitle(ctx, pool, c.id)
}

// RepairCatalog adalah langkah worker: audit katalog, perbaiki
// judul teratas satu per satu, catat hasilnya per judul dan
// ringkasan siklus. Keberhasilan diukur dari skor audit sesudah
// perbaikan yang lebih rendah daripada sebelumnya.
func RepairCatalog(ctx context.Context, pool *pgxpool.Pool) error {
	cands, err := auditCatalog(ctx, pool, repairPerCycle)
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		log.Printf("perbaikan katalog: semua judul lengkap atau dalam masa jeda")
		return nil
	}
	fixed, still := 0, 0
	for _, c := range cands {
		repairOneTitle(ctx, pool, c)
		after, err := auditTitle(ctx, pool, c.id)
		if err != nil {
			log.Printf("perbaikan %s: audit ulang gagal: %v", c.name, err)
			continue
		}
		if after.score < c.score {
			fixed++
			_, _ = pool.Exec(ctx, `
				update titles set repair_attempts = 0, last_repair_at = now()
				where id = $1::uuid`, c.id)
			log.Printf("perbaikan %s: skor %d -> %d (kurang awal: %s; sisa: %s)",
				c.name, c.score, after.score,
				strings.Join(c.missing, ", "), strings.Join(after.missing, ", "))
		} else {
			still++
			_, _ = pool.Exec(ctx, `
				update titles set repair_attempts = repair_attempts + 1, last_repair_at = now()
				where id = $1::uuid`, c.id)
			log.Printf("perbaikan %s: belum membaik (skor %d; kurang: %s)",
				c.name, c.score, strings.Join(c.missing, ", "))
		}
	}
	log.Printf("perbaikan katalog selesai: %d membaik, %d masih kurang dari %d judul diperiksa",
		fixed, still, len(cands))
	return nil
}
