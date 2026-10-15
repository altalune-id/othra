package session_test

import (
	"testing"

	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/session/sessiontest"
)

func TestMemoryStore_Contract(t *testing.T) {
	sessiontest.RunStoreContract(t, func(*testing.T) session.Store { return session.NewMemoryStore() })
}
