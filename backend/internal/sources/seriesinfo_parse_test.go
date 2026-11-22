package sources

import (
	"strings"
	"testing"
)

// Uji regresi pengurai metadata seri dengan contoh yang diperkecil
// dari halaman ASLI anichin.moe (diambil 2026-11-22): halaman seri
// "Soul Land Movie: Sword Dao Chen Xin" dan beranda situs (kasus
// sinopsis boilerplate iklan unduhan yang sempat tersimpan ke
// katalog karena kandidat mirror salah path — /series/<slug>/ di
// .moe membalas redirect ke beranda).

const soulLandSeriesPage = `<html><body><span class="alter">斗罗大陆剧场版 剑道尘心</span><div class="spe"><span><b>Status:</b> Completed</span><span><b>Network:</b> <a href="/network/tencent-penguin-pictures/" rel="tag">Tencent Penguin Pictures</a></span><span><b>Studio:</b> <a href="/studio/sparkly-key-animation-studio/" rel="tag">Sparkly Key Animation Studio</a></span><span class="split"><b>Tanggal rilis:</b> Jul 12, 2025</span><span><b>Durasi:</b> 1h 30min</span><span><b>Season:</b> <a href="/season/spring-2025/" rel="tag">Spring 2025</a></span><span><b>Negara:</b> <a href="/country/china/" rel="tag">China</a></span><span><b>Tipe:</b> Movie</span><span><b>Episode:</b> 1</span><span class="split"><b>Subber:</b> Ray19</span><span class="author vcard"><b>Diposting oleh:</b> <i class="fn">Ray19</i></span><span class="split"><b>Ditambahkan:</b> <time itemprop="datePublished" datetime="2025-07-11T21:36:28+07:00" class="updated">July 11, 2025</time></span><span class="split"><b>Diperbarui pada:</b> <time itemprop="dateModified" datetime="2025-07-14T18:15:14+07:00">July 14, 2025</time></span></div><div class="genxed"><a href="/genres/action/" rel="tag">Action</a> <a href="/genres/adventure/" rel="tag">Adventure</a> <a href="/genres/cultivation/" rel="tag">Cultivation</a> <a href="/genres/fantasy/" rel="tag">Fantasy</a></div><div class="entry-content" itemprop="description"><h4 data-start="1414" data-end="2079">[Soul Land Movie: Sword Dao Chen Xin]<br>Dua puluh tahun sebelum Pertempuran Akhir di Benua Douluo,<br>Aula Wuhun diam-diam merancang konspirasi untuk menyatukan seluruh dunia roh dan mencaplok semua sekte besar. Sekte Seven Treasure Glazed Tile menjadi target pertama dan menghadapi ancaman kehancuran.<br>Bone Douluo membawa calon pewaris sekte — Ning Rongrong yang masih berusia lima tahun — untuk meminta Sword Douluo turun gunung demi menyelamatkan mereka.<br>Maka, Sword Douluo Chen Xin pun memulai perjalanan pelarian hidup dan mati bersama Rongrong kecil.<br>Namun, yang tidak mereka sadari, perjalanan ini hanyalah permulaan dari sebuah konspirasi besar yang akan mengguncang seluruh Benua Douluo&#8230;</h4></div></body></html>`

const anichinHomepage = `<html><body><div class="entry-content" itemprop="text"><p>Adtival Network (Main Shortlink) LANGKAH SINGKAT : Pilih Server Filehosting (ex. Mirrored) ➜ Isi Captcha (kalo ada) &amp; Click here to Continue ➜ Tunggu 10 detik ➜ Selesai.  *Note  :&#8230;</p></div></body></html>`

func TestParseSeriesPageSoulLandMovie(t *testing.T) {
	meta := parseSeriesPage(soulLandSeriesPage)
	if meta.Network != "Tencent Penguin Pictures" {
		t.Errorf("network = %q", meta.Network)
	}
	if meta.Studio != "Sparkly Key Animation Studio" {
		t.Errorf("studio = %q", meta.Studio)
	}
	if meta.Country != "China" {
		t.Errorf("country = %q", meta.Country)
	}
	if meta.Year != 2025 {
		t.Errorf("year = %d", meta.Year)
	}
	if meta.Season != "spring" {
		t.Errorf("season = %q", meta.Season)
	}
	if meta.DurationMin != 90 {
		t.Errorf("duration_min = %d, mau 90 (\"1h 30min\")", meta.DurationMin)
	}
	wantGenres := []string{"Action", "Adventure", "Cultivation", "Fantasy"}
	if len(meta.Genres) != len(wantGenres) {
		t.Fatalf("genres = %v", meta.Genres)
	}
	for i, g := range wantGenres {
		if meta.Genres[i] != g {
			t.Errorf("genres[%d] = %q, mau %q", i, meta.Genres[i], g)
		}
	}
	if !strings.HasPrefix(meta.Synopsis, "Dua puluh tahun sebelum Pertempuran Akhir") {
		t.Errorf("sinopsis tidak diawali teks asli: %.100q", meta.Synopsis)
	}
	if IsBoilerplateSynopsis(meta.Synopsis) {
		t.Errorf("sinopsis asli tertuduh boilerplate")
	}
	if s := seriesMetaScore(meta); s < 8 {
		t.Errorf("skor kelengkapan = %d, mau >= 8", s)
	}
}

func TestParseSeriesPageHomepageBoilerplate(t *testing.T) {
	meta := parseSeriesPage(anichinHomepage)
	if meta.Synopsis != "" {
		t.Errorf("sinopsis beranda harus ditolak sebagai boilerplate, dapat %.100q", meta.Synopsis)
	}
	if s := seriesMetaScore(meta); s != 0 {
		t.Errorf("skor beranda = %d, mau 0", s)
	}
}

func TestMetaComplete(t *testing.T) {
	lengkap := func() *SeriesMeta {
		return &SeriesMeta{
			Synopsis: "Dua puluh tahun sebelum Pertempuran Akhir di Benua Douluo, Aula Wuhun diam-diam merancang konspirasi besar.",
			Studio:   "Sparkly Key Animation Studio",
			Network:  "Tencent Penguin Pictures",
			Genres:   []string{"Action", "Fantasy"},
		}
	}
	if !metaComplete(lengkap()) {
		t.Error("meta dengan sinopsis sah + studio + network + genre harus lengkap")
	}
	if !metaComplete(parseSeriesPage(soulLandSeriesPage)) {
		t.Error("hasil urai halaman Soul Land Movie harus lengkap")
	}
	kurang := map[string]func(*SeriesMeta){
		"sinopsis kosong": func(m *SeriesMeta) { m.Synopsis = "" },
		"studio kosong":   func(m *SeriesMeta) { m.Studio = "" },
		"network kosong":  func(m *SeriesMeta) { m.Network = "" },
		"genre kosong":    func(m *SeriesMeta) { m.Genres = nil },
		"sinopsis boilerplate": func(m *SeriesMeta) {
			m.Synopsis = "Adtival Network (Main Shortlink) LANGKAH SINGKAT : Pilih Server Filehosting (ex. Mirrored) & Click here to Continue"
		},
	}
	for nama, ubah := range kurang {
		m := lengkap()
		ubah(m)
		if metaComplete(m) {
			t.Errorf("kasus %q tidak boleh lengkap", nama)
		}
	}
	if metaComplete(nil) {
		t.Error("meta nil tidak boleh lengkap")
	}
	if metaComplete(parseSeriesPage(anichinHomepage)) {
		t.Error("hasil urai beranda tidak boleh lengkap")
	}
}

func TestIsBoilerplateSynopsis(t *testing.T) {
	if !IsBoilerplateSynopsis("Adtival Network (Main Shortlink) LANGKAH SINGKAT : Pilih Server Filehosting (ex. Mirrored) - Isi Captcha (kalo ada) & Click here to Continue") {
		t.Error("teks Adtival harus terdeteksi boilerplate")
	}
	if IsBoilerplateSynopsis("Dua puluh tahun sebelum Pertempuran Akhir di Benua Douluo, Aula Wuhun diam-diam merancang konspirasi besar.") {
		t.Error("sinopsis asli tidak boleh terdeteksi boilerplate")
	}
	if IsBoilerplateSynopsis("") {
		t.Error("string kosong bukan boilerplate")
	}
}

func TestSeriesPathForHost(t *testing.T) {
	cases := []struct{ path, host, want string }{
		{"/series/soul-land-movie-sword-dao-chen-xin/", "anichin.moe", "/soul-land-movie-sword-dao-chen-xin/"},
		{"/soul-land-movie-sword-dao-chen-xin/", "anichin.my", "/series/soul-land-movie-sword-dao-chen-xin/"},
		{"/series/soul-land-movie-sword-dao-chen-xin/", "anichin.my", "/series/soul-land-movie-sword-dao-chen-xin/"},
		{"/soul-land-movie-sword-dao-chen-xin/", "anichin.moe", "/soul-land-movie-sword-dao-chen-xin/"},
		{"/genre/action/", "anichin.my", "/genre/action/"},
		{"/genre/action/", "anichin.moe", "/genre/action/"},
	}
	for _, c := range cases {
		if got := seriesPathForHost(c.path, c.host); got != c.want {
			t.Errorf("seriesPathForHost(%q, %q) = %q, mau %q", c.path, c.host, got, c.want)
		}
	}
}

func TestMirrorCandidatesAnichin(t *testing.T) {
	// anichin.my adalah kloningan berbahaya (arahan Rams 2026-11-22):
	// kandidat TIDAK BOLEH pernah mengarah ke .my. URL .my tersimpan
	// lama hanya dipakai sebagai bahan membentuk URL .moe-nya.
	fromMy := mirrorCandidates("https://anichin.my/series/soul-land-movie-sword-dao-chen-xin/")
	if len(fromMy) != 1 || fromMy[0] != "https://anichin.moe/soul-land-movie-sword-dao-chen-xin/" {
		t.Errorf("kandidat dari .my = %v, mau hanya URL .moe", fromMy)
	}
	fromMoe := mirrorCandidates("https://anichin.moe/soul-land-movie-sword-dao-chen-xin/")
	if len(fromMoe) != 1 || fromMoe[0] != "https://anichin.moe/soul-land-movie-sword-dao-chen-xin/" {
		t.Errorf("kandidat dari .moe = %v, mau URL itu sendiri saja", fromMoe)
	}
	first := moeFirstCandidates("https://anichin.my/series/soul-land-movie-sword-dao-chen-xin/")
	if len(first) != 1 || first[0] != "https://anichin.moe/soul-land-movie-sword-dao-chen-xin/" {
		t.Errorf("moeFirst = %v", first)
	}
	other := mirrorCandidates("https://kuronime.sbs/series/contoh-anime/")
	if len(other) != 1 {
		t.Errorf("host selain Anichin tidak boleh dapat kandidat silang: %v", other)
	}
}

func TestMoeBase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://anichin.my", "https://anichin.moe"},
		{"https://anichin.my/", "https://anichin.moe"},
		{"https://anichin.moe", "https://anichin.moe"},
		{"https://anichin.moe/", "https://anichin.moe"},
		// Host lain tidak disentuh.
		{"https://kuronime.sbs/", "https://kuronime.sbs"},
		{"", ""},
	}
	for _, c := range cases {
		if got := moeBase(c.in); got != c.want {
			t.Errorf("moeBase(%q) = %q, mau %q", c.in, got, c.want)
		}
	}
}

func TestParseDurationMin(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"1h 30min", 90},
		{"2h", 120},
		{"20 min", 20},
		{"20min", 20},
		{"1h30min", 90},
		{"15 Menit", 15},
		{"1 Jam 30 Menit", 90},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseDurationMin(c.in); got != c.want {
			t.Errorf("parseDurationMin(%q) = %d, mau %d", c.in, got, c.want)
		}
	}
}
