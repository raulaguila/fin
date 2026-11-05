package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

// Client wraps go-redis for Valkey/Redis.
type Client struct {
	rdb *redis.Client
}

var _ output.CacheStore = (*Client)(nil)

func Connect(cfg *config.CacheConfig) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cache config is nil")
	}

	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		// The cache sits in the path of every request: a stuck one must fail fast.
		DialTimeout:           time.Second,
		ReadTimeout:           500 * time.Millisecond,
		WriteTimeout:          500 * time.Millisecond,
		MaxRetries:            1,
		ContextTimeoutEnabled: true,
	}
	if cfg.User != "" && cfg.User != "default" {
		opts.Username = cfg.User
	}

	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("cache ping: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	return c.rdb.Get(ctx, key).Bytes()
}

func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}
