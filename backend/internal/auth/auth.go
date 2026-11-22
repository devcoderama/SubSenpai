// Package auth berisi verifikasi token ID Google (JWKS RS256), JWT sesi
// backend (HS256), dan middleware otorisasi. Sesuai rancangan: login hanya
// Google; backend memverifikasi token Google lalu menerbitkan JWT-nya
// sendiri — aplikasi tidak pernah membawa token Google setelah login.
package auth

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

var b64 = base64.RawURLEncoding

// ---------- Token ID Google ----------

// GoogleClaims adalah klaim penting dari token ID Google terverifikasi.
type GoogleClaims struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// GoogleVerifier memverifikasi token ID Google terhadap kunci publik
// Google (JWKS) yang di-cache di memori.
type GoogleVerifier struct {
	clientID  string
	certsURL  string
	client    *http.Client
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func NewGoogleVerifier(clientID, certsURL string) *GoogleVerifier {
	return &GoogleVerifier{
		clientID: clientID,
		certsURL: certsURL,
		client:   &http.Client{Timeout: 10 * time.Second},
		keys:     map[string]*rsa.PublicKey{},
	}
}

func (v *GoogleVerifier) keyFor(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok && time.Since(v.fetchedAt) < time.Hour {
		return k, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ambil sertifikat Google: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sertifikat Google: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("JWKS Google tidak terbaca: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range jwks.Keys {
		nBytes, err1 := b64.DecodeString(k.N)
		eBytes, err2 := b64.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 | int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
	}
	v.keys = keys
	v.fetchedAt = time.Now()
	k, ok := keys[kid]
	if !ok {
		return nil, errors.New("kid tidak dikenal")
	}
	return k, nil
}

// Verify memverifikasi tanda tangan + klaim token ID Google.
func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (*GoogleClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("format token tidak valid")
	}
	headerBytes, err := b64.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("header token tidak valid")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Alg != "RS256" || header.Kid == "" {
		return nil, errors.New("header token tidak valid")
	}
	key, err := v.keyFor(ctx, header.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("tanda tangan tidak valid")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return nil, errors.New("tanda tangan tidak valid")
	}
	payloadBytes, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("payload token tidak valid")
	}
	var claims struct {
		Iss           string `json:"iss"`
		Aud           string `json:"aud"`
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		Exp           int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("payload token tidak valid")
	}
	if claims.Iss != "accounts.google.com" && claims.Iss != "https://accounts.google.com" {
		return nil, errors.New("penerbit token bukan Google")
	}
	if v.clientID != "" && claims.Aud != v.clientID {
		return nil, errors.New("audiens token tidak cocok")
	}
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token kedaluwarsa")
	}
	if claims.Sub == "" || claims.Email == "" {
		return nil, errors.New("token tanpa sub/email")
	}
	if !claims.EmailVerified {
		return nil, errors.New("email Google belum terverifikasi")
	}
	return &GoogleClaims{
		Sub: claims.Sub, Email: claims.Email, EmailVerified: claims.EmailVerified,
		Name: claims.Name, Picture: claims.Picture,
	}, nil
}

// ---------- JWT sesi backend (HS256) ----------

// IssueJWT menerbitkan JWT sesi backend untuk satu profil.
func IssueJWT(secret, profileID, role string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("JWT_SECRET belum diatur")
	}
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"sub":  profileID,
		"role": role,
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
	})
	signing := b64.EncodeToString(header) + "." + b64.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + b64.EncodeToString(mac.Sum(nil)), nil
}

// ParseJWT memverifikasi JWT sesi backend; mengembalikan id profil & peran.
func ParseJWT(secret, token string) (string, string, error) {
	if secret == "" {
		return "", "", errors.New("JWT_SECRET belum diatur")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", errors.New("format JWT tidak valid")
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return "", "", errors.New("tanda tangan JWT tidak valid")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", "", errors.New("tanda tangan JWT tidak valid")
	}
	payloadBytes, err := b64.DecodeString(parts[1])
	if err != nil {
		return "", "", errors.New("payload JWT tidak valid")
	}
	var claims struct {
		Sub  string `json:"sub"`
		Role string `json:"role"`
		Exp  int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil || claims.Sub == "" {
		return "", "", errors.New("payload JWT tidak valid")
	}
	if time.Now().Unix() > claims.Exp {
		return "", "", errors.New("JWT kedaluwarsa")
	}
	return claims.Sub, claims.Role, nil
}
