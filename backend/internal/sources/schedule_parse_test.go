package sources

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Fixture Kuronime: meniru struktur terverifikasi halaman
// /jadwal-rilis/ (h3 hari + kartu anchor /anime/<slug> berisi
// poster data-lazy-src dan teks "Jam HH:MM <ep> <Judul>").
const kuroSchedFixture = `
<h3>Minggu</h3>
<div class="listupd">
<article class="bs"><div class="bsx"><a href="/anime/title-a" title="Title A"><div class="limit"><img src="data:image/svg+xml,%3Csvg%3E" data-lazy-src="https://i2.wp.com/kuronime.sbs/wp-content/uploads/a.jpg"></div><div class="tt">Jam 16:26 17 Title A</div></a></div></article>
<article class="bs"><div class="bsx"><a href="/anime/title-b" title="Title B"><div class="limit"><img src="https://i2.wp.com/kuronime.sbs/wp-content/uploads/b.jpg"></div><div class="tt">Sudah Rilis 8 Title B</div></a></div></article>
<article class="bs"><div class="bsx"><a href="/anime/title-e" title="Title E"><div class="limit"><img src="https://kuronime.sbs/wp-content/themes/animenonton/assets/images/controls-play.svg"><img width="225" height="321" src="data:image/svg+xml,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%20viewBox='0%200%20225%20321'%3E%3C/svg%3E" data-lazy-src="https://i0.wp.com/kuronime.sbs/wp-content/uploads/2026/10/e.jpg" class="attachment-post-thumbnail wp-post-image"></div><div class="tt">Jam 13:44 8 Title E</div></a></div></article>
<article class="bs"><div class="bsx"><a href="/anime/title-f" title="Title F"><div class="limit"><img src="https://kuronime.sbs/wp-content/themes/animenonton/assets/images/controls-play.svg"></div><div class="tt">Jam 22:28 8 Title F</div></a></div></article>
</div>
<h3>Jum'at</h3>
<div class="listupd">
<article class="bs"><div class="bsx"><a href="/anime/title-c" title="Title C"><div class="limit"><img src="https://i2.wp.com/kuronime.sbs/wp-content/uploads/c.jpg"></div><div class="tt">Jam 07:00 3 Title C</div></a></div></article>
</div>
<h3>Sedang Tayang</h3>
<div><a href="/anime/decoy" title="Decoy">Decoy tanpa pola jadwal</a></div>
<h3>Update Acak</h3>
<div class="listupd">
<article class="bs"><div class="bsx"><a href="https://kuronime.sbs/anime/title-d" title="Title D"><div class="limit"><img data-lazy-src="https://i2.wp.com/kuronime.sbs/wp-content/uploads/d.jpg"></div><div class="tt">?? 11 Title D</div></a></div></article>
</div>`

func TestParseKuronimeSchedule(t *testing.T) {
	groups := ParseKuronimeSchedule(kuroSchedFixture)
	if len(groups) != 3 {
		t.Fatalf("grup = %d, mau 3 (Jumat, Minggu, Update Acak): %+v", len(groups), groups)
	}
	// Urutan kanonik: Jumat (dari Jum'at) sebelum Minggu; Acak terakhir.
	if groups[0].Day != "Jumat" || groups[1].Day != "Minggu" || groups[2].Day != "Update Acak" {
		t.Fatalf("urutan hari salah: %s, %s, %s", groups[0].Day, groups[1].Day, groups[2].Day)
	}
	minggu := groups[1]
	if len(minggu.Items) != 4 {
		t.Fatalf("item Minggu = %d, mau 4", len(minggu.Items))
	}
	// Kartu E (markup asli halaman 2026-11-23): gambar overlay tema
	// controls-play.svg mendahului poster; poster harus diambil
	// dari data-lazy-src, bukan dari gambar pertama.
	e := minggu.Items[2]
	if e.Slug != "title-e" || e.Poster != "https://i0.wp.com/kuronime.sbs/wp-content/uploads/2026/10/e.jpg" {
		t.Fatalf("poster E harus dari data-lazy-src poster kedua: %+v", e)
	}
	// Kartu F hanya punya ikon tema: poster harus kosong, bukan .svg.
	if f := minggu.Items[3]; f.Slug != "title-f" || f.Poster != "" {
		t.Fatalf("poster F harus kosong: %+v", f)
	}
	a := minggu.Items[0]
	if a.Slug != "title-a" || a.Title != "Title A" || a.Time != "16:26" || a.Episode != 17 {
		t.Fatalf("item A salah: %+v", a)
	}
	if a.Poster != "https://i2.wp.com/kuronime.sbs/wp-content/uploads/a.jpg" {
		t.Fatalf("poster A harus dari data-lazy-src: %q", a.Poster)
	}
	b := minggu.Items[1]
	if b.Time != "Sudah Rilis" || b.Episode != 8 {
		t.Fatalf("item B salah: %+v", b)
	}
	acak := groups[2]
	if len(acak.Items) != 1 || acak.Items[0].Slug != "title-d" || acak.Items[0].Time != "??" || acak.Items[0].Episode != 11 {
		t.Fatalf("grup acak salah: %+v", acak)
	}
}

// Fixture Anichin: potongan markup asli anichin.moe/schedule/
// (kotak sch_monday + sch_tuesday, kartu .bsx).
const anichinSchedFixture = `
<div class="bixbox schedulepage sch_monday"><div class="releases"><h3><span>Senin</span></h3></div><div class="listupd"><div class="bs"><div class="bsx"><a href="/one-slash-to-the-heavens/" title="One Slash to the Heavens"><div class="limit"><div class="bt"> <span class='epx cndwn' data-cndwn='12577' data-rlsdt='1795381446'>Jam 04:04</span><span class="sb Sub">19</span></div><img width="225" height="321" src="/wp-content/uploads/2026/07/One-Slash.webp" class="attachment-post-thumbnail wp-post-image"></div><div class="tt">One Slash to the Heavens</div></a></div></div><div class="bs"><div class="bsx"><a href="/renegade-immortal/" title="Renegade Immortal"><div class="limit"><div class="bt"> <span class='epx cndwn' data-cndwn='33944' data-rlsdt='1795402813'>Jam 11:15</span><span class="sb Sub">169</span></div><img width="225" height="321" src="/wp-content/uploads/2022/03/Renegade.jpg" class="attachment-post-thumbnail wp-post-image"></div><div class="tt">Renegade Immortal</div></a></div></div></div></div>
<div class="bixbox schedulepage sch_tuesday"><div class="releases"><h3><span>Selasa</span></h3></div><div class="listupd"><div class="bs"><div class="bsx"><a href="/soul-land/" title="Soul Land"><div class="limit"><div class="bt"> <span class='epx cndwn'>Jam 20:00</span><span class="sb Sub">42</span></div><img src="https://anichin.moe/wp-content/uploads/soul.jpg"></div><div class="tt">Soul Land</div></a></div></div></div></div>`

func TestParseAnichinSchedule(t *testing.T) {
	groups := ParseAnichinSchedule(anichinSchedFixture)
	if len(groups) != 2 {
		t.Fatalf("grup = %d, mau 2: %+v", len(groups), groups)
	}
	if groups[0].Day != "Senin" || groups[1].Day != "Selasa" {
		t.Fatalf("hari salah: %s, %s", groups[0].Day, groups[1].Day)
	}
	senin := groups[0]
	if len(senin.Items) != 2 {
		t.Fatalf("item Senin = %d, mau 2", len(senin.Items))
	}
	a := senin.Items[0]
	if a.Slug != "one-slash-to-the-heavens" || a.Time != "04:04" || a.Episode != 19 {
		t.Fatalf("item A salah: %+v", a)
	}
	if a.Poster != "https://anichin.moe/wp-content/uploads/2026/07/One-Slash.webp" {
		t.Fatalf("poster A harus absolut: %q", a.Poster)
	}
	if senin.Items[1].Episode != 169 {
		t.Fatalf("episode Renegade = %d, mau 169", senin.Items[1].Episode)
	}
}

// TestAnichinScheduleLive: ambil halaman asli (hanya dengan
// SUBSENPAI_LIVE_TEST=1). Kuronime tidak diuji hidup dari sini —
// Cloudflare menolak IP VM; parser-nya diuji via fixture dan
// diverifikasi lewat API produksi setelah deploy.
func TestAnichinScheduleLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	groups, err := FetchAnichinSchedule(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, g := range groups {
		total += len(g.Items)
	}
	if len(groups) < 5 || total < 20 {
		t.Fatalf("jadwal hidup mencurigakan: %d grup, %d item", len(groups), total)
	}
	if !strings.Contains(groups[0].Items[0].Poster, "anichin.moe") &&
		groups[0].Items[0].Poster != "" {
		t.Fatalf("poster item pertama janggal: %q", groups[0].Items[0].Poster)
	}
}
