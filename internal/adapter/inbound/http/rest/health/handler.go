package health

import (
	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/core/port/input"
)

type Handler struct {
	uc input.HealthChecker
}

func NewHandler(uc input.HealthChecker) *Handler {
	return &Handler{uc: uc}
}

// Routes mounts the versioned health route; /health stays outside, before the limiter.
func (h *Handler) Routes(r fiber.Router) {
	r.Get("/health", h.Get)
}

type responseDTO struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Get handles GET /health (and /v1/health).
//
// @Summary Service status
// @Tags Health
// @Produce json
// @Success 200 {object} responseDTO
// @Failure 503 {object} mapper.errorBody
// @Router /health [get]
// @Router /v1/health [get]
func (h *Handler) Get(c fiber.Ctx) error {
	report, err := h.uc.Check(c.Context())
	if err != nil {
		return mapper.WriteError(c, err)
	}

	checks := make(map[string]string, len(report.Checks))
	for k, v := range report.Checks {
		checks[k] = string(v)
	}

	status := fiber.StatusOK
	if report.Status == "down" {
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(responseDTO{
		Status: report.Status,
		Checks: checks,
	})
}
