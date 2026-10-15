package org

import (
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool db.Pool
	orgs *entity.Orgs
}

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	return &postgresStore{pool: pool, orgs: entity.NewOrgs(schema, tablePrefix)}
}
