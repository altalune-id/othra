package session_test

import (
	"testing"

	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/kernel/session/sessiontest"
)

func TestMemoryStore_Contract(t *testing.T) {
	sessiontest.RunStoreContract(t, func(*testing.T) session.Store { return session.NewMemoryStore() })
}
