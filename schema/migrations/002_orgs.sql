-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}orgs (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  slug                TEXT NOT NULL UNIQUE,
  name                TEXT NOT NULL,
  created_by          UUID,
  created_at          TIMESTAMPTZ NOT NULL,
  updated_at          TIMESTAMPTZ NOT NULL,
  is_system           BOOLEAN NOT NULL DEFAULT false,
  CONSTRAINT {{.TablePrefix}}orgs_created_by_fkey FOREIGN KEY (created_by) REFERENCES {{.Schema}}.{{.TablePrefix}}users (id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX {{.TablePrefix}}orgs_one_system
  ON {{.Schema}}.{{.TablePrefix}}orgs (is_system)
  WHERE is_system;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}orgs;

-- +goose StatementEnd
