package shared

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/pkg/apperror"
)

var (
	ErrDuplicatedKey           = errors.New("duplicated key not allowed")
	ErrForeignKeyViolated      = errors.New("violates foreign key constraint")
	ErrUndefinedColumn         = errors.New("undefined column or parameter name")
	ErrDatabaseAlreadyExists   = errors.New("database already exists")
	ErrCheckConstraintViolated = errors.New("check constraint violated")
)

// handlerError converts PostgreSQL errors to domain errors
func handlerError(err error) error {
	if pgError, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgError.SQLState() {
		case "23505":
			return ErrDuplicatedKey
		case "23503":
			return ErrForeignKeyViolated
		case "42703":
			return ErrUndefinedColumn
		case "42P04":
			return ErrDatabaseAlreadyExists
		case "23514":
			return ErrCheckConstraintViolated
		default:
			// Unhandled PG error code — return original error
		}
	}

	return err
}

// WrapError translates infrastructure-level errors (gorm, pgerror) into
// domain-level apperror types. If the error is not a recognized infrastructure
// error, it is wrapped as an internal database error.
func WrapError(err error, resource string) error {
	if err == nil {
		return nil
	}

	// Already an apperror — pass through
	if _, ok := errors.AsType[*apperror.Error](err); ok {
		return err
	}

	// PostgreSQL-specific errors
	pgErr := handlerError(err)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return apperror.New(apperror.CodeTimedOut, resource+" query timed out").WithCause(err)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperror.New(apperror.CodeNotFound, resource+" not found").WithDetails("resource", resource)
	case errors.Is(pgErr, ErrDuplicatedKey):
		return apperror.New(apperror.CodeAlreadyExists, resource+" already exists").WithDetails("resource", resource)
	case errors.Is(pgErr, ErrForeignKeyViolated):
		return apperror.New(apperror.CodeResourceInUse, resource+" is in use").WithDetails("resource", resource)
	case errors.Is(pgErr, ErrUndefinedColumn):
		return apperror.New(apperror.CodeInvalidInput, "undefined column or parameter name").WithField("column")
	case errors.Is(pgErr, ErrCheckConstraintViolated):
		return apperror.New(apperror.CodeValidationFailed, "constraint violation")
	}

	// Unknown DB error — wrap with cause for debugging
	return apperror.New(apperror.CodeDatabaseError, resource+" database error").WithCause(err)
}
