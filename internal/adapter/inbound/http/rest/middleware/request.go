package middleware

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/pkg/apperror"
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

// RateLimit allows max requests per client IP per minute.
// shortcut: counters live in process memory, move them to Valkey before
// running more than one backend instance or prefork.
func RateLimit(max int) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: time.Minute,
		LimitReached: func(c fiber.Ctx) error {
			return mapper.WriteError(c, apperror.New(apperror.CodeTooManyRequests, "too many requests"))
		},
	})
}
