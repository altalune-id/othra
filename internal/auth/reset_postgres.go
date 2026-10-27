package auth

import (
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/schema/entity"
)

type resetPostgres struct {
	pool   db.Pool
	resets *entity.PasswordResets
}

// NewResetStore builds the Postgres ResetStore over pool.
func NewResetStore(pool db.Pool, schema, tablePrefix string) ResetStore {
	return &resetPostgres{pool: pool, resets: entity.NewPasswordResets(schema, tablePrefix)}
}
