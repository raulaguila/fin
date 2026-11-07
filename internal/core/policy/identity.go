// Package policy contains identity rules shared by application entry points.
package policy

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/raulaguila/authenticity/pkg/apperror"
)

const (
	maxEmailBytes    = 254
	minPasswordBytes = 8
	maxPasswordBytes = 128
)

// ValidateIdentity returns the name and email a new account stores.
func ValidateIdentity(rawName, rawEmail string) (name, email string, err error) {
	if name, err = ValidateName(rawName); err != nil {
		return "", "", err
	}
	email = NormalizeEmail(rawEmail)
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email || email != strings.ToLower(strings.TrimSpace(rawEmail)) {
		return "", "", apperror.New(apperror.CodeInvalidInput, "invalid email").WithField("email")
	}
	return name, email, nil
}

// ValidateName returns the name an account stores.
func ValidateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(name); n < 2 || n > 80 || strings.ContainsFunc(name, unicode.IsControl) {
		return "", apperror.New(apperror.CodeInvalidInput, "name must be 2-80 characters").WithField("name")
	}
	return name, nil
}

// NormalizeEmail makes an address safe to compare and to store: Postgres refuses
// NUL, and the email index refuses very long values, so a stored attempt never fails.
func NormalizeEmail(raw string) string {
	email := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(raw, "\x00", "")))
	if len(email) > maxEmailBytes {
		email = strings.ToValidUTF8(email[:maxEmailBytes], "")
	}
	return email
}

// ValidatePassword applies the password policy.
func ValidatePassword(plain string) error {
	if len(plain) < minPasswordBytes {
		return apperror.New(apperror.CodeInvalidInput, "password too short").WithField("password")
	}
	if len(plain) > maxPasswordBytes {
		return apperror.New(apperror.CodeInvalidInput, "password too long").WithField("password")
	}
	return nil
}
