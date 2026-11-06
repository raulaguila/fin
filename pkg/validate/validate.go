// Package validate holds input checks shared by use cases.
package validate

import (
	"github.com/google/uuid"

	"github.com/raulaguila/authenticity/pkg/apperror"
)

// ID refuses anything but a UUID before it reaches a uuid column.
func ID(id string) error {
	if uuid.Validate(id) != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid id").WithField("id")
	}
	return nil
}
