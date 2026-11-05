package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/logger"
)

const (
	rateKeyPrefix = "ratelimit:"
	rateTimeout   = 100 * time.Millisecond // a stuck cache must not slow every request
)

// RateCounter keeps rate limit windows in Valkey, shared by all processes.
type RateCounter struct {
	rdb *redis.Client
	log *logger.Logger
}

var _ output.RateCounter = (*RateCounter)(nil)

// RateCounter returns the rate limit counters on this connection.
func (c *Client) RateCounter(log *logger.Logger) *RateCounter {
	return &RateCounter{rdb: c.rdb, log: log}
}

// Hit counts atomically: MULTI keeps the window from losing its expiry, and INCR
// never loses a hit to a parallel request, on this instance or another.
func (r *RateCounter) Hit(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, rateTimeout)
	defer cancel()

	key = rateKeyPrefix + key
	pipe := r.rdb.TxPipeline()
	count := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, window)
	ttl := pipe.PTTL(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		r.log.Warn("Rate limit counter failed", slog.String("error", err.Error()))
		return 0, 0, err
	}
	return count.Val(), ttl.Val(), nil
}
