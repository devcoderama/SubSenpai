package payments

import (
	"testing"
	"time"
)

func TestMatchNominalUnik(t *testing.T) {
	created := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	expires := created.Add(15 * time.Minute)
	txs := []Tx{
		{Amount: 29000, Time: created.Add(3 * time.Minute), Ref: "bukan"},
		{Amount: 29456, Time: created.Add(20 * time.Minute), Ref: "terlambat"},
		{Amount: 29456, Time: created.Add(5 * time.Minute), Ref: "pas"},
	}
	ref, ok := Match(txs, 29456, created, expires)
	if !ok || ref != "pas" {
		t.Fatalf("Match = %q, %v; mau pas, true", ref, ok)
	}
	if _, ok := Match(txs, 29111, created, expires); ok {
		t.Fatal("nominal tak dikenal tidak boleh cocok")
	}
}

func TestContainsLinkAdaDiChat(t *testing.T) {
	// Pemeriksa tautan tinggal di paket chat; di sini hanya memastikan
	// paket payments tidak ikut campur. (Uji filter ada di chat_test.)
}
