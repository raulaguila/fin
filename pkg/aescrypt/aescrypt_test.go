package aescrypt

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		t.Fatalf("random key: %v", err)
	}
	return k
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := mustKey(t)
	ct, err := Encrypt(key, "hello world")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	pt, err := Decrypt(key, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if pt != "hello world" {
		t.Fatalf("expected plaintext 'hello world', got %q", pt)
	}
}

func TestWrapUnwrapRoundTrip(t *testing.T) {
	kek := mustKey(t)
	dek, err := NewDEK()
	if err != nil {
		t.Fatalf("NewDEK: %v", err)
	}
	if len(dek) != DEKSize {
		t.Fatalf("expected DEK size %d, got %d", DEKSize, len(dek))
	}
	wrapped, err := WrapKey(kek, dek)
	if err != nil {
		t.Fatalf("WrapKey: %v", err)
	}
	unwrapped, err := UnwrapKey(kek, wrapped)
	if err != nil {
		t.Fatalf("UnwrapKey: %v", err)
	}
	if !bytes.Equal(dek, unwrapped) {
		t.Fatalf("DEK mismatch after unwrap")
	}
}

func TestUnwrapFailsWithWrongKEK(t *testing.T) {
	kek := mustKey(t)
	wrongKEK := mustKey(t)
	dek, err := NewDEK()
	if err != nil {
		t.Fatalf("NewDEK: %v", err)
	}
	wrapped, err := WrapKey(kek, dek)
	if err != nil {
		t.Fatalf("WrapKey: %v", err)
	}
	if _, err := UnwrapKey(wrongKEK, wrapped); err == nil {
		t.Fatal("expected UnwrapKey to fail with wrong KEK")
	}
}

func TestWrapKeyRejectsBadDEKLength(t *testing.T) {
	kek := mustKey(t)
	if _, err := WrapKey(kek, []byte("too-short")); !errors.Is(err, ErrDEKLength) {
		t.Fatalf("expected ErrDEKLength, got %v", err)
	}
}

func TestPassphraseBundleRoundTrip(t *testing.T) {
	payload := []byte(`{"host":"smtp.example.com","port":587,"smtp_password":"super-secret"}`)
	bundle, err := SealWithPassphrase("correct horse battery staple", payload)
	if err != nil {
		t.Fatalf("SealWithPassphrase: %v", err)
	}
	if bundle.Version != 1 || bundle.Iterations < 10000 {
		t.Fatalf("bundle metadata wrong: %+v", bundle)
	}
	// Marshal → Unmarshal round-trip to mimic the HTTP boundary.
	data, err := MarshalBundle(bundle)
	if err != nil {
		t.Fatalf("MarshalBundle: %v", err)
	}
	parsed, err := UnmarshalBundle(data)
	if err != nil {
		t.Fatalf("UnmarshalBundle: %v", err)
	}
	plain, err := OpenWithPassphrase("correct horse battery staple", parsed)
	if err != nil {
		t.Fatalf("OpenWithPassphrase: %v", err)
	}
	if string(plain) != string(payload) {
		t.Fatalf("plaintext mismatch:\n got:  %s\n want: %s", plain, payload)
	}
}

func TestPassphraseBundleWrongPassphraseFails(t *testing.T) {
	bundle, err := SealWithPassphrase("pass-one", []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := OpenWithPassphrase("pass-two", bundle); err == nil {
		t.Fatal("expected wrong passphrase to fail GCM auth")
	}
}

func TestPassphraseBundleTamperedCiphertextFails(t *testing.T) {
	bundle, err := SealWithPassphrase("pw", []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	// Flip a nibble in the ciphertext.
	if len(bundle.Ciphertext) < 2 {
		t.Fatal("ciphertext too short")
	}
	b := []byte(bundle.Ciphertext)
	if b[0] == 'a' {
		b[0] = 'b'
	} else {
		b[0] = 'a'
	}
	bundle.Ciphertext = string(b)
	if _, err := OpenWithPassphrase("pw", bundle); err == nil {
		t.Fatal("expected tampered ciphertext to fail GCM auth")
	}
}

func TestDEKEncryptsPassword(t *testing.T) {
	kek := mustKey(t)
	dek, err := NewDEK()
	if err != nil {
		t.Fatalf("NewDEK: %v", err)
	}
	// KEK wraps DEK, DEK encrypts secret.
	wrapped, err := WrapKey(kek, dek)
	if err != nil {
		t.Fatalf("WrapKey: %v", err)
	}
	ct, err := Encrypt(dek, "super-secret")
	if err != nil {
		t.Fatalf("Encrypt with DEK: %v", err)
	}
	// Simulate round-trip through storage.
	unwrappedDEK, err := UnwrapKey(kek, wrapped)
	if err != nil {
		t.Fatalf("UnwrapKey: %v", err)
	}
	pt, err := Decrypt(unwrappedDEK, ct)
	if err != nil {
		t.Fatalf("Decrypt with unwrapped DEK: %v", err)
	}
	if pt != "super-secret" {
		t.Fatalf("expected 'super-secret', got %q", pt)
	}
}
