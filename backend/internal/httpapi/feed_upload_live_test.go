package httpapi

import (
	"context"
	"os"
	"testing"
)

// Uji hidup uploadImage2URL — hanya jalan bila SUBSENPAI_LIVE_TEST=1.
// Memakai berkas /tmp/test-vid.mp4 (video uji kecil).
func TestUploadImage2URLLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji hidup dinonaktifkan")
	}
	data, err := os.ReadFile("/tmp/test-vid.mp4")
	if err != nil {
		t.Fatalf("berkas uji tidak ada: %v", err)
	}
	a := &API{}
	url, err := a.uploadImage2URL(context.Background(), data, ".mp4")
	if err != nil {
		t.Fatalf("uploadImage2URL gagal: %v", err)
	}
	t.Logf("URL hasil: %s", url)
	if len(url) < 40 || url[:37] != "https://www.image2url.com/r2/default/" {
		t.Fatalf("URL tidak sesuai pola: %s", url)
	}
}
