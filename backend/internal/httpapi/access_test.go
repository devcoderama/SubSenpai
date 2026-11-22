package httpapi

import "testing"

// Aturan nonton (permintaan Rams 2026-11-22): premium bebas;
// non-premium hanya episode pertama anime/donghua; film premium.
func TestWatchAllowed(t *testing.T) {
	cases := []struct {
		tier      tier
		titleType string
		isFirst   bool
		want      bool
	}{
		{tierPremium, "film", false, true},
		{tierPremium, "anime", false, true},
		{tierPremium, "donghua", true, true},
		{tierFree, "anime", true, true},
		{tierFree, "donghua", true, true},
		{tierFree, "anime", false, false},
		{tierFree, "donghua", false, false},
		{tierFree, "film", true, false},
		{tierFree, "film", false, false},
		{tierGuest, "anime", true, true},
		{tierGuest, "donghua", true, true},
		{tierGuest, "anime", false, false},
		{tierGuest, "film", true, false},
	}
	for _, c := range cases {
		if got := watchAllowed(c.tier, c.titleType, c.isFirst); got != c.want {
			t.Errorf("watchAllowed(%s, %s, %v) = %v, mau %v",
				c.tier, c.titleType, c.isFirst, got, c.want)
		}
	}
}
