// Package emailaddr normalizes email addresses the same way for admins and end users.
package emailaddr

import (
	"net/mail"
	"strings"
)

// MaxLen is the longest address Normalize accepts.
const MaxLen = 254

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
