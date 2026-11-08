package audit

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

type Handler struct {
	svc input.AuditService
}

func New(svc input.AuditService) *Handler {
	return &Handler{svc: svc}
}

// Routes expects a router already behind Bearer.
func (h *Handler) Routes(r fiber.Router, authz input.Authorizer) {
	r.Get("/", middleware.Require(authz, domain.ResourceAudit, domain.ActionRead), h.List)
}

type recordDTO struct {
	ID          string         `json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	ActorID     string         `json:"actor_id,omitempty"`
	ActorName   string         `json:"actor_name,omitempty"`
	ActorEmail  string         `json:"actor_email,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
	Action      string         `json:"action" example:"role.update"`
	TargetID    string         `json:"target_id,omitempty"`
	TargetLabel string         `json:"target_label,omitempty"`
	Details     map[string]any `json:"details"`
}

func parseTime(c fiber.Ctx, key string) (*time.Time, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, apperror.New(apperror.CodeInvalidInput, key+" must be an RFC 3339 time").WithField(key)
	}
	return &t, nil
}

// @Summary List the audit log, newest first
// @Tags Audit
// @Produce json
// @Param page query int false "Page" default(1)
// @Param limit query int false "Items per page, up to 100" default(20)
// @Param action query string false "Action prefix, such as role. or user.disable"
// @Param user_id query string false "The actor or the target"
// @Param from query string false "From this time on (RFC 3339)"
// @Param to query string false "Before this time (RFC 3339)"
// @Security BearerAuth
// @Success 200 {object} object{items=[]recordDTO,pagination=mapper.PaginationDTO}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/audit [get]
func (h *Handler) List(c fiber.Ctx) error {
	filter := domain.AuditFilter{
		Page:   domain.NewPage(fiber.Query[int](c, "page"), fiber.Query[int](c, "limit")),
		Action: c.Query("action"),
		UserID: c.Query("user_id"),
	}
	var err error
	if filter.From, err = parseTime(c, "from"); err != nil {
		return mapper.WriteError(c, err)
	}
	if filter.To, err = parseTime(c, "to"); err != nil {
		return mapper.WriteError(c, err)
	}
	records, total, err := h.svc.List(c.Context(), filter)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]recordDTO, 0, len(records))
	for _, r := range records {
		out = append(out, recordDTO(r))
	}
	return c.JSON(fiber.Map{"items": out, "pagination": mapper.Pagination(filter.Page, total)})
}
