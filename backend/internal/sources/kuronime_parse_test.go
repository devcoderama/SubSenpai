package sources

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Uji pengurai Kuronime dengan contoh markup asli kuronime.sbs
// (terverifikasi 2026-11-22; tanpa jaringan).

func TestParseKuroArchivePage(t *testing.T) {
	// Kartu asli dari beranda/arsip Kuronime: img poster TAK
	// berpetik dengan ekor Photon "?resize=" — URL bersihnya yang
	// harus tersimpan. Judul memakai entitas &#8217;.
	page := `<div class="listupd"><div class="excstf">
	<article class="bs" itemscope="itemscope" itemtype="http://schema.org/CreativeWork">
		<div class="bsx">
			<a href="https://kuronime.sbs/anime/demons-crest/" itemprop="url" title="Demons&#8217; Crest">
				<div class="limit">
					<div class="bt"><span class="type ONA">ONA</span></div>
					<img loading=”eager” fetchpriority="high" itemprop="image" alt="" width="145" height="207" src=https://i0.wp.com/kuronime.sbs/wp-content/uploads/2026/11/158889l.jpg?resize= 145,207>
				</div>
				<div class="tt">
					<h2 itemprop="headline">Demons &#8217; Crest</h2>
					<div class="rating"><div class="rtg"></div><i>7.00</i></div>
				</div>
			</a>
		</div>
	</article>
	<article class="bs" itemscope="itemscope" itemtype="http://schema.org/CreativeWork">
		<div class="bsx">
			<a href="https://kuronime.sbs/anime/one-piece-op/" itemprop="url" title="One Piece">
				<div class="limit">
					<img itemprop="image" src="https://i1.wp.com/kuronime.sbs/wp-content/uploads/2015/02/op11232.webp" alt="One Piece">
				</div>
				<div class="tt"><h2 itemprop="headline">One Piece</h2></div>
			</a>
		</div>
	</article>
	<article class="bsu" itemscope="itemscope" itemtype="http://schema.org/CreativeWork">
		<div class="bsux"><a href="https://kuronime.sbs/nonton-dreamland-episode-6/" itemprop="url" title="Dreamland Episode 6 Subtitle Indonesia"></a></div>
	</article>
	</div></div>`
	out := parseKuroArchivePage(page, "https://kuronime.sbs")
	if len(out) != 2 {
		t.Fatalf("kartu seri terbaca = %d, mau 2 (kartu episode bsu harus dilewati): %+v", len(out), out)
	}
	dc := out[0]
	if dc.Slug != "demons-crest" || dc.Title != "Demons ’ Crest" && dc.Title != "Demons' Crest" {
		t.Errorf("kartu pertama salah: %+v", dc)
	}
	if dc.URL != "https://kuronime.sbs/anime/demons-crest/" {
		t.Errorf("URL kartu pertama = %q", dc.URL)
	}
	if dc.Poster != "https://i0.wp.com/kuronime.sbs/wp-content/uploads/2026/11/158889l.jpg" {
		t.Errorf("poster harus dibersihkan dari ekor ?resize=: %q", dc.Poster)
	}
	op := out[1]
	if op.Slug != "one-piece-op" || op.Poster != "https://i1.wp.com/kuronime.sbs/wp-content/uploads/2015/02/op11232.webp" {
		t.Errorf("kartu kedua salah: %+v", op)
	}
	if got := parseKuroArchivePage(`<html><body></body></html>`, "https://kuronime.sbs"); len(got) != 0 {
		t.Errorf("halaman tanpa kartu harus kosong, dapat %d", len(got))
	}
}

// Halaman seri One Piece versi diperkecil dengan nilai-nilai yang
// terverifikasi dari halaman aslinya (label info, sinopsis, dan
// href episode persis).
const kuroSeriesFixture = `<html><body>
<div id="sidebar"><a href="https://kuronime.sbs/nonton-black-clover-season-2-episode-8/">Black Clover Season 2 Episode 8</a></div>
<article id="post-1925" class="post-1925 hentry">
	<h1 class="entry-title">One Piece</h1>
	<div class="info">
		<span>Judul: ONE PIECE</span>
		<span>Genre: <a href="https://kuronime.sbs/genres/action/">Action</a>, <a href="https://kuronime.sbs/genres/adventure/">Adventure</a>, <a href="https://kuronime.sbs/genres/shounen/">Shounen</a></span>
		<span>Status: Ongoing</span>
		<span>Studio: Toei Animation</span>
		<span>Tayang: Oct 20, 1999 to ?</span>
		<span>Season: Fall 1999</span>
		<span>Tipe: TV</span>
		<span>Durasi: 24 min</span>
	</div>
	<h2>Sinopsis One Piece Sub Indo</h2>
	<h3>Gol D Roger dikenal sebagai Raja Bajak Laut, Orang terkuat dan paling terkenal yang pernah mengarungi Grand Line. Penangkapan dan Eksekusi Roger oleh Pemerintahan Dunia telah membawa perubahan di seluruh dunia. Subtitle Indonesia</h3>
	<h4>Episode One Piece</h4>
	<ul>
		<li><a href="https://kuronime.sbs/nonton-one-piece-episode-1180/">Episode 1180</a> <a href="https://kuronime.sbs/nonton-one-piece-episode-1180/">Nonton</a></li>
		<li><a href="https://kuronime.sbs/nonton-one-piece-episode-1179/">Episode 1179</a> <a href="https://kuronime.sbs/nonton-one-piece-episode-1179/">Nonton</a></li>
		<li><a href="https://kuronime.sbs/nonton-one-piece-episode-1/">Episode 1</a> <a href="https://kuronime.sbs/nonton-one-piece-episode-1/">Nonton</a></li>
	</ul>
</article>
</body></html>`

func TestParseKuroSeriesPage(t *testing.T) {
	meta := parseKuroSeriesPage(kuroSeriesFixture)
	if meta.TitleAlt != "ONE PIECE" {
		t.Errorf("judul alt = %q, mau ONE PIECE", meta.TitleAlt)
	}
	if len(meta.Genres) != 3 || meta.Genres[0] != "Action" || meta.Genres[2] != "Shounen" {
		t.Errorf("genre salah: %v", meta.Genres)
	}
	if meta.Studio != "Toei Animation" {
		t.Errorf("studio = %q", meta.Studio)
	}
	if meta.Season != "fall" || meta.Year != 1999 {
		t.Errorf("season/tahun = %q/%d, mau fall/1999", meta.Season, meta.Year)
	}
	if meta.DurationMin != 24 {
		t.Errorf("durasi = %d, mau 24", meta.DurationMin)
	}
	if meta.Synopsis == "" || len(meta.Synopsis) < 100 {
		t.Errorf("sinopsis terlalu pendek/kosong: %q", meta.Synopsis)
	}
	if got := meta.Synopsis[len(meta.Synopsis)-10:]; got == " Indonesia" {
		t.Errorf("sufiks ' Subtitle Indonesia' harus dibuang: …%q", got)
	}
}

func TestParseKuroEpisodes(t *testing.T) {
	eps := parseKuroEpisodes(kuroSeriesFixture, "https://kuronime.sbs/anime/one-piece-op/")
	if len(eps) != 3 {
		t.Fatalf("episode terbaca = %d, mau 3 (duplikat tautan Nonton + widget sidebar harus tersaring): %+v", len(eps), eps)
	}
	if eps[0].Number != 1 || eps[1].Number != 1179 || eps[2].Number != 1180 {
		t.Errorf("urutan/nomor episode salah: %+v", eps)
	}
	if eps[2].URL != "https://kuronime.sbs/nonton-one-piece-episode-1180/" {
		t.Errorf("URL episode 1180 = %q", eps[2].URL)
	}
}

// Halaman nyata Fate/Apocrypha (terverifikasi via browser
// 2026-11-22): slug seri "fateapocrypha" TANPA tanda hubung,
// slug episode "fate-apocrypha" DENGAN tanda hubung; daftar
// episode berada di luar <article>; sinopsis diakhiri sisipan
// kata kunci "Subtitle Indonesia" berulang.
const kuroFateFixture = `<html><body>
<article id="post-151" class="post-151 hentry">
<h1 class="entry-title">Fate/Apocrypha</h1>
<div class="info-content">
<div class="spe"><span>Judul: Fate/Apocrypha</span></div>
<div class="spe"><span>Genre: Action, Drama, Fantasy</span></div>
<div class="spe"><span>Studio: A-1 Pictures</span></div>
<div class="spe"><span>Durasi: 23 min. per ep.</span></div>
</div>
<h2>Sinopsis Fate/Apocrypha Sub Indo</h2>
<div class="desc"><div class="entry-content">
<h3>Pada dahulu kala, di kota Fuyuki pernah diadakan sebuah ritual kuno yang bernama Perang Cawan Suci yang melibatkan tujuh master dan tujuh roh pahlawan. Akan tetapi, ketika perang kedua berlangsung, situasi berubah menjadi pertempuran besar yang melibatkan banyak pihak dari berbagai faksi. Fate Apocrypha Subtitle Indonesia, Fate/Apocrypha Subtitle Indonesia</h3>
</div></div>
</article>
<div class="bixbox"><h4>Episode Fate/Apocrypha</h4><ul>
<li><a href="https://kuronime.sbs/nonton-fate-apocrypha-episode-25/">Episode 25 END</a> <a href="https://kuronime.sbs/nonton-fate-apocrypha-episode-25/">Nonton</a></li>
<li><a href='https://kuronime.sbs/nonton-fate-apocrypha-episode-1/'>Episode 1</a></li>
</ul></div>
<div class="widget"><ul>
<li><a href="https://kuronime.sbs/nonton-dreamland-episode-6/">Dreamland Episode 6</a></li>
<li><a href="https://kuronime.sbs/nonton-one-piece-episode-1180/">One Piece Episode 1180</a></li>
</ul></div>
</body></html>`

func TestParseKuroSeriesPageFate(t *testing.T) {
	meta := parseKuroSeriesPage(kuroFateFixture)
	if meta.Studio != "A-1 Pictures" {
		t.Errorf("studio = %q", meta.Studio)
	}
	if !strings.Contains(meta.Synopsis, "Perang Cawan Suci") {
		t.Errorf("sinopsis tidak terbaca: %q", meta.Synopsis)
	}
	if strings.Contains(meta.Synopsis, "Subtitle Indonesia") {
		t.Errorf("ekor kata kunci harus dibuang: …%q", meta.Synopsis)
	}
}

func TestParseKuroEpisodesFate(t *testing.T) {
	eps := parseKuroEpisodes(kuroFateFixture, "https://kuronime.sbs/anime/fateapocrypha/")
	if len(eps) != 2 {
		t.Fatalf("episode terbaca = %d, mau 2 (slug beda tanda hubung + di luar article + widget seri lain tersaring): %+v", len(eps), eps)
	}
	if eps[0].Number != 1 || eps[1].Number != 25 {
		t.Errorf("nomor episode salah: %+v", eps)
	}
	if eps[1].URL != "https://kuronime.sbs/nonton-fate-apocrypha-episode-25/" {
		t.Errorf("URL episode 25 = %q", eps[1].URL)
	}
}

func TestParseKuroEmbed(t *testing.T) {
	// Tag iframe asli halaman episode Kuronime (token dipersingkat):
	// src kosong, URL embed di data-src; iframe cbox adalah chat.
	page := `<article id="post-202543" class="post-202543 hentry">
		<button class="gp">playads</button>
		<iframe id="iframedc" class=" lazyloaded" src="" data-src="https://player.animeku.org/?data=eyJjdCI6InRva2VuIn0" width="100%" height="100%" frameborder="0" allowfullscreen="true" loading="lazy"></iframe>
		<iframe src="https://www5.cbox.ws/box/?boxid=941701&boxtag=z8VCJl"></iframe>
	</article>`
	if got := parseKuroEmbed(page); got != "https://player.animeku.org/?data=eyJjdCI6InRva2VuIn0" {
		t.Errorf("embed = %q, mau URL player.animeku.org dari data-src", got)
	}
	if got := parseKuroEmbed(`<article><p>kosong</p></article>`); got != "" {
		t.Errorf("halaman tanpa iframedc harus kosong, dapat %q", got)
	}
}

func TestParseKuroEmbedAboutBlank(t *testing.T) {
	// Tag nyata produksi: src hanya placeholder "about:blank",
	// URL asli di data-lazy-src (tema ini memakai lazy-load).
	page := `<article class="hentry">
		<button class="gp">playads</button>
		<iframe id="iframedc" class="lazyload" src="about:blank" data-lazy-src="https://player.animeku.org/?data=eyJhbGciOiJIUzI1NiJ9" frameborder="0" allowfullscreen></iframe>
		<iframe src="https://www5.cbox.ws/box/?boxid=941701&boxtag=z8VCJl"></iframe>
	</article>`
	if got := parseKuroEmbed(page); got != "https://player.animeku.org/?data=eyJhbGciOiJIUzI1NiJ9" {
		t.Errorf("embed = %q, mau URL animeku dari data-lazy-src (bukan about:blank)", got)
	}
	// URL hanya tertulis di string JS halaman.
	page2 := `<article class="hentry"><iframe id="iframedc" src="about:blank"></iframe>
	<script>var ply = "https://player.animeku.org/?data=dG9rZW4tdGVzdA";</script></article>`
	if got := parseKuroEmbed(page2); got != "https://player.animeku.org/?data=dG9rZW4tdGVzdA" {
		t.Errorf("embed dari JS = %q, mau URL animeku", got)
	}
	// Hanya placeholder, tanpa URL di mana pun → kosong.
	page3 := `<article class="hentry"><iframe id="iframedc" src="about:blank"></iframe></article>`
	if got := parseKuroEmbed(page3); got != "" {
		t.Errorf("about:blank tidak boleh jadi embed, dapat %q", got)
	}
}

func TestParseKuroPosterMap(t *testing.T) {
	page := `<article class="bs"><div class="bsx"><a href="https://kuronime.sbs/anime/demons-crest/" title="Demons Crest"><div class="limit"><img itemprop="image" src=https://i0.wp.com/kuronime.sbs/wp-content/uploads/2026/11/158889l.jpg?resize= 145,207></div><div class="tt"><h2 itemprop="headline">Demons Crest</h2></div></a></div></article>
	<article class="bsu"><div class="bsux"><a href="https://kuronime.sbs/nonton-dreamland-episode-6/"><div class="bsuxtt"><h2>Dreamland</h2></div><div class="limit"><img itemprop="image" src="https://i2.wp.com/kuronime.sbs/wp-content/uploads/2026/10/112232I.webp"></div></a></div></article>`
	m := parseKuroPosterMap(page, "https://kuronime.sbs")
	if got := m["demons crest"]; got != "https://i0.wp.com/kuronime.sbs/wp-content/uploads/2026/11/158889l.jpg" {
		t.Errorf("poster demons crest = %q", got)
	}
	if got := m["dreamland"]; got != "https://i2.wp.com/kuronime.sbs/wp-content/uploads/2026/10/112232I.webp" {
		t.Errorf("poster dreamland (kartu episode) = %q", got)
	}
}

// TestKuronimeLive adalah uji asap terhadap situs asli — hanya
// jalan bila SUBSENPAI_LIVE_TEST=1. CATATAN: dari jaringan yang
// kena Cloudflare (mis. VM agen) uji ini gagal di fetch pertama;
// jalankan dari VPS.
func TestKuronimeLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	r, ok := ByKey("kuronime")
	if !ok {
		t.Fatal("resolver kuronime tidak terdaftar")
	}
	src := Source{Name: "Kuronime", ContentType: "anime", Lang: "id", BaseURL: "https://kuronime.sbs", Resolver: "kuronime"}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	disc := r.(Discoverer)
	found, err := disc.Discover(ctx, src, 60)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	t.Logf("ditemukan: %d", len(found))
	withPoster := 0
	for _, d := range found {
		if d.Poster != "" {
			withPoster++
		}
	}
	t.Logf("berposter: %d/%d", withPoster, len(found))
	if len(found) < 30 || withPoster < 30 {
		t.Fatalf("penemuan arsip kurang: %d judul, %d berposter", len(found), withPoster)
	}
	one := found[0]
	meta, err := r.(SeriesInfoer).SeriesInfo(ctx, src, one.URL)
	if err != nil {
		t.Fatalf("SeriesInfo %s: %v", one.URL, err)
	}
	t.Logf("meta %s: studio=%q genres=%v tahun=%d sinopsis %d aksara", one.Title, meta.Studio, meta.Genres, meta.Year, len(meta.Synopsis))
	eps, err := r.Episodes(ctx, src, one.URL)
	if err != nil {
		t.Fatalf("Episodes %s: %v", one.URL, err)
	}
	t.Logf("episode %s: %d", one.Title, len(eps))
	if len(eps) == 0 {
		t.Fatalf("episode kosong untuk %s", one.Title)
	}
	res, err := r.Resolve(ctx, src, eps[len(eps)-1])
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	t.Logf("aliran: %+v", res.Streams)
	if len(res.Streams) == 0 {
		t.Fatal("aliran kosong")
	}
}

// --- API sumber Animeku (alur pintar.js) ------------------------

// Blob + xenHash asli dari halaman episode Fate/Apocrypha ep 1
// (dijaring dari HTML mentah produksi 2026-11-22).
const kuroFateBlob = "dXl1RHBYeXlpcW1GWEMzb29Fb3ZCSWZsUUcyTEdPb3lISkNiV0hLTjErc2tGb1MyNkxJMnpnSlhXWmt4YndKUW9wTnE5U1M1aWd1eEgrSEovUnJraVB0bC9UZkZxWU9mZWdJZG4zbkh4aTZTUi9NMlZpclFCNWllUDRGQWVqNDJWQXZVQXV1ZDJ2cm10OW5aQnArNjgvUXgzMmgvbHlOcUo1QXNGN3orMm5ZbHpydnBuM3gzdDYrUW1XUCtPZHh3anpYWnlQcTQ5alBQd25JZm1UOXVlNDdyL05DRk9yRTJmME1hQmlzbnlZcz0="

const kuroFatePageSnippet = `<script>var xenHash = "awar";  var is_singular = true; var _0xa100d42aa = "` + kuroFateBlob + `";</script>`

func TestParseKuroBlob(t *testing.T) {
	blob, xen := parseKuroBlob(kuroFatePageSnippet)
	if blob != kuroFateBlob {
		t.Fatalf("blob salah: %q", blob)
	}
	if xen != "awar" {
		t.Fatalf("xenHash salah: %q", xen)
	}
	if b, _ := parseKuroBlob("<html>tanpa blob</html>"); b != "" {
		t.Fatalf("halaman tanpa blob harus kosong, dapat %q", b)
	}
}

// Payload mirror ASLI dari API Animeku untuk Fate ep 1
// (respons hidup 2026-11-22) — dekripsi harus menghasilkan
// peta embed yang sama seperti yang dilihat browser.
const kuroFateMirror = "eyJjdCI6IkMwUHlmaFMzeU5kRGJIWVZQVEJ6RnIxSDVSSDc2TXh4RWtCQ2JHRExDWDQwVGxrbTV3L1A1VmFYZDBpcTB0bEVrNmFQQWJORFFzckw0UkdqSEZRY1daWm43eW9tYzZpQmIvUjJlbWlseStPT0pBMzZoNXk0WUozZUxRNWNwaCtTM0RicnRJZVpvYVBBWjE3OW9MVnh6cGR2ZkszZEttLzNpRDlOK2FxMTZMcVQ4bEFJb1BmMmlLdmlsSVN2WkNpQnVUUGpXSW5JOER2UitzSTB3Wm9BRzM3Sit2eHdPYU1rZWZtY0l0eHRxOCt3UU4rL20zR1BRaGdkM0ZwalFmTWJHeXdJRWUzZ1VqRGxEYUJtTmR1ektScy9Ga2VGdVRaQi9WeXpNTHBsUm5qd2pkRXk2cXVnNDkweTlwM2xoTEhCYlM3T3IreXJOb0QrMkxNc0dIeE1YT01aWFQrcnd6VDROZUFYaHY1S1pJQjlPWHdCZ1A4MjJhZnRuaWptWGphYm41RlVYNmVRU285Z3hURW5ON09NNzZDYUVBTWtLQlpDejZqUG1hWmNSd2xJUlVnTEtTOElKUm96Y2JYME1sUS9oV1I2ZEhmVmJ1ZVJBemo0ZGJnbkhLR3ZYbSszaGRLclhYMk1lZHovN3B1RGtVWXpueVNGUWY2Y1NsTXE0ZmxCQk01eWtlUDJaVEJ3aG8zS0RBRVBrVWg4STVzcHlvNHRoMUtmSmxLZ04xeGtsM2lkR2hlSEN3T2pUOHNXeWUwUjJWTm5xZGZVSFlTdEFoaU9YYW5sYVZJdXNHREdDRStwbnRIeG1zQnVsOTBHSVMyTmZycVQ5dTNmZ1p0V2Y4OXE5U2NvblptWVZSemIwMWROd1RXN0pVTWM2WTBOOFQxcEQya290VjFJejZWcTlocWQzY09ac1JUNDFpVmtnUTBCYm44NzRiTnZucXZsNWV0TElRbkcrTHBDNTlkaFYxeDJ2dXByUHY5azRRaEJGNmNmYzZzeVdBMXRzaW9menJOT1J5R2hTdmpmTUw2N2oxdVpFcDhhTEIwT1hmaktZOGxiYVNZODQ4Z0RHdm5jL090c2p6aW9Cb1c0b21NTGZPT2VRbkxGVUZyeU9vSmFNdkR3QjRTTisyZW45WVFOZTRaZVhMT3UxazJ6QWxqc1g1MEtFSDZrWm9BUC85K1pXN1pLS3BBQi8zNFNIbUptaXhob1A2allMVi9CZjM5NDJiZUJPeDNpTlpSWitlNElNakdqUkdTNDlNUktkSG5ONHN2bW11aEhmVmF1Z2lKMmN0aUpCaC9EVVNsQ2dXbWJwZFcrN294WGx5dURxU1lCTkZIcDhVdDhFY2NSNTRONkRHaHZtMUQrUWhrWWd0ajhvdVF0c3RiazRwVUlpaGI1TlB6cGV1b01icnFCOER2RXFoeGNwamhvYXZYRFN6ZndOSFUydWZmMUErdlFzdm95VWZnZmd4WVpPeWxCZ0JVRjRLV2F5MUJtWkFCK2xybEJpSStDNXdzUEVVRjdNblF5YXpZdW5BWUpNZVdzRWgrQ0tucXVXci9FamVMWlc5NTIzZU9FcUJCMVVXSDdqUFZHRVA5R1hETm9XMEZzOGQ1blhYbFkvSTROV250Z3pORU03ZU9kellhaVBTMjBnZXpUcmtyVWtya1BjZ1RCSGNVaDZMeStPc0YyTXRKTCs2UUxJQmxyVVBxc0QzYjhINEhFRkRJY1U3Z2k1dkdzVmxtc1N0WHh1d2Y3RUpWdVkvdHdvOFJwbk9HcFYvWWtLTWIrZmwyZlV6MjZpUVZudEtXb1pETk9PNmdYU2J1THFWNk10MEhBaXhrUWp6cG5sY1Qray9IbnN6SEQrcDhsNnlsZnA2ZGxpQ2xPMklnd3JRbGV5Y3R5R245ekFQL2lKV0VOWU5JdEpGN0V2L2VpUU5mVWw1MjRTWE9TcTVPUzJZRjNDUHlvM0FQWlgyTWtma2ttUlBaVVNqSGxvcmhtcnRLWWFoNGtocS9mRXovK3hWK3lSL2VaWlpoUCtWM3hBQWZDaGhiRjczcFFPRXcvaHBidURzVkk1MWxlcGhmNTlNWVBFUlRBcldCWVN3aTZoRGJGNmw2SGNOR09CeHEzekdxSHp0MGthZ21hMFlId1BrQ0VKQUsyVmVXUXozTXBQNWg3b29QMHlpcFV2UlRTdkVONmYwWmIyaTNneGE0bC9LRVUvR3c2UWNMRFpVcVI1Q0lWRUFKcTJRU1lCUVRKdkVZYjZXeDhMcjAvU3RXYUY0N0Y2cE43VnJMbUgyVEdsaU9SMWxDdXRJOTZSMGc3SDZsTnZWZzFMaHJVamdSSGZKbjV6MWVkVzFybG85K0dMa2NNUm5laDEvNUxEcUY2OTRnMi9xSkVrY2dmUUlqMVQ5V0cxZzV6RE1XbzNNOHFIOXVublA3eHpSN0RHckJ1a1VNMk5BejREUDhKR1U1R0FCUU95ZGZndHJidVVjWkxuWThiUGx0VUtnT0RkQmNoNzhPZ1hoU0U4TU1sZ1JvTUYrbm92c05FZDcyQ042Vy9YMDVLZS9EcW1XU1RZckhuUVhpOERWM3gzZEkvdGhYY1RGbi9oNk9vNzVidWxmdG0zVXVjaU1UdDFIcXEwU0NyV0p3TitCUGhPOExodlV1SGpLZnpOVlByc0FwV1JuRFl6WFFjQmU1OVd2T05rWjJMRVdwSzlkcHRsa29FazRrbmt2OEFtSG43b2FUUlJVUG5VSnQwMGRLZTl5U1JKV2xodzJQWUhocHJMOXI2bVdhRzRYdUU4Zmg4KzB0aFVISDViTnN4d08wVUFleU04MVlkbFp5TzloSExtdHBOK1NWcXhRbURzT0Y2eTlaUEIzcVFwTUVXait4YXBXR1kzdVJVN3ZncjREMzZGN0VjWWJ4cWRYNElBemw1bE9zSW1JME1PbWJ6eERHZGZjQ2ZJY0pmM3JUMHgzVGNmMGVoaUFLa1gvSWk5LzZKM1htZDFrdXk4eXVQWnloekdXbXZkS1FxU3N1dUNBc1RKTHR3TTl2cWk5MjA4M1hzMFczZlFnZ3orZkp0WWZJRHJIWWd3M3pURlhEUk5ZWjRmdlZiN2c9PSIsIml2IjoiMmQ3NzlmOTMxYjM1NTdmNDYwNDNlZWI4MWFmYWUyMjAiLCJzIjoiODUwOTk0Zjg5ZjNkMGZjZiJ9"

func TestDecryptKuroMirrorReal(t *testing.T) {
	data, err := decryptKuroMirror(kuroFateMirror)
	if err != nil {
		t.Fatalf("dekripsi mirror gagal: %v", err)
	}
	if got := data.Embed["v720p"]["mp4upload"]; got == nil || *got != "https://www.mp4upload.com/embed-fs6922bqzi5g.html" {
		t.Fatalf("embed mp4upload 720p salah: %v", got)
	}
	if got := data.Embed["v360p"]["doodstream"]; got == nil || *got != "https://dooood.com/e/lpase1uayf99nj1yhhx91xmjpu5mchf3" {
		t.Fatalf("embed doodstream 360p salah: %v", got)
	}
}

func TestKuroStreamsFromAPI(t *testing.T) {
	resp := &kuroAPIResp{Status: 200, Src: "TOKENHD", SrcSD: "TOKENSD", Mirror: kuroFateMirror}
	streams := kuroStreamsFromAPI(resp, "awar", "id")
	if len(streams) < 5 {
		t.Fatalf("aliran terlalu sedikit: %d", len(streams))
	}
	if streams[0].URL != kuroPlayerBase+"TOKENHD" || streams[0].Label != "Animeku" {
		t.Fatalf("aliran pertama salah: %+v", streams[0])
	}
	if streams[1].URL != kuroPlayerBase+"TOKENSD" || streams[1].Label != "Animeku SD" {
		t.Fatalf("aliran kedua salah: %+v", streams[1])
	}
	found := false
	for _, s := range streams {
		if s.URL == "https://www.mp4upload.com/embed-fs6922bqzi5g.html" && s.Label == "Mp4Upload 720p" && s.Lang == "id" {
			found = true
		}
		if strings.Contains(s.URL, "about:blank") {
			t.Fatalf("aliran placeholder ikut: %+v", s)
		}
	}
	if !found {
		t.Fatalf("aliran Mp4Upload 720p tidak ada: %+v", streams)
	}
	// Blog hanya untuk situs xenHash "rawa".
	rawa := kuroStreamsFromAPI(&kuroAPIResp{Status: 200, Blog: "BLOGID"}, "rawa", "id")
	if len(rawa) != 1 || rawa[0].URL != kuroBlogBase+"BLOGID" {
		t.Fatalf("aliran blog rawa salah: %+v", rawa)
	}
}

// Uji hidup API Animeku dengan blob Fate ep 1 yang tertangkap:
// hanya jalan bila SUBSENPAI_LIVE_TEST=1. API ini dapat dijangkau
// dari lingkungan dev (beda dengan kuronime.sbs yang ber-403).
func TestKuronimeSourcesLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := fetchKuroSources(ctx, kuroFateBlob, "https://kuronime.sbs/nonton-fate-apocrypha-episode-1/")
	if err != nil {
		t.Fatalf("API Animeku gagal: %v", err)
	}
	streams := kuroStreamsFromAPI(resp, "awar", "id")
	if len(streams) == 0 || !strings.HasPrefix(streams[0].URL, kuroPlayerBase) {
		t.Fatalf("aliran hidup tidak sah: %+v", streams)
	}
	t.Logf("aliran hidup: %d (pertama %s)", len(streams), streams[0].Label)
}
