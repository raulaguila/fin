package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/raulaguila/authenticity/pkg/telemetry"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/rs/zerolog"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/clientip"
	"github.com/raulaguila/authenticity/pkg/logger"
)

// RequestLog always records server errors and degraded responses. all also
// enables ordinary request logs. Only paths are logged, never query strings.
func RequestLog(log *logger.Logger, all bool) fiber.Handler {
	zero := log.Zerolog()
	return func(c fiber.Ctx) error {
		start := time.Now()
		defer func() { telemetry.ObserveRequest(time.Since(start), c.Response().StatusCode()) }()

		failure := c.Next()
		if failure != nil {
			if err := c.App().ErrorHandler(c, failure); err != nil {
				return err
			}
		}

		status := c.Response().StatusCode()
		cause, _ := c.Locals(mapper.LocalCause).(error)
		degraded := cause != nil
		if !all && !degraded && status < fiber.StatusInternalServerError {
			return nil
		}
		if cause == nil {
			cause = failure
		}

		level, message := zerolog.InfoLevel, "HTTP Request"
		switch {
		case status >= fiber.StatusInternalServerError:
			level, message = zerolog.ErrorLevel, "Server error"
		case degraded:
			level, message = zerolog.WarnLevel, "Request degraded"
		case status >= fiber.StatusBadRequest:
			level, message = zerolog.WarnLevel, "Client error"
		}
		event := zero.WithLevel(level).
			Int("status", status).
			Str("method", c.Method()).
			Str("ip", c.IP()).
			Dur("latency", time.Since(start)).
			Str("path", c.Path()).
			Err(cause)
		if appErr, ok := errors.AsType[*apperror.Error](cause); ok && appErr.Cause != nil {
			event.Str("cause", appErr.Cause.Error())
		}
		event.Msg(message)
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

// RateLimit allows max requests per client per minute.
// shortcut: counters live in process memory and reset on restart. Each instance
// counts alone; put limit_req in the proxy before scaling out.
func RateLimit(m int) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        m,
		Expiration: time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			first, _ := clientip.Range(c.IP())
			return first
		},
		LimitReached: func(c fiber.Ctx) error {
			return mapper.WriteError(c, apperror.New(apperror.CodeTooManyRequests, "too many requests"))
		},
	})
}
