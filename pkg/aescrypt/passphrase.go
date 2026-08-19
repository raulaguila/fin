package aescrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// Defaults for the passphrase-protected export bundle used by POST
// /platform/smtp/export and /import. Parameters are embedded in every
// bundle so future tweaks stay backward compatible.
const (
	bundleVersion          = 1
	bundlePBKDF2Iterations = 100_000
	bundleSaltSize         = 16 // 128-bit salt per bundle
)

// PassphraseBundle is the wire format for exported SMTP configurations. The
// JSON tags are stable — consumers (CLI tools, future phase-7 KEK rotation,
// other deployments) rely on this shape.
type PassphraseBundle struct {
	Version    int    `json:"version"`
	Salt       string `json:"salt"`       // hex-encoded
	Iterations int    `json:"iterations"` // PBKDF2 rounds actually used
	Nonce      string `json:"nonce"`      // hex-encoded GCM nonce
	Ciphertext string `json:"ciphertext"` // hex-encoded AES-GCM output
}

// DeriveKey runs PBKDF2-HMAC-SHA256 against the passphrase with the supplied
// salt and iteration count, producing a 32-byte AES-256 key. Exposed so
// higher layers can precompute keys for a rotation job without opening the
// bundle twice.
func DeriveKey(passphrase string, salt []byte, iterations int) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, iterations, 32, sha256.New)
}

// SealWithPassphrase encrypts plaintext with a fresh random salt + nonce and
// returns the bundle in its canonical JSON shape. The caller decides what
// `plaintext` contains — we never inspect it.
func SealWithPassphrase(passphrase string, plaintext []byte) (*PassphraseBundle, error) {
	if passphrase == "" {
		return nil, errors.New("aescrypt: passphrase required")
	}
	salt := make([]byte, bundleSaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("aescrypt: generate salt: %w", err)
	}
	key := DeriveKey(passphrase, salt, bundlePBKDF2Iterations)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return &PassphraseBundle{
		Version:    bundleVersion,
		Salt:       hex.EncodeToString(salt),
		Iterations: bundlePBKDF2Iterations,
		Nonce:      hex.EncodeToString(nonce),
		Ciphertext: hex.EncodeToString(ct),
	}, nil
}

// OpenWithPassphrase reverses SealWithPassphrase. Returns the plaintext on
// success; a generic "decryption failed" error on wrong passphrase or
// tampered ciphertext (GCM auth failure). Version is checked so an older
// client cannot silently consume a newer format it does not understand.
func OpenWithPassphrase(passphrase string, bundle *PassphraseBundle) ([]byte, error) {
	if bundle == nil {
		return nil, errors.New("aescrypt: bundle is nil")
	}
	if bundle.Version != bundleVersion {
		return nil, fmt.Errorf("aescrypt: unsupported bundle version %d", bundle.Version)
	}
	if bundle.Iterations < 10_000 {
		return nil, errors.New("aescrypt: bundle iterations too low")
	}
	salt, err := hex.DecodeString(bundle.Salt)
	if err != nil || len(salt) == 0 {
		return nil, errors.New("aescrypt: invalid bundle salt")
	}
	nonce, err := hex.DecodeString(bundle.Nonce)
	if err != nil {
		return nil, errors.New("aescrypt: invalid bundle nonce")
	}
	ct, err := hex.DecodeString(bundle.Ciphertext)
	if err != nil {
		return nil, errors.New("aescrypt: invalid bundle ciphertext")
	}
	key := DeriveKey(passphrase, salt, bundle.Iterations)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("aescrypt: invalid bundle nonce size")
	}
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, errors.New("aescrypt: decryption failed")
	}
	return plaintext, nil
}

// MarshalBundle is a small convenience so handlers can emit a bundle with
// stable field ordering without pulling encoding/json in every caller.
func MarshalBundle(b *PassphraseBundle) ([]byte, error) {
	return json.Marshal(b)
}

// UnmarshalBundle is the inverse of MarshalBundle with a defensive type
// check so we never feed partially-decoded JSON into OpenWithPassphrase.
func UnmarshalBundle(data []byte) (*PassphraseBundle, error) {
	var b PassphraseBundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	if b.Version == 0 || b.Ciphertext == "" || b.Nonce == "" || b.Salt == "" {
		return nil, errors.New("aescrypt: bundle missing required fields")
	}
	return &b, nil
}
