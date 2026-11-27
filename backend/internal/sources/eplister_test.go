package sources

import "testing"

// TestParseEplisterMovieAndShortSlug menguji penulisan ulang
// parseEplister (diagnosis 2026-11-27): parser lama hanya mengenali
// link "-episode-N" + filter prefix slug seri, sehingga (1) halaman
// movie/special yang link-nya "/deep-sea-subtitle-indonesia/" dan
// labelnya "Movie" tidak terbaca sama sekali, dan (2) halaman seri
// yang link episodenya pakai slug pendek (ARCANE) ditolak filter
// prefix. Fixture diambil dari HTML asli anichin.moe 2026-11-27.
func TestParseEplisterMovieAndShortSlug(t *testing.T) {
	deepSea := `<div class="eplister"><div class="ephead"><div class="eph-num">Ep</div></div><ul><li data-index="0"> <a href="/deep-sea-subtitle-indonesia/"> <div class="epl-num">Movie</div> <div class="epl-title">Deep Sea Subtitle Indonesia</div> <div class="epl-sub"><span class="status Sub">Sub</span></div> <div class="epl-date">July 11, 2023</div> </a></li></ul></div>`
	got := parseEplister(deepSea, "https://anichin.moe/deep-sea/")
	if len(got) != 1 {
		t.Fatalf("Deep Sea: %d episode, harus 1: %+v", len(got), got)
	}
	if got[0].Number != 1 || got[0].URL != "https://anichin.moe/deep-sea-subtitle-indonesia/" {
		t.Fatalf("Deep Sea: %+v, harus episode 1 ke /deep-sea-subtitle-indonesia/", got[0])
	}

	arcane := `<div class="eplister"><div class="ephead"></div><ul>` +
		`<li data-index="0"> <a href="/arcane-league-of-legends-episode-09-tamat-subtitle-indonesia/"> <div class="epl-num">9</div> </a></li>` +
		`<li data-index="1"> <a href="/arcane-league-of-legends-episode-08-subtitle-indonesia/"> <div class="epl-num">8</div> </a></li>` +
		`<li data-index="2"> <a href="/arcane-league-of-legends-episode-07-subtitle-indonesia/"> <div class="epl-num">7</div> </a></li>` +
		`<li data-index="3"> <a href="/arcane-league-of-legends-episode-06-subtitle-indonesia/"> <div class="epl-num">6</div> </a></li>` +
		`<li data-index="4"> <a href="/arcane-league-of-legends-episode-05-subtitle-indonesia/"> <div class="epl-num">5</div> </a></li>` +
		`<li data-index="5"> <a href="/arcane-league-of-legends-episode-04-subtitle-indonesia/"> <div class="epl-num">4</div> </a></li>` +
		`<li data-index="6"> <a href="/arcane-league-of-legends-episode-03-subtitle-indonesia/"> <div class="epl-num">3</div> </a></li>` +
		`<li data-index="7"> <a href="/arcane-league-of-legends-episode-02-subtitle-indonesia/"> <div class="epl-num">2</div> </a></li>` +
		`<li data-index="8"> <a href="/arcane-league-of-legends-episode-01-subtitle-indonesia/"> <div class="epl-num">1</div> </a></li>` +
		`</ul></div>`
	got = parseEplister(arcane, "https://anichin.moe/arcane-league-of-legends-battle-of-the-two-cities/")
	if len(got) != 9 {
		t.Fatalf("ARCANE: %d episode, harus 9: %+v", len(got), got)
	}
	for i, e := range got {
		if e.Number != i+1 {
			t.Fatalf("ARCANE: urutan %d bernomor %d, harus %d", i, e.Number, i+1)
		}
	}
	if got[8].URL != "https://anichin.moe/arcane-league-of-legends-episode-09-tamat-subtitle-indonesia/" {
		t.Fatalf("ARCANE ep 9: %s", got[8].URL)
	}

	btth := `<div class="eplister"><div class="ephead"></div><ul>` +
		`<li data-index="0"> <a href="/btth-movie-battle-of-two-emperors-subtitle-indonesia/"> <div class="epl-num">Final Movie</div> <div class="epl-title">Final Battle BTTH</div> </a></li>` +
		`<li data-index="1"> <a href="/pv-btth-movie-battle-of-two-emperors-subtitle-indonesia/"> <div class="epl-num">PV</div> <div class="epl-title">PV BTTH Movie: Battle of Two Emperors Subtitle Indonesia</div> </a></li>` +
		`</ul></div>`
	got = parseEplister(btth, "https://anichin.moe/btth-movie-battle-of-two-emperors/")
	if len(got) != 1 {
		t.Fatalf("BTTH Movie: %d episode, harus 1 (PV dilewati): %+v", len(got), got)
	}
	if got[0].Number != 1 || got[0].URL != "https://anichin.moe/btth-movie-battle-of-two-emperors-subtitle-indonesia/" {
		t.Fatalf("BTTH Movie: %+v", got[0])
	}

	// Cadangan regex lama tetap jalan bila markup <li> tidak ada.
	legacy := `<div class="eplister"><ul><a href="/some-slug-episode-3-sub-indo/">Ep 3</a></ul></div>`
	got = parseEplister(legacy, "https://anichin.moe/some-slug/")
	if len(got) != 1 || got[0].Number != 3 {
		t.Fatalf("legacy: %+v, harus episode 3", got)
	}
}
