// Package emailaddr normalizes email addresses the same way for admins and end users.
package emailaddr

import (
	"errors"
	"net/mail"
	"strings"
)

// MaxLen is the longest address Normalize accepts.
const MaxLen = 254

// ErrInvalid is returned for anything that is not a bare address.
var ErrInvalid = errors.New("emailaddr: invalid email address")

// Normalize trims and lowercases email and rejects anything that is not a bare address of at most MaxLen bytes.
func Normalize(email string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" || len(e) > MaxLen {
		return "", ErrInvalid
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e || addr.Name != "" {
		return "", ErrInvalid
	}
	return e, nil
}
