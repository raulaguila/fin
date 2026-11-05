package middleware

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/clientip"
	"github.com/raulaguila/authenticity/pkg/logger"
)

// RequestLog records every 5xx with its cause, and every request when all is
// true. It logs the path only: query strings and bodies may carry secrets.
func RequestLog(log *logger.Logger, all bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		var failure error
		if err := c.Next(); err != nil { // panics and Fiber errors arrive here
			failure = err
			if err := c.App().ErrorHandler(c, err); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		status := c.Response().StatusCode()
		if status < fiber.StatusInternalServerError && !all {
			return nil
		}
		attrs := []slog.Attr{
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
		}
		if status < fiber.StatusInternalServerError {
			log.Info("Request", attrs...)
			return nil
		}
		if cause, ok := c.Locals(mapper.LocalCause).(error); ok {
			failure = cause
		}
		if failure != nil {
			attrs = append(attrs, slog.String("error", failure.Error()))
			if appErr, ok := errors.AsType[*apperror.Error](failure); ok && appErr.Cause != nil {
				attrs = append(attrs, slog.String("cause", appErr.Cause.Error()))
			}
		}
		log.Error("Request failed", attrs...)
		return nil
	}
}

// Timeout bounds the work of a request, so a slow database cannot pile up
// requests without end.
func Timeout(d time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), d)
		defer cancel()
		c.SetContext(ctx)
		return c.Next()
	}
}

const rateWindow = time.Minute

// RateLimit allows max requests per client per minute. name separates limiters
// sharing one counter. A nil counter, or one that fails, leaves counting in this
// process: limits stay on, but each instance counts alone.
func RateLimit(name string, max int, counter output.RateCounter) fiber.Handler {
	key := func(c fiber.Ctx) string {
		first, _ := clientip.Range(c.IP())
		return name + ":" + first
	}
	reached := func(c fiber.Ctx) error {
		return mapper.WriteError(c, apperror.New(apperror.CodeTooManyRequests, "too many requests"))
	}
	local := limiter.New(limiter.Config{Max: max, Expiration: rateWindow, KeyGenerator: key, LimitReached: reached})
	if counter == nil {
		return local
	}

	return func(c fiber.Ctx) error {
		hits, reset, err := counter.Hit(c.Context(), key(c), rateWindow)
		if err != nil {
			return local(c)
		}
		seconds := strconv.Itoa(int(reset.Seconds()) + 1)
		remaining := int64(max) - hits
		if remaining < 0 {
			remaining = 0
		}
		c.Set("X-RateLimit-Limit", strconv.Itoa(max))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Set("X-RateLimit-Reset", seconds)
		if hits > int64(max) {
			c.Set(fiber.HeaderRetryAfter, seconds)
			return reached(c)
		}
		return c.Next()
	}
}
