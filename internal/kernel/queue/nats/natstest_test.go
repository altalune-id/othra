package nats

import (
	"testing"

	"github.com/nats-io/nats-server/v2/server"

	"github.com/altalune-id/othra/internal/testutil/natstest"
)

func startNATS(t *testing.T) string {
	t.Helper()
	return natstest.Start(t).ClientURL()
}

func startNATSServer(t *testing.T) *server.Server {
	t.Helper()
	return natstest.Start(t)
}
