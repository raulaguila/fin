package cache

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/raulaguila/authenticity/pkg/logger"
)

// A cache that accepts connections and never answers must fail, fast.
func TestStuckCacheFailsQuickly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	rdb := redis.NewClient(&redis.Options{Addr: ln.Addr().String(), Protocol: 2, ContextTimeoutEnabled: true})
	defer func() { _ = rdb.Close() }()
	counter := (&Client{rdb: rdb}).RateCounter(logger.New(logger.Config{}))

	start := time.Now()
	if _, _, err := counter.Hit(context.Background(), "k", time.Minute); err == nil {
		t.Fatal("no error from a stuck cache")
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("took %s", took)
	}
}
