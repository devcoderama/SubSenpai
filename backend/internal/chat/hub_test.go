package chat

import "testing"

func TestContainsLink(t *testing.T) {
	dilarang := []string{
		"cek ini https://contoh.com/abc",
		"kunjungi www.situs-keren.net",
		"download di filegratis.xyz sekarang",
		"gabung grup tele di t.me/gruprahasia",
	}
	for _, s := range dilarang {
		if !ContainsLink(s) {
			t.Errorf("harus ditolak: %q", s)
		}
	}
	boleh := []string{
		"episode 5 bagus banget, skor 9.5 dari aku",
		"besok jam 19.30 rilis ya",
		"aku suka ending-nya... titik.",
	}
	for _, s := range boleh {
		if ContainsLink(s) {
			t.Errorf("tidak boleh ditolak: %q", s)
		}
	}
}
