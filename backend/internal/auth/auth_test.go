package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func signGoogleLike(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	payload, _ := json.Marshal(claims)
	signing := b64u(header) + "." + b64u(payload)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64u(sig)
}

func TestGoogleVerifyAndJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks, _ := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kid": "test-kid",
			"n":   b64u(key.PublicKey.N.Bytes()),
			"e":   b64u(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jwks)
	}))
	defer srv.Close()

	v := NewGoogleVerifier("client-123", srv.URL)
	now := time.Now()
	tok := signGoogleLike(t, key, "test-kid", map[string]any{
		"iss": "https://accounts.google.com", "aud": "client-123",
		"sub": "google-sub-1", "email": "user@example.com", "email_verified": true,
		"name": "User Uji", "exp": now.Add(time.Hour).Unix(),
	})
	claims, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("verify token sah gagal: %v", err)
	}
	if claims.Sub != "google-sub-1" || claims.Email != "user@example.com" {
		t.Fatalf("klaim salah: %+v", claims)
	}

	bad := signGoogleLike(t, key, "test-kid", map[string]any{
		"iss": "https://accounts.google.com", "aud": "client-LAIN",
		"sub": "x", "email": "x@example.com", "email_verified": true,
		"exp": now.Add(time.Hour).Unix(),
	})
	if _, err := v.Verify(context.Background(), bad); err == nil {
		t.Fatal("token dengan audiens salah seharusnya ditolak")
	}

	jwt, err := IssueJWT("rahasia-uji", "profil-1", "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sub, role, err := ParseJWT("rahasia-uji", jwt)
	if err != nil || sub != "profil-1" || role != "user" {
		t.Fatalf("parse JWT gagal: %v %q %q", err, sub, role)
	}
	if _, _, err := ParseJWT("rahasia-LAIN", jwt); err == nil {
		t.Fatal("JWT dengan secret lain seharusnya ditolak")
	}
}
