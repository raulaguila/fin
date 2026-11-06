package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/logger"
)

func TestRateLimitAnswers429AsJSON(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RateLimit(2))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	var last int
	var body []byte
	for range 3 {
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		last = resp.StatusCode
		body, _ = io.ReadAll(resp.Body)
	}
	if last != fiber.StatusTooManyRequests {
		t.Fatalf("status %d", last)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil || parsed["code"] != string(apperror.CodeTooManyRequests) {
		t.Fatalf("body %s", body)
	}
}

func TestRequestLogRecordsPanics(t *testing.T) {
	var out bytes.Buffer
	log := logger.New(logger.Config{Format: "json", Level: "info", Out: &out})
	app := fiber.New()
	app.Use(middleware.RequestLog(log, false))
	app.Use(recoverer.New())
	app.Get("/boom", func(fiber.Ctx) error { panic("handler exploded") })

	resp, err := app.Test(httptest.NewRequest("GET", "/boom", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !strings.Contains(out.String(), "handler exploded") {
		t.Fatalf("log: %s", out.String())
	}
}

func TestTimeoutReachesTheHandler(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.Timeout(time.Hour))
	app.Get("/", func(c fiber.Ctx) error {
		if _, ok := c.Context().Deadline(); !ok {
			return c.SendStatus(fiber.StatusTeapot)
		}
		return c.SendStatus(fiber.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("%v %v", resp, err)
	}
}

func TestRequestLogKeepsCauseFromClientAndLog(t *testing.T) {
	var out bytes.Buffer
	log := logger.New(logger.Config{Format: "json", Level: "info", Out: &out})
	app := fiber.New()
	app.Use(middleware.RequestLog(log, false))
	app.Get("/boom", func(c fiber.Ctx) error {
		return mapper.WriteError(c, apperror.New(apperror.CodeDatabaseError, "db error").WithCause(errors.New("pg is on fire")))
	})
	app.Get("/fine", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest("GET", "/boom?token=secret", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "pg is on fire") {
		t.Fatalf("cause leaked to client: %s", body)
	}
	logged := out.String()
	if !strings.Contains(logged, "pg is on fire") || strings.Contains(logged, "secret") {
		t.Fatalf("log: %s", logged)
	}

	out.Reset()
	if _, err := app.Test(httptest.NewRequest("GET", "/fine", nil)); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("2xx logged with all=false: %s", out.String())
	}
}

func TestRequestLogReportsDegradedAnswers(t *testing.T) {
	var out bytes.Buffer
	log := logger.New(logger.Config{Format: "json", Level: "info", Out: &out})
	app := fiber.New()
	app.Use(middleware.RequestLog(log, false))
	app.Get("/", func(c fiber.Ctx) error {
		c.Locals(mapper.LocalCause, errors.New("profile lost"))
		return c.SendStatus(fiber.StatusOK)
	})
	if _, err := app.Test(httptest.NewRequest("GET", "/", nil)); err != nil {
		t.Fatal(err)
	}
	if line := out.String(); !strings.Contains(line, "Request degraded") || !strings.Contains(line, "profile lost") {
		t.Fatalf("log: %s", line)
	}
}
