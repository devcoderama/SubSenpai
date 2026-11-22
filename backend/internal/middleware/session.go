// Package middleware berisi sesi enkripsi xdata (ECDH), pembungkus envelope
// response, dan pembatas laju — sesuai rancangan STRUKTUR.md Bagian 11.
//
// Skema enkripsi (persis rancangan):
//   - Handshake POST /api/v1/session: klien & server bertukar kunci publik
//     X25519 sementara; kunci sesi = HKDF-SHA256(ECDH, info "subsenpai-xdata-v1").
//   - Kunci hanya hidup di memori kedua sisi; sesi maks. 24 jam.
//   - Response JSON dibungkus {xdata, xtime}: AES-256-GCM,
//     xdata = base64url-nopad(IV 12 byte ‖ ciphertext ‖ tag),
//     xtime (detik Unix) menjadi AAD dan ditolak bila selisih > 300 detik.
package middleware

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"
)

const (
	hkdfInfo      = "subsenpai-xdata-v1"
	maxClockSkew  = 300 * time.Second
	sessionIDSize = 32
)

var b64 = base64.RawURLEncoding

type session struct {
	key       []byte
	expiresAt time.Time
}

// Sessions adalah penyimpan sesi handshake di memori server.
type Sessions struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]*session
}

func NewSessions(ttl time.Duration) *Sessions {
	return &Sessions{ttl: ttl, m: make(map[string]*session)}
}

// Handshake membuat sesi baru dari kunci publik klien (base64url, 32 byte).
// Mengembalikan id sesi, kunci publik server (base64url), dan waktu berakhir.
func (s *Sessions) Handshake(clientPubB64 string) (string, string, time.Time, error) {
	pubBytes, err := b64.DecodeString(clientPubB64)
	if err != nil {
		return "", "", time.Time{}, errors.New("kunci publik klien tidak valid")
	}
	clientPub, err := ecdh.X25519().NewPublicKey(pubBytes)
	if err != nil {
		return "", "", time.Time{}, errors.New("kunci publik klien tidak valid")
	}
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", time.Time{}, err
	}
	shared, err := priv.ECDH(clientPub)
	if err != nil {
		return "", "", time.Time{}, err
	}
	key, err := hkdf.Key(sha256.New, shared, nil, hkdfInfo, 32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	idRaw := make([]byte, sessionIDSize)
	if _, err := rand.Read(idRaw); err != nil {
		return "", "", time.Time{}, err
	}
	id := hex.EncodeToString(idRaw)
	expires := time.Now().Add(s.ttl)
	s.mu.Lock()
	s.m[id] = &session{key: key, expiresAt: expires}
	s.mu.Unlock()
	return id, b64.EncodeToString(priv.PublicKey().Bytes()), expires, nil
}

// Key mengambil kunci sesi yang masih berlaku.
func (s *Sessions) Key(id string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[id]
	if !ok {
		return nil, false
	}
	if time.Now().After(sess.expiresAt) {
		delete(s.m, id)
		return nil, false
	}
	return sess.key, true
}

// Seal mengenkripsi plaintext menjadi pasangan (xdata, xtime).
func Seal(key, plaintext []byte) (string, int64, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", 0, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", 0, err
	}
	xtime := time.Now().Unix()
	aad := []byte(strconv.FormatInt(xtime, 10))
	ct := gcm.Seal(nil, nonce, plaintext, aad)
	payload := append(nonce, ct...)
	return b64.EncodeToString(payload), xtime, nil
}

// Open membuka xdata dengan kunci sesi (dipakai pengujian & klien referensi).
func Open(key []byte, xdata string, xtime int64) ([]byte, error) {
	if d := time.Since(time.Unix(xtime, 0)); d > maxClockSkew || d < -maxClockSkew {
		return nil, errors.New("xtime di luar toleransi 300 detik")
	}
	payload, err := b64.DecodeString(xdata)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(payload) < gcm.NonceSize() {
		return nil, errors.New("xdata terlalu pendek")
	}
	aad := []byte(strconv.FormatInt(xtime, 10))
	return gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], aad)
}

// DeriveKeyFromShared menurunkan kunci sesi dari rahasia bersama ECDH —
// dipakai sisi klien (pengujian) agar simetris dengan Handshake.
func DeriveKeyFromShared(shared []byte) ([]byte, error) {
	return hkdf.Key(sha256.New, shared, nil, hkdfInfo, 32)
}
