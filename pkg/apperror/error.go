// Package apperror provides centralized error handling for the application.
// These errors are domain-agnostic and can be mapped to different protocols
// (HTTP, gRPC, CLI) by their respective adapters.
package apperror

import (
	"errors"
	"fmt"
)

// Code represents an application error code
type Code string

const (
	// Authentication & Authorization
	CodeUnauthorized       Code = "UNAUTHORIZED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeInvalidCredentials Code = "INVALID_CREDENTIALS"
	CodeDisabledUser       Code = "DISABLED_USER"
	CodeAccountPending     Code = "ACCOUNT_PENDING"
	CodeAccountExpired     Code = "ACCOUNT_EXPIRED"
	CodeAccountSuspended   Code = "ACCOUNT_SUSPENDED"
	CodeAccountDisabled    Code = "ACCOUNT_DISABLED"
	CodeAccountLocked      Code = "ACCOUNT_LOCKED"
	CodeTokenExpired       Code = "TOKEN_EXPIRED"

	// Resource errors
	CodeNotFound      Code = "NOT_FOUND"
	CodeAlreadyExists Code = "ALREADY_EXISTS"
	CodeConflict      Code = "CONFLICT"
	CodeResourceInUse Code = "RESOURCE_IN_USE"

	// Validation errors
	CodeInvalidInput     Code = "INVALID_INPUT"
	CodeValidationFailed Code = "VALIDATION_FAILED"
	CodeTooManyRequests  Code = "TOO_MANY_REQUESTS"

	// System errors
	CodeInternal        Code = "INTERNAL_ERROR"
	CodeDatabaseError   Code = "DATABASE_ERROR"
	CodeExternalService Code = "EXTERNAL_SERVICE_ERROR"
	CodeTimedOut        Code = "TIMED_OUT_ERROR"
)

// Error represents an application error with rich context
type Error struct {
	Code    Code
	Message string
	Field   string
	Details map[string]any
	Cause   error
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Field, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a target
func (e *Error) Is(target error) bool {
	if appErr, ok := errors.AsType[*Error](target); ok {
		return e.Code == appErr.Code
	}
	return false
}

// WithField adds a field name to the error
func (e *Error) WithField(field string) *Error {
	e.Field = field
	return e
}

// WithDetails adds details to the error
func (e *Error) WithDetails(key string, value any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// WithCause wraps an underlying error
func (e *Error) WithCause(err error) *Error {
	e.Cause = err
	return e
}

// New creates a new application error
func New(code Code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// IsCode checks if an error has a specific code
func IsCode(err error, code Code) bool {
	if appErr, ok := errors.AsType[*Error](err); ok {
		return appErr.Code == code
	}
	return false
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return IsCode(err, CodeNotFound)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	return IsCode(err, CodeInvalidInput) || IsCode(err, CodeValidationFailed)
}

// GetCode extracts the error code from an error
func GetCode(err error) Code {
	if appErr, ok := errors.AsType[*Error](err); ok {
		return appErr.Code
	}
	return CodeInternal
}
