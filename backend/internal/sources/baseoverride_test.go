package sources

import "testing"

// Uji penimpa base URL sumber lewat env (DONGHUA_ID/ANIME_ID/FILM):
// peta variabel→resolver, pembersihan nilai, penolakan skema asing,
// dan regresi nol bila env tidak diset.

func TestParseBaseOverride(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"", "", false},
		{"   ", "", false},
		{"https://anichin.moe", "https://anichin.moe", true},
		{"https://anichin.moe/", "https://anichin.moe", true},
		{"  https://kuronime.sbs//  ", "https://kuronime.sbs", true},
		{"http://hashflix.top", "http://hashflix.top", true},
		{"ftp://anichin.moe", "", false},
		{"anichin.moe", "", false},
		{"https://", "", false},
		{"javascript:alert(1)", "", false},
	}
	for _, c := range cases {
		got, ok := parseBaseOverride(c.raw)
		if got != c.want || ok != c.ok {
			t.Errorf("parseBaseOverride(%q) = %q,%v — mau %q,%v", c.raw, got, ok, c.want, c.ok)
		}
	}
}

func TestEffectiveBaseURL(t *testing.T) {
	// Env tidak diset → base DB apa adanya.
	if got := EffectiveBaseURL("anichin", "https://anichin.my"); got != "https://anichin.my" {
		t.Errorf("tanpa env = %q, mau base DB", got)
	}
	// Resolver di luar peta → tidak pernah ditimpa.
	t.Setenv("DONGHUA_ID", "https://anichin.moe")
	if got := EffectiveBaseURL("hianime", "https://hianime.to"); got != "https://hianime.to" {
		t.Errorf("resolver tak terpeta = %q, mau base DB", got)
	}
	// Env sah (dengan garis miring akhir) → menimpa.
	if got := EffectiveBaseURL("anichin", "https://anichin.my"); got != "https://anichin.moe" {
		t.Errorf("DONGHUA_ID sah = %q, mau https://anichin.moe", got)
	}
	// Env tidak sah → diabaikan, base DB dipakai.
	t.Setenv("ANIME_ID", "ftp://kuronime.sbs")
	if got := EffectiveBaseURL("kuronime", "https://kuronime.sbs"); got != "https://kuronime.sbs" {
		t.Errorf("ANIME_ID tak sah = %q, mau base DB", got)
	}
	// Env hanya spasi → dianggap tidak diset.
	t.Setenv("FILM", "   ")
	if got := EffectiveBaseURL("hashflix", "https://hashflix.top"); got != "https://hashflix.top" {
		t.Errorf("FILM kosong = %q, mau base DB", got)
	}
}

func TestApplyBaseOverride(t *testing.T) {
	t.Setenv("DONGHUA_ID", "https://anichin.moe/")
	s := Source{Resolver: "anichin", BaseURL: "https://anichin.my"}
	ApplyBaseOverride(&s)
	if s.BaseURL != "https://anichin.moe" {
		t.Errorf("ApplyBaseOverride = %q, mau https://anichin.moe", s.BaseURL)
	}
}

// Uji URL seri hasil Discover per host base: .moe level akar,
// host lain tetap /series/<slug>/ (regresi nol untuk base bawaan DB).
func TestSeriesURL(t *testing.T) {
	cases := []struct{ base, slug, want string }{
		{"https://anichin.moe", "soul-land", "https://anichin.moe/soul-land/"},
		{"https://anichin.moe/", "soul-land", "https://anichin.moe/soul-land/"},
		{"https://anichin.my", "soul-land", "https://anichin.my/series/soul-land/"},
		{"https://kuronime.sbs", "one-piece", "https://kuronime.sbs/series/one-piece/"},
	}
	for _, c := range cases {
		if got := seriesURL(c.base, c.slug); got != c.want {
			t.Errorf("seriesURL(%q,%q) = %q — mau %q", c.base, c.slug, got, c.want)
		}
	}
}
