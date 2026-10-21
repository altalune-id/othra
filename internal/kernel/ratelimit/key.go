package ratelimit

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
)

// Key namespaces subject under the rule's name, so every store keys the same way.
func Key(r Rule, subject string) string { return r.Name + ":" + subject }

// ProjectSubject scopes subject to one project, so a limit in one project never locks the same IP or email out of another.
func ProjectSubject(projectID uuid.UUID, subject string) string {
	return "project:" + projectID.String() + ":" + subject
}

// IPSubject is the subject for a client IP.
func IPSubject(ip string) string { return "ip:" + ip }

// EmailSubject is the subject for an email: a SHA-256 of the trimmed, lowercased address, so keys hold no email.
func EmailSubject(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return "email:" + hex.EncodeToString(sum[:])
}
