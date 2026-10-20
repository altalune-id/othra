package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/tenant"
)

const setScope = `SELECT set_config('app.current_org_id', $1, true),
       set_config('app.current_project_id', $2, true),
       set_config('app.current_user_id', $3, true)`

const setProject = `SELECT set_config('app.current_project_id', $1, true)`

type txCtxKey struct{}

type unitOfWork struct {
	tx    *sql.Tx
	scope tenant.Context
}

// Transactor runs a use case as one unit of work.
type Transactor interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type poolTransactor struct {
	pool Pool
}

// NewTransactor returns a Transactor that begins its transactions on p.W.
func NewTransactor(p Pool) Transactor {
	return poolTransactor{pool: p}
}

func (t poolTransactor) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return RunInTx(ctx, t.pool, fn)
}

// RunInTx runs fn in the unit of work on ctx, or begins one on pool.W with the tenant scope of ctx, and commits when fn returns nil.
func RunInTx(ctx context.Context, pool Pool, fn func(ctx context.Context) error) error {
	if uow, ok := unitOfWorkFromContext(ctx); ok {
		if err := uow.join(ctx); err != nil {
			return err
		}
		return fn(ctx)
	}
	return own(ctx, pool.W, nil, func(uow *unitOfWork) error {
		return fn(contextWithUnitOfWork(ctx, uow))
	})
}

// InTx runs fn in the unit of work on ctx, or in its own scoped transaction that it commits or rolls back.
func InTx(ctx context.Context, pool Pool, fn func(tx *sql.Tx) error) error {
	if uow, ok := unitOfWorkFromContext(ctx); ok {
		if err := uow.join(ctx); err != nil {
			return err
		}
		return fn(uow.tx)
	}
	return own(ctx, pool.W, nil, func(uow *unitOfWork) error { return fn(uow.tx) })
}

func own(ctx context.Context, handle *sql.DB, opts *sql.TxOptions, fn func(uow *unitOfWork) error) error {
	if handle == nil {
		return errors.New("db: pool handle is nil")
	}
	tx, err := handle.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	scope := tenant.Peek(ctx)
	if _, err := tx.ExecContext(ctx, setScope, scopeValue(scope.OrgID), scopeValue(scope.ProjectID), scopeValue(scope.UserID)); err != nil {
		return fmt.Errorf("db: set tenant scope: %w", err)
	}
	if err := fn(&unitOfWork{tx: tx, scope: scope}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

// SECURITY: a join keeps the org and makes the transaction's project scope exactly the joining context's, cleared when it names none.
func (u *unitOfWork) join(ctx context.Context) error {
	tc := tenant.Peek(ctx)
	if tc.OrgID != uuid.Nil && u.scope.OrgID != uuid.Nil && tc.OrgID != u.scope.OrgID {
		return ErrTenantMismatch
	}
	if tc.ProjectID == u.scope.ProjectID {
		return nil
	}
	if _, err := u.tx.ExecContext(ctx, setProject, scopeValue(tc.ProjectID)); err != nil {
		return fmt.Errorf("db: set project scope: %w", err)
	}
	u.scope.ProjectID = tc.ProjectID
	return nil
}

func scopeValue(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

// CurrentTx returns the transaction bound to ctx, or (nil, false) if none is active.
func CurrentTx(ctx context.Context) (*sql.Tx, bool) {
	uow, ok := unitOfWorkFromContext(ctx)
	if !ok {
		return nil, false
	}
	return uow.tx, true
}

// ContextWithTx returns a context carrying tx as the active unit of work, with no tenant scope.
func ContextWithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return contextWithUnitOfWork(ctx, &unitOfWork{tx: tx})
}

func contextWithUnitOfWork(ctx context.Context, uow *unitOfWork) context.Context {
	return context.WithValue(ctx, txCtxKey{}, uow)
}

func unitOfWorkFromContext(ctx context.Context) (*unitOfWork, bool) {
	uow, ok := ctx.Value(txCtxKey{}).(*unitOfWork)
	return uow, ok
}
