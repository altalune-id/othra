-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}memberships (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  org_id              UUID NOT NULL REFERENCES {{.Schema}}.{{.TablePrefix}}orgs (id) ON DELETE CASCADE,
  user_id             UUID NOT NULL REFERENCES {{.Schema}}.{{.TablePrefix}}users (id) ON DELETE CASCADE,
  role                TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
  created_at          TIMESTAMPTZ NOT NULL,
  UNIQUE (org_id, user_id)
);

CREATE INDEX {{.TablePrefix}}memberships_user_idx
  ON {{.Schema}}.{{.TablePrefix}}memberships (user_id, created_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}memberships_tenant ON {{.Schema}}.{{.TablePrefix}}memberships
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() OR user_id = {{.Schema}}.{{.TablePrefix}}current_user_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id());

CREATE POLICY {{.TablePrefix}}orgs_tenant ON {{.Schema}}.{{.TablePrefix}}orgs
  FOR ALL
  USING (id = {{.Schema}}.{{.TablePrefix}}current_org_id() OR id IN (SELECT m.org_id FROM {{.Schema}}.{{.TablePrefix}}memberships m WHERE m.user_id = {{.Schema}}.{{.TablePrefix}}current_user_id()))
  WITH CHECK (id = {{.Schema}}.{{.TablePrefix}}current_org_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS {{.TablePrefix}}orgs_tenant ON {{.Schema}}.{{.TablePrefix}}orgs;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}memberships;

-- +goose StatementEnd
