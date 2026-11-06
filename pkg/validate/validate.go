// Package validate holds input checks shared by use cases.
package validate

import (
	"strings"
	"unicode"
	"unicode/utf8"

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

// Search refuses a search term that cannot be a piece of stored text.
func Search(s string, max int) error {
	if len(s) > max || !utf8.ValidString(s) || strings.ContainsFunc(s, unicode.IsControl) {
		return apperror.New(apperror.CodeInvalidInput, "invalid search").WithField("search")
	}
	return nil
}

// Text cuts s to at most max bytes of what a Postgres text column takes: valid
// UTF-8 without NUL. For untrusted text kept as is, such as a header or a remote
// error; a refused write would otherwise turn it into a failure of its own.
func Text(s string, max int) string {
	if len(s) > max {
		s = s[:max]
	}
	return strings.ReplaceAll(strings.ToValidUTF8(s, ""), "\x00", "")
}
