package sources

import "testing"

// Uji pengurai penemuan katalog dengan contoh respons asli yang
// diperkecil (tanpa jaringan).

func TestParseMTLatest(t *testing.T) {
	raw := `{"items":[
		{"title":"Solo Leveling","mangaUrl":"https://manhuarmtl.com/manga/solo-leveling/","thumbnail":"https://manhuarmtl.com/wp-content/uploads/Solo-Leveling.webp","type":"manhwa","status":"end","year":2018,"genreSlugs":["action","martial-arts"]},
		{"title":"Martial Peak","mangaUrl":"https://manhuarmtl.com/manga/martial-peak/","thumbnail":"","type":"manhua","status":"ongoing","year":2017,"genreSlugs":[]},
		{"title":"Tanpa Type","mangaUrl":"https://manhuarmtl.com/manga/tanpa-type/","thumbnail":"","type":"","status":"ongoing","year":0,"genreSlugs":[]}
	],"total_pages":17}`
	out, pages, err := parseMTLatest(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pages != 17 {
		t.Errorf("total_pages = %d, mau 17", pages)
	}
	if len(out) != 2 {
		t.Fatalf("judul terbaca = %d, mau 2 (type kosong dilewati)", len(out))
	}
	sl := out[0]
	if sl.Slug != "solo-leveling" || sl.Type != "manhwa" || !sl.Completed || sl.Year != 2018 {
		t.Errorf("solo leveling salah urai: %+v", sl)
	}
	if sl.Poster == "" || len(sl.Genres) != 2 || sl.Genres[1] != "Martial arts" {
		t.Errorf("poster/genre salah: %+v", sl)
	}
	if out[1].Completed {
		t.Errorf("martial peak seharusnya belum tamat")
	}
}

func TestParsePosterMap(t *testing.T) {
	page := `<div class="listupd">
		<img src="/wp-content/uploads/2024/10/Martial-Master.webp" class="ts-post-image wp-post-image" title="Martial Master Episode 710 Subtitle Indonesia" alt="Martial Master Episode 710"/>
		<img data-src="https://anichin.moe/wp-content/uploads/Perfect-World.webp" src="data:image/gif;base64,xxx" class="ts-post-image" title="Perfect World Episode 240 Subtitle Indonesia"/>
		<img src="/logo.png" class="lain" title="Bukan Kartu"/>
	</div>`
	m := parsePosterMap(page, "https://anichin.moe")
	if len(m) != 2 {
		t.Fatalf("peta poster = %d entri, mau 2: %v", len(m), m)
	}
	if got := m["martial master"]; got != "https://anichin.moe/wp-content/uploads/2024/10/Martial-Master.webp" {
		t.Errorf("martial master poster = %q", got)
	}
	if got := m["perfect world"]; got != "https://anichin.moe/wp-content/uploads/Perfect-World.webp" {
		t.Errorf("perfect world poster = %q (data-src harus dipakai)", got)
	}
}

func TestParsePosterMapKuronime(t *testing.T) {
	// Halaman daftar Kuronime memakai tema Animestream yang sama
	// dengan Anichin — kartu episode membawa poster serinya.
	page := `<div class="listupd">
		<img src="/wp-content/uploads/2024/01/Baka-to-Test.webp" class="ts-post-image wp-post-image" title="Baka to Test to Shoukanjuu Episode 3 Subtitle Indonesia" alt="Baka to Test to Shoukanjuu Episode 3"/>
		<img data-src="https://kuronime.sbs/wp-content/uploads/Jitsu-wa.webp" src="data:image/gif;base64,xxx" class="ts-post-image" title="Jitsu wa Watashi wa Episode 5 Subtitle Indonesia"/>
		<img src="/logo.png" class="lain" title="Bukan Kartu"/>
	</div>`
	m := parsePosterMap(page, "https://kuronime.sbs")
	if len(m) != 2 {
		t.Fatalf("peta poster kuronime = %d entri, mau 2: %v", len(m), m)
	}
	if got := m["baka to test to shoukanjuu"]; got != "https://kuronime.sbs/wp-content/uploads/2024/01/Baka-to-Test.webp" {
		t.Errorf("baka to test poster = %q (URL relatif harus dibuat absolut ke base kuronime)", got)
	}
	if got := m["jitsu wa watashi wa"]; got != "https://kuronime.sbs/wp-content/uploads/Jitsu-wa.webp" {
		t.Errorf("jitsu wa poster = %q (data-src harus dipakai)", got)
	}
}

func TestPosterMapBase(t *testing.T) {
	anichin := Source{BaseURL: "https://anichin.my"}
	if base, ok := posterMapBase("anichin", anichin); !ok || base != "https://anichin.moe" {
		t.Errorf("anichin base = %q, %v — mau mirror anichin.moe", base, ok)
	}
	kuro := Source{BaseURL: "https://kuronime.sbs/"}
	if base, ok := posterMapBase("kuronime", kuro); !ok || base != "https://kuronime.sbs" {
		t.Errorf("kuronime base = %q, %v — mau base sumbernya sendiri tanpa garis miring akhir", base, ok)
	}
	if _, ok := posterMapBase("kuronime", Source{}); ok {
		t.Errorf("kuronime tanpa base URL harus tidak tersedia")
	}
	if _, ok := posterMapBase("hianime", anichin); ok {
		t.Errorf("resolver lain harus tidak tersedia")
	}
}

func TestParseHiCards(t *testing.T) {
	page := `<div class="flw-item"><div class="film-poster"><a href="/watch/one-piece-100"><img data-src="https://cdn.example/op.jpg" class="film-poster-img"></a></div><div class="film-detail"><h3 class="film-name"><a href="/watch/one-piece-100">One Piece</a></h3></div></div>
	<div class="flw-item"><div class="film-poster"><a href="/watch/jujutsu-kaisen-200"><img data-src="https://cdn.example/jjk.jpg" class="film-poster-img"></a></div><div class="film-detail"><h3 class="film-name"><a href="/watch/jujutsu-kaisen-200">Jujutsu Kaisen</a></h3></div></div>`
	out := parseHiCards(page, "https://hianime.to")
	if len(out) != 2 {
		t.Fatalf("kartu terbaca = %d, mau 2", len(out))
	}
	if out[0].Slug != "one-piece-100" || out[0].Poster != "https://cdn.example/op.jpg" || out[0].URL != "https://hianime.to/watch/one-piece-100" {
		t.Errorf("kartu pertama salah: %+v", out[0])
	}
}

func TestParseArchivePage(t *testing.T) {
	// Fixture mengikuti struktur nyata arsip anichin.moe /anime/
	// (terverifikasi 2026-11-22): kartu <article class="bs"> menaut
	// langsung ke seri level akar, judul di div.tt, status di
	// div.status / span.epx, poster img.ts-post-image relatif.
	page := `<div class="listupd normal">
	<article class="bs" itemscope="itemscope" itemtype="http://schema.org/CreativeWork"><div class="bsx"><a href="/100-000-years-of-refining-qi/" itemprop="url" title="100.000 Years of Refining Qi" class="tip" rel="21864"><div class="limit"><div class="typez Donghua">Donghua</div><div class="ply"><i class="far fa-play-circle"></i></div><div class="bt"><span class="epx">Ongoing</span><span class="sb Sub">Sub</span></div><img src="/wp-content/uploads/2023/02/100.000-Years-of-Refining-Qi-1-768x1077.jpg" class="ts-post-image wp-post-image" loading="lazy" itemprop="image" title="100.000 Years of Refining Qi"/></div><div class="tt">100.000 Years of Refining Qi<h2 itemprop="headline">100.000 Years of Refining Qi</h2></div></a></div></article>
	<article class="bs" itemscope="itemscope" itemtype="http://schema.org/CreativeWork"><div class="bsx"><a href="/36-cavalry/" itemprop="url" title="36 Cavalry" class="tip" rel="28576"><div class="limit"><div class="status Completed">Completed</div><div class="typez Donghua">Donghua</div><div class="bt"><span class="epx">Completed</span><span class="sb Sub">Sub</span></div><img src="/wp-content/uploads/2023/10/36-Cavalry.webp" class="ts-post-image wp-post-image" loading="lazy" itemprop="image" title="36 Cavalry"/></div><div class="tt">36 Cavalry<h2 itemprop="headline">36 Cavalry</h2></div></a></div></article>
	<article class="bs" itemscope="itemscope" itemtype="http://schema.org/CreativeWork"><div class="bsx"><a href="/btth-movie-battle-of-two-emperors-subtitle-indonesia/" itemprop="url" title="BTTH Movie: Battle of Two Emperors Subtitle Indonesia" class="tip" rel="59559"><div class="limit"><div class="status Completed">Completed</div><div class="typez Donghua">Donghua</div><div class="bt"><span class="epx">Ep Final Movie</span><span class="sb Sub">Sub</span></div><img src="https://anichin.moe/wp-content/uploads/2026/11/BTTH-Movie.webp" class="ts-post-image wp-post-image" loading="lazy" itemprop="image" title="BTTH Movie: Battle of Two Emperors"/></div><div class="tt">BTTH Movie: Battle of Two Emperors<h2 itemprop="headline">BTTH Movie: Battle of Two Emperors Subtitle Indonesia</h2></div></a></div></article>
	</div>`
	out := parseArchivePage(page, "https://anichin.moe")
	if len(out) != 3 {
		t.Fatalf("kartu terbaca = %d, mau 3: %+v", len(out), out)
	}
	first := out[0]
	if first.Slug != "100-000-years-of-refining-qi" || first.Title != "100.000 Years of Refining Qi" {
		t.Errorf("kartu pertama salah: %+v", first)
	}
	if first.URL != "https://anichin.moe/100-000-years-of-refining-qi/" {
		t.Errorf("URL kartu pertama = %q", first.URL)
	}
	if first.Poster != "https://anichin.moe/wp-content/uploads/2023/02/100.000-Years-of-Refining-Qi-1-768x1077.jpg" {
		t.Errorf("poster relatif harus dibuat absolut: %q", first.Poster)
	}
	if first.Completed {
		t.Errorf("kartu Ongoing tidak boleh bertanda tamat")
	}
	second := out[1]
	if second.Slug != "36-cavalry" || !second.Completed {
		t.Errorf("kartu Completed salah: %+v", second)
	}
	third := out[2]
	if third.Slug != "btth-movie-battle-of-two-emperors-subtitle-indonesia" ||
		third.Title != "BTTH Movie: Battle of Two Emperors" || !third.Completed {
		t.Errorf("kartu film (status div saja, epx 'Ep Final Movie') salah: %+v", third)
	}
	if third.Poster != "https://anichin.moe/wp-content/uploads/2026/11/BTTH-Movie.webp" {
		t.Errorf("poster absolut harus dipertahankan: %q", third.Poster)
	}
	if got := parseArchivePage(`<html><body><div class="listupd"></div></body></html>`, "https://anichin.moe"); len(got) != 0 {
		t.Errorf("halaman tanpa kartu harus kosong, dapat %d", len(got))
	}
}
