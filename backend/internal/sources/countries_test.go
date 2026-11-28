package sources

import "testing"

// Uji peta negara ISO -> nama Indonesia (dipakai kolom country).
func TestCountryName(t *testing.T) {
	cases := map[string]string{
		"ID": "Indonesia",
		"id": "Indonesia",
		"US": "Amerika Serikat",
		"GB": "Inggris",
		"JP": "Jepang",
		"KR": "Korea Selatan",
		"XX": "XX", // tak dikenal: kembali apa adanya
		"":   "",
	}
	for in, want := range cases {
		if got := CountryName(in); got != want {
			t.Errorf("CountryName(%q) = %q, mau %q", in, got, want)
		}
	}
	if len(isoCountryNames) < 100 {
		t.Errorf("peta negara hanya %d entri", len(isoCountryNames))
	}
}
