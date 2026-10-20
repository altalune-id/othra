package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/kernel/tenant"
)

func expectProject(mock sqlmock.Sqlmock, project uuid.UUID) *sqlmock.ExpectedExec {
	return mock.ExpectExec(`SELECT set_config\('app\.current_project_id', \$1, true\)`).
		WithArgs(scopeArg(project)).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func joins() map[string]func(ctx context.Context, pool Pool) error {
	return map[string]func(ctx context.Context, pool Pool) error{
		"RunInTx": func(ctx context.Context, pool Pool) error {
			return RunInTx(ctx, pool, func(context.Context) error { return nil })
		},
		"InTx": func(ctx context.Context, pool Pool) error { return InTx(ctx, pool, func(*sql.Tx) error { return nil }) },
		"Read": func(ctx context.Context, pool Pool) error { return Read(ctx, pool, func(qrm.DB) error { return nil }) },
	}
}

// SECURITY: the transaction's project scope follows the joining context exactly, and is set only when it changes.
func TestJoin_TheProjectScopeFollowsTheJoiningContext(t *testing.T) {
	for name, join := range joins() {
		t.Run(name, func(t *testing.T) {
			pool, mock := mockPool(t)
			org, a, b := uuid.New(), uuid.New(), uuid.New()
			outer := tenant.Context{OrgID: org}
			mock.ExpectBegin()
			expectScope(mock, outer)
			expectProject(mock, a)
			expectProject(mock, b)
			expectProject(mock, uuid.Nil)
			mock.ExpectCommit()

			require.NoError(t, RunInTx(tenant.Into(context.Background(), outer), pool, func(ctx context.Context) error {
				for _, step := range []context.Context{
					tenant.WithProject(ctx, a), tenant.WithProject(ctx, a),
					tenant.WithProject(ctx, b),
					ctx, ctx,
				} {
					if err := join(step, pool); err != nil {
						return err
					}
				}
				return nil
			}))
			assert.NoError(t, mock.ExpectationsWereMet(), "A, then B, then none; repeats set nothing")
		})
	}
}

func TestJoin_KeepsTheOutermostProjectWhenTheContextNamesIt(t *testing.T) {
	pool, mock := mockPool(t)
	ctx, tc := scoped()
	mock.ExpectBegin()
	expectScope(mock, tc)
	mock.ExpectCommit()

	require.NoError(t, RunInTx(ctx, pool, func(ctx context.Context) error {
		return Read(tenant.WithOrg(ctx, tc.OrgID), pool, func(qrm.DB) error { return nil })
	}))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestJoin_ScopeErrorFailsTheJoin(t *testing.T) {
	pool, mock := mockPool(t)
	org, project := uuid.New(), uuid.New()
	mock.ExpectBegin()
	expectScope(mock, tenant.Context{OrgID: org})
	expectProject(mock, project).WillReturnError(errors.New("no"))
	mock.ExpectRollback()

	err := RunInTx(tenant.Into(context.Background(), tenant.Context{OrgID: org}), pool, func(ctx context.Context) error {
		return InTx(tenant.WithProject(ctx, project), pool, func(*sql.Tx) error { return nil })
	})
	require.ErrorContains(t, err, "db: set project scope")
	assert.NoError(t, mock.ExpectationsWereMet())
}
