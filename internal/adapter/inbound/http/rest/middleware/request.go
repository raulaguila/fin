package middleware

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/raulaguila/authenticity/pkg/telemetry"

	fiberzerolog "github.com/gofiber/contrib/v3/zerolog"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/rs/zerolog"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/clientip"
	"github.com/raulaguila/authenticity/pkg/logger"
)

func defaultZerolog(log *logger.Logger) *zerolog.Logger {
	if log == nil || log.Zerolog() == nil {
		base := zerolog.New(os.Stdout).With().Timestamp().Logger()
		return &base
	}
	return log.Zerolog()
}

// RequestLog always records server errors and degraded responses. all also
// enables ordinary request logs. Only paths are logged, never query strings.
func RequestLog(log *logger.Logger, all bool) fiber.Handler {
	zero := defaultZerolog(log)

	handler := fiberzerolog.New(fiberzerolog.Config{
		Logger: zero,
		Skip: func(c fiber.Ctx) bool {
			cause, _ := c.Locals(mapper.LocalCause).(error)
			// The wrapper records mapped errors and degraded responses with
			// their internal cause; contrib handles ordinary and returned errors.
			return cause != nil || (!all && c.Response().StatusCode() < fiber.StatusInternalServerError)
		},
		Fields: []string{
			fiberzerolog.FieldStatus,
			fiberzerolog.FieldMethod,
			fiberzerolog.FieldIP,
			fiberzerolog.FieldLatency,
			fiberzerolog.FieldPID,
			fiberzerolog.FieldError,
			fiberzerolog.FieldPath,
		},
		FieldsSnakeCase: true,
		Messages:        []string{"Server error", "Client error", "HTTP Request"},
	})

	return func(c fiber.Ctx) error {
		start := time.Now()
		defer func() { telemetry.ObserveRequest(time.Since(start), c.Response().StatusCode()) }()
		err := handler(c) // contrib invokes the error handler before logging
		cause, _ := c.Locals(mapper.LocalCause).(error)
		if cause == nil {
			return err
		}

		level, message := zerolog.WarnLevel, "Request degraded"
		status := c.Response().StatusCode()
		if status >= fiber.StatusInternalServerError {
			level, message = zerolog.ErrorLevel, "Server error"
		}
		event := zero.WithLevel(level).
			Int("status", status).
			Str("method", c.Method()).
			Str("ip", c.IP()).
			Dur("latency", time.Since(start)).
			Int("pid", os.Getpid()).
			Str("path", c.Path()).
			Err(cause)
		if appErr, ok := errors.AsType[*apperror.Error](cause); ok && appErr.Cause != nil {
			event.Str("cause", appErr.Cause.Error())
		}
		event.Msg(message)
		return err
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
