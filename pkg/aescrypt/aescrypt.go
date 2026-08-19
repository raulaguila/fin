package aescrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

var (
	ErrKeyLength = errors.New("aescrypt: key must be 32 hex-encoded bytes (64 hex chars)")
	ErrDEKLength = errors.New("aescrypt: DEK must be 32 raw bytes")
)

// DEKSize is the required length for a data-encryption key (AES-256).
const DEKSize = 32

// NewDEK generates a cryptographically random 32-byte Data Encryption Key
// suitable for AES-256-GCM. Caller is responsible for wrapping it with a KEK
// before persistence (see WrapKey).
func NewDEK() ([]byte, error) {
	dek := make([]byte, DEKSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("aescrypt: generate DEK: %w", err)
	}
	return dek, nil
}

// ParseKey decodes a 64-character hex string into a 32-byte AES-256 key.
func ParseKey(hexKey string) ([]byte, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKeyLength, err)
	}
	if len(b) != 32 {
		return nil, ErrKeyLength
	}
	return b, nil
}

// Encrypt encrypts plaintext with AES-256-GCM and returns a hex-encoded
// nonce+ciphertext string safe for storage.
func Encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

// WrapKey seals a raw DEK with the KEK using AES-256-GCM and returns the
// nonce||ciphertext bytes. Unlike Encrypt, the return value is raw bytes (not
// hex-encoded) because the result is stored in a BYTEA column.
func WrapKey(kek, dek []byte) ([]byte, error) {
	if len(dek) != DEKSize {
		return nil, ErrDEKLength
	}
	block, err := aes.NewCipher(kek)
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
	return gcm.Seal(nonce, nonce, dek, nil), nil
}

// UnwrapKey reverses WrapKey and returns the raw DEK.
func UnwrapKey(kek, wrapped []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(wrapped) < nonceSize {
		return nil, errors.New("aescrypt: wrapped DEK too short")
	}
	nonce, ct := wrapped[:nonceSize], wrapped[nonceSize:]
	dek, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	if len(dek) != DEKSize {
		return nil, ErrDEKLength
	}
	return dek, nil
}

// Decrypt reverses Encrypt. Returns the plaintext string.
func Decrypt(key []byte, cipherHex string) (string, error) {
	data, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("aescrypt: ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
