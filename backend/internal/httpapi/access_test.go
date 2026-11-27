package httpapi

import "testing"

// Aturan nonton (diubah Rams 2026-11-27): anime & donghua bebas
// untuk semua — semua episode tanpa premium dan tanpa iklan;
// film khusus premium (atau dibuka lewat iklan).
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
		{tierFree, "anime", false, true},
		{tierFree, "donghua", false, true},
		{tierFree, "film", true, false},
		{tierFree, "film", false, false},
		{tierGuest, "anime", true, true},
		{tierGuest, "donghua", true, true},
		{tierGuest, "anime", false, true},
		{tierGuest, "donghua", false, true},
		{tierGuest, "film", true, false},
		{tierGuest, "film", false, false},
	}
	for _, c := range cases {
		if got := watchAllowed(c.tier, c.titleType, c.isFirst); got != c.want {
			t.Errorf("watchAllowed(%s, %s, %v) = %v, mau %v",
				c.tier, c.titleType, c.isFirst, got, c.want)
		}
	}
}
