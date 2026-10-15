# Add a store method

Use this when a service needs a database verb its `Store` does not have. The `Store` is the driven port the domain owns, in `<domain>/store.go`. Othra has one driver, Postgres, built with go-jet. A new verb is three edits plus tests: the interface, the Postgres code and the fake. For the service side, see [service-method.md](service-method.md). For a new table or column, see [migration.md](migration.md).

## Plan

- [ ] **Reader or Writer?** `Store` embeds `Reader` (reads, may use a replica) and `Writer` (writes). Put the verb on the right one.
- [ ] **Name.** Verbs used today: `ByID`, `ByPublicID`, `BySlug`, `List`, `Save`, `Delete`, `Add`, `Find`. Reuse one before you invent one.
- [ ] **Keys.** It takes UUIDs. `ByPublicID` exists for the edge only ([public-ids.md](public-ids.md)).
- [ ] **Scope.** Which predicate confines the row? An explicit argument (`List(ctx, orgID)`), or the tenant on `ctx` (`tenant.From(ctx)`, as the webhook store does).
- [ ] **Failures.** No row → the domain's `*NotFoundError`. Unique violation → `*AlreadyExistsError`. Nothing else is typed.
- [ ] **Order.** A list needs a stable order. Use a total order: a timestamp and then `id`. The `apikey`, `member` and `webhook` readers do this; the `project` and `org` `List` readers order by `created_at` only, so don't copy those.
- [ ] **Text search.** No store does a `LIKE`/`ILIKE` or full-text search yet, so there is no reference for case, escaping or indexes. Ask before adding the first one.
- [ ] **Transaction.** The store never begins one. It joins the unit of work on `ctx`, if any.

## Build

1. **Interface** in `internal/<domain>/store.go`, `ctx` first:

   ```go
   type Reader interface {
       ByID(ctx context.Context, id uuid.UUID) (*Project, error)
       ByPublicID(ctx context.Context, publicID string) (*Project, error)
       BySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error)
       List(ctx context.Context, orgID uuid.UUID) ([]*Project, error)
   }
   ```

2. **Postgres.** Reads in `pgreader.go`, writes in `pgwriter.go`, both on `postgresStore` (`postgres.go`). Build with the go-jet builder and the `schema/entity` binding. Run reads on `db.Reader(ctx, s.pool)` and writes on `db.Writer(ctx, s.pool)`; both return the transaction on `ctx` when there is one:

   ```go
   // ByPublicID loads a project by its public id.
   func (s *postgresStore) ByPublicID(ctx context.Context, publicID string) (*Project, error) {
       stmt := postgres.SELECT(s.projects.AllColumns).
           FROM(s.projects).
           WHERE(s.projects.PublicID.EQ(postgres.String(publicID))).
           LIMIT(1)
       var row projectRow
       if err := stmt.QueryContext(ctx, db.Reader(ctx, s.pool), &row); err != nil {
           if db.IsNoRows(err) {
               return nil, &NotFoundError{ID: publicID}
           }
           return nil, fmt.Errorf("project.postgres: ByPublicID: %w", err)
       }
       return row.toProject(), nil
   }
   ```

3. **Writes.** An upsert is `INSERT ... ON_CONFLICT(id).DO_UPDATE(...)` that sets only mutable columns. Guard the conflict clause by org and treat zero rows as not found, as `webhook.postgres.Save` does:

   ```go
   ON_CONFLICT(s.endpoints.ID).
   // SECURITY: the conflict clause is guarded by org, or an upsert carrying another tenant's row id would rewrite that row.
   DO_UPDATE(postgres.SET(...).WHERE(s.endpoints.OrgID.EQ(postgres.UUID(tc.OrgID))))
   ```

   Map a unique violation with `db.IsUniqueViolation(err)` to `*AlreadyExistsError` (also `db.IsForeignKeyViolation`, and `db.ViolatedConstraint` for a named constraint); a domain never imports the driver. A `Delete` checks `RowsAffected() == 0` → `*NotFoundError` (`webhook.postgres.Delete`). A SQL `NULL` uses the helpers in `schema/entity/null.go` (`entity.NullUUID()`).

4. **Several statements in one verb?** Wrap them in `db.InTx(ctx, s.pool, func(tx *sql.Tx) error {...})`. It joins the outer transaction or owns one.
5. **Raw SQL** only where go-jet has no builder. Mark it with a `NOTE:` naming what jet could not express (see `apikey.NewStore`). Bind values as parameters; never concatenate user input.
6. **Fake** in `internal/testutil/fakes/<domain>.go`. `var _ project.Store = (*ProjectStore)(nil)` turns a missing method into a compile error. Copy the real predicate; never filter more strictly than the store does.
7. **Unit test** in `postgres_test.go` (package `<domain>`, `go-sqlmock`): the row mapping, not-found and the error paths (`TestStore_QueryErrors`).
8. **Integration test** in `postgres_integration_test.go` (`//go:build integration`, `pgtest.Migrated(t)`), named `TestPostgres_<Behaviour>`: round trip, not found, constraints, order and scope, and that the verb joins a unit of work (`TestPostgres_KeyUnitOfWork` in `apikey`).
9. **Gates.** `make check`, `make comment-check`, `make test-integration`, `make cover` (at least 85% for the package).

## Review

- [ ] The store never begins a transaction. It uses `db.Reader`, `db.Writer` or `db.InTx` (`TestOnlyKernelDBBeginsTransactions` fails otherwise).
- [ ] No driver error leaves the store untyped when it is expected: no row → `*NotFoundError`, unique violation → `*AlreadyExistsError`. Other errors are wrapped with the domain and the verb, like `"project.postgres: ByID: %w"`.
- [ ] The query names its scope (org, project or tenant from `ctx`). An upsert's conflict clause is guarded by org.
- [ ] A list orders by a total order.
- [ ] Raw SQL has a `NOTE:` and binds every value.
- [ ] The fake mirrors the real predicate and write contract.
- [ ] A Postgres integration test exists for every new verb.
- [ ] A new column is in the migration, the `schema/entity` binding and `AllColumns`, and in `schema/README.md`.

## Reference code

- Interfaces: [internal/project/store.go](../../internal/project/store.go) (`Store`, `Reader`, `Writer`)
- Reads: [internal/project/pgreader.go](../../internal/project/pgreader.go) (`ByID`, `ByPublicID`, `BySlug`, `List`)
- Writes: [internal/project/pgwriter.go](../../internal/project/pgwriter.go) (`Save`, `createdByExpr`), [internal/webhook/pgwriter.go](../../internal/webhook/pgwriter.go) (`Delete`)
- Guarded upsert and tenant on `ctx`: [internal/webhook/pgwriter.go](../../internal/webhook/pgwriter.go) (`Save`), [internal/webhook/pgreader.go](../../internal/webhook/pgreader.go) (`ByPublicID`)
- Total order: [internal/apikey/pgreader.go](../../internal/apikey/pgreader.go), [internal/org/member/pgreader.go](../../internal/org/member/pgreader.go)
- Unit of work: [internal/kernel/db/uow.go](../../internal/kernel/db/uow.go) (`Reader`, `Writer`, `InTx`), [internal/kernel/db/errors.go](../../internal/kernel/db/errors.go) (`IsNoRows`, `IsUniqueViolation`, `IsForeignKeyViolation`, `ViolatedConstraint`)
- Guard test: [internal/kernel/db/guard_test.go](../../internal/kernel/db/guard_test.go) (`TestOnlyKernelDBBeginsTransactions`)
- Binding: [schema/entity/projects.go](../../schema/entity/projects.go) (`NewProjects`)
- Fake: [internal/testutil/fakes/project.go](../../internal/testutil/fakes/project.go) (`ProjectStore`)
- Tests: [internal/project/postgres_test.go](../../internal/project/postgres_test.go), [internal/project/postgres_integration_test.go](../../internal/project/postgres_integration_test.go)
