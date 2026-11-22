package jobs

import (
	"testing"

	"subsenpai/backend/internal/sources"
)

// Uji penurunan slug katalog untuk kandidat pencarian live.

func TestSlugFromCandidate(t *testing.T) {
	cases := []struct {
		name string
		cand sources.Candidate
		want string
	}{
		{"animestream series", sources.Candidate{Title: "Soul Land", URL: "https://anichin.my/series/soul-land/"}, "soul-land"},
		{"hianime watch", sources.Candidate{Title: "One Piece", URL: "https://hianime.to/watch/one-piece-100"}, "one-piece-100"},
		{"tanpa url pakai judul", sources.Candidate{Title: "Battle Through the Heavens!"}, "battle-through-the-heavens"},
		{"url akar pakai judul", sources.Candidate{Title: "Solo Leveling", URL: "https://example.com/"}, "solo-leveling"},
		{"kosong semua", sources.Candidate{}, ""},
	}
	for _, c := range cases {
		if got := slugFromCandidate(c.cand); got != c.want {
			t.Errorf("%s: slugFromCandidate = %q, mau %q", c.name, got, c.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	if got := slugify("  Tales of Herding Gods (2024) "); got != "tales-of-herding-gods-2024" {
		t.Errorf("slugify = %q", got)
	}
	if got := slugify("---"); got != "" {
		t.Errorf("slugify tanda hubung = %q, mau kosong", got)
	}
}
