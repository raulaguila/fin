package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// OWASP-oriented interactive defaults (RFC 9106 second recommendation).
const (
	timeCost  = 3
	memoryKiB = 64 * 1024
	threads   = 4
	keyLen    = 32
	saltLen   = 16

	// maxHashes keeps live hash memory at 256 MiB however many CPUs the host has.
	maxHashes = 4
)

// Each hash holds memoryKiB of RAM and runs threads goroutines, so more hashes
// than CPUs/threads add memory without adding throughput.
var slots = make(chan struct{}, max(1, min(runtime.GOMAXPROCS(0)/threads, maxHashes)))

// idKey waits for a slot only as long as ctx lives: nobody reads the answer of
// a request that already timed out.
func idKey(ctx context.Context, plain string, salt []byte, t, m uint32, p uint8, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-slots }()
	return argon2.IDKey([]byte(plain), salt, t, m, p, n), nil
}

var dummyHash = sync.OnceValue(func() string {
	h, err := Hash(context.Background(), "timing equalizer")
	if err != nil {
		panic(err)
	}
	return h
})

// Waste spends the time of a real verify, so accounts without a usable
// password answer as slowly as accounts with one.
func Waste(ctx context.Context, plain string) {
	_, _ = Verify(ctx, dummyHash(), plain)
}

// Hash returns a PHC-encoded argon2id string.
func Hash(ctx context.Context, plain string) (string, error) {
	if plain == "" {
		return "", errors.New("empty password")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash, err := idKey(ctx, plain, salt, timeCost, memoryKiB, threads, keyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memoryKiB,
		timeCost,
		threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// Verify compares plain password against a PHC argon2id hash.
func Verify(ctx context.Context, encoded, plain string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}
	var mem, timeN, pars int
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &timeN, &pars); err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got, err := idKey(ctx, plain, salt, uint32(timeN), uint32(mem), uint8(pars), uint32(len(want)))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
