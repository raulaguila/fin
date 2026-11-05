package mapper

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/pkg/apperror"
)

// LocalCause holds the error behind a 5xx answer, for the request log. Clients
// never see it.
const LocalCause = "http.error_cause"

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Field   string         `json:"field,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// WriteError maps apperror (or generic error) to an HTTP JSON response.
func WriteError(c fiber.Ctx, err error) error {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		appErr = apperror.New(apperror.CodeInternal, "internal error").WithCause(err)
	}

	status := statusFor(appErr.Code)
	if status >= fiber.StatusInternalServerError {
		c.Locals(LocalCause, err)
	}
	return c.Status(status).JSON(errorBody{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Field:   appErr.Field,
		Details: appErr.Details,
	})
}

func statusFor(code apperror.Code) int {
	switch code {
	case apperror.CodeUnauthorized, apperror.CodeInvalidCredentials, apperror.CodeTokenExpired:
		return fiber.StatusUnauthorized
	case apperror.CodeForbidden, apperror.CodeDisabledUser,
		apperror.CodeAccountPending, apperror.CodeAccountExpired,
		apperror.CodeAccountSuspended, apperror.CodeAccountDisabled:
		return fiber.StatusForbidden
	case apperror.CodeNotFound:
		return fiber.StatusNotFound
	case apperror.CodeAlreadyExists, apperror.CodeConflict, apperror.CodeResourceInUse:
		return fiber.StatusConflict
	case apperror.CodeInvalidInput, apperror.CodeValidationFailed:
		return fiber.StatusBadRequest
	case apperror.CodeTooManyRequests:
		return fiber.StatusTooManyRequests
	case apperror.CodeTimedOut:
		return fiber.StatusGatewayTimeout
	default:
		return fiber.StatusInternalServerError
	}
}
