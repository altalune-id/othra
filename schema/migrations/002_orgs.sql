-- +goose Up
-- +goose StatementBegin

-- SECURITY: a transaction-local set_config resets to '' rather than NULL when the transaction ends, so a pooled
-- connection that once carried a scope would make ''::uuid throw 22P02; NULLIF turns "no scope" into "no tenant rows".
CREATE FUNCTION {{.Schema}}.{{.TablePrefix}}current_org_id()
RETURNS uuid
LANGUAGE sql STABLE
SET search_path = {{.Schema}}, pg_catalog, pg_temp
AS $$
  SELECT NULLIF(current_setting('app.current_org_id', true), '')::uuid;
$$;

CREATE FUNCTION {{.Schema}}.{{.TablePrefix}}current_project_id()
RETURNS uuid
LANGUAGE sql STABLE
SET search_path = {{.Schema}}, pg_catalog, pg_temp
AS $$
  SELECT NULLIF(current_setting('app.current_project_id', true), '')::uuid;
$$;

CREATE FUNCTION {{.Schema}}.{{.TablePrefix}}current_user_id()
RETURNS uuid
LANGUAGE sql STABLE
SET search_path = {{.Schema}}, pg_catalog, pg_temp
AS $$
  SELECT NULLIF(current_setting('app.current_user_id', true), '')::uuid;
$$;

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

-- NOTE: the orgs policy reads memberships, so it is created in 007_memberships.sql.
ALTER TABLE {{.Schema}}.{{.TablePrefix}}orgs ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}orgs FORCE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}orgs;
DROP FUNCTION IF EXISTS {{.Schema}}.{{.TablePrefix}}current_user_id();
DROP FUNCTION IF EXISTS {{.Schema}}.{{.TablePrefix}}current_project_id();
DROP FUNCTION IF EXISTS {{.Schema}}.{{.TablePrefix}}current_org_id();

-- +goose StatementEnd
