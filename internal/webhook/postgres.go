package webhook

import (
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/schema/entity"
)

type postgresStore struct {
	pool      db.Pool
	endpoints *entity.WebhookEndpoints
	attempts  *entity.WebhookDeliveries
}

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	return &postgresStore{
		pool:      pool,
		endpoints: entity.NewWebhookEndpoints(schema, tablePrefix),
		attempts:  entity.NewWebhookDeliveries(schema, tablePrefix),
	}
}
