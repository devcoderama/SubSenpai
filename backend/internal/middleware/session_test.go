package middleware

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

// Uji pulang-pergi penuh: handshake ECDH → kunci sama di kedua sisi →
// envelope terbuka kembali ke plaintext asal.
func TestHandshakeEnvelopeRoundtrip(t *testing.T) {
	sessions := NewSessions(time.Hour)

	clientPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientPubB64 := base64.RawURLEncoding.EncodeToString(clientPriv.PublicKey().Bytes())

	id, serverPubB64, _, err := sessions.Handshake(clientPubB64)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("id sesi kosong")
	}

	serverPubBytes, err := base64.RawURLEncoding.DecodeString(serverPubB64)
	if err != nil {
		t.Fatal(err)
	}
	serverPub, err := ecdh.X25519().NewPublicKey(serverPubBytes)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := clientPriv.ECDH(serverPub)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := DeriveKeyFromShared(shared)
	if err != nil {
		t.Fatal(err)
	}

	serverKey, ok := sessions.Key(id)
	if !ok {
		t.Fatal("sesi tidak ditemukan")
	}

	plain := []byte(`{"judul":"uji"}`)
	xdata, xtime, err := Seal(serverKey, plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(clientKey, xdata, xtime)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("hasil dekripsi beda: %s", got)
	}

	// xtime basi harus ditolak.
	if _, err := Open(clientKey, xdata, xtime-1000); err == nil {
		t.Fatal("xtime basi seharusnya ditolak")
	}
	// Sesi tak dikenal harus gagal.
	if _, ok := sessions.Key("tidak-ada"); ok {
		t.Fatal("sesi tak dikenal seharusnya gagal")
	}
}
