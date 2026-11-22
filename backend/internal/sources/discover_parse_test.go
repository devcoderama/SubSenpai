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
