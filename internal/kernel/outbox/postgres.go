package outbox

import (
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/schema/entity"
)

type postgresStore struct {
	pool    db.Pool
	entries *entity.OutboxEntries
}

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	return &postgresStore{pool: pool, entries: entity.NewOutboxEntries(schema, tablePrefix)}
}
