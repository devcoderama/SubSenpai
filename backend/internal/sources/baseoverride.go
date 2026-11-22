package sources

// Penimpa base URL sumber lewat environment (permintaan Rams
// 2026-11-22): base_url di tabel content_sources adalah nilai bawaan,
// tetapi tiga sumber utama bisa dipindah domain cukup dengan mengedit
// env + restart service — tanpa menyentuh DB:
//
//	DONGHUA_ID → resolver "anichin"  (bawaan DB: https://anichin.my)
//	ANIME_ID   → resolver "kuronime" (bawaan DB: https://kuronime.sbs)
//	FILM       → resolver "hashflix" (bawaan DB: https://hashflix.top)
//
// Nilai kosong/tidak diset = pakai base_url DB seperti biasa. Timpa
// hanya berlaku di memori saat runtime; data DB tidak pernah diubah.

import (
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
)

// baseOverrideEnv memetakan kunci resolver → nama variabel env
// penimpa base URL-nya.
var baseOverrideEnv = map[string]string{
	"anichin":  "DONGHUA_ID",
	"kuronime": "ANIME_ID",
	"hashflix": "FILM",
}

// warnedOverride mencatat variabel env yang sudah pernah diberi
// peringatan nilai tidak sah, agar log tidak dibanjiri setiap muat.
var warnedOverride sync.Map // nama env → bool

// parseBaseOverride membersihkan & memvalidasi nilai env penimpa:
// spasi dan garis miring akhir dibuang; hanya URL http/https
// ber-host yang diterima.
func parseBaseOverride(raw string) (string, bool) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if v == "" {
		return "", false
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	return v, true
}

func warnInvalidOverride(envName, raw string) {
	if _, loaded := warnedOverride.LoadOrStore(envName, true); !loaded {
		log.Printf("env %s=%q bukan URL http/https yang sah — penimpa base sumber diabaikan", envName, raw)
	}
}

// EffectiveBaseURL mengembalikan base URL efektif satu resolver:
// nilai env penimpa bila diset & sah, selain itu base dari DB apa
// adanya (regresi nol bila env tidak diset).
func EffectiveBaseURL(resolverKey, dbBase string) string {
	envName, ok := baseOverrideEnv[resolverKey]
	if !ok {
		return dbBase
	}
	raw := os.Getenv(envName)
	if strings.TrimSpace(raw) == "" {
		return dbBase
	}
	if v, ok := parseBaseOverride(raw); ok {
		return v
	}
	warnInvalidOverride(envName, raw)
	return dbBase
}

// ApplyBaseOverride menimpa BaseURL sumber ini dengan nilai env
// penimpa untuk resolvernya (bila ada). Dipanggil di SEMUA titik
// bangun Source dari baris content_sources.
func ApplyBaseOverride(s *Source) {
	s.BaseURL = EffectiveBaseURL(s.Resolver, s.BaseURL)
}

// LogBaseOverrides mencatat penimpa base sumber yang aktif (nama
// variabel + host tujuan) — dipanggil sekali saat start server &
// worker agar perpindahan domain sumber terlihat di jurnal.
func LogBaseOverrides() {
	keys := make([]string, 0, len(baseOverrideEnv))
	for k := range baseOverrideEnv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		envName := baseOverrideEnv[key]
		raw := os.Getenv(envName)
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if v, ok := parseBaseOverride(raw); ok {
			u, _ := url.Parse(v)
			log.Printf("override base sumber aktif: %s → %s (resolver %s)", envName, u.Host, key)
		} else {
			warnInvalidOverride(envName, raw)
		}
	}
}
