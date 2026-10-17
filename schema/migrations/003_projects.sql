-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}projects (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  org_id              UUID NOT NULL REFERENCES {{.Schema}}.{{.TablePrefix}}orgs (id) ON DELETE CASCADE,
  slug                TEXT NOT NULL,
  name                TEXT NOT NULL,
  created_by          UUID,
  created_at          TIMESTAMPTZ NOT NULL,
  updated_at          TIMESTAMPTZ NOT NULL,
  issuer_handle           TEXT NOT NULL,
  issuer_handle_locked_at TIMESTAMPTZ,
  display_name            TEXT NOT NULL DEFAULT '',
  primary_color           TEXT NOT NULL DEFAULT '',
  support_email           TEXT NOT NULL DEFAULT '',
  terms_url               TEXT NOT NULL DEFAULT '',
  privacy_url             TEXT NOT NULL DEFAULT '',
  terms_version           INTEGER NOT NULL DEFAULT 0,
  terms_published_at      TIMESTAMPTZ,
  signin_email_link       BOOLEAN NOT NULL DEFAULT true,
  UNIQUE (org_id, slug),
  CONSTRAINT {{.TablePrefix}}projects_org_id_id_key UNIQUE (org_id, id),
  CONSTRAINT {{.TablePrefix}}projects_issuer_handle_key UNIQUE (issuer_handle),
  CONSTRAINT {{.TablePrefix}}projects_issuer_handle_format CHECK (
    length(issuer_handle) BETWEEN 3 AND 40 AND issuer_handle ~ '^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$'
  ),
  CONSTRAINT {{.TablePrefix}}projects_primary_color_format CHECK (primary_color = '' OR primary_color ~ '^#[0-9a-fA-F]{6}$'),
  CONSTRAINT {{.TablePrefix}}projects_terms_version_nonnegative CHECK (terms_version >= 0)
);

CREATE INDEX {{.TablePrefix}}projects_org_id_idx ON {{.Schema}}.{{.TablePrefix}}projects (org_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}projects FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}projects_tenant ON {{.Schema}}.{{.TablePrefix}}projects
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}project_logos (
  project_id    UUID PRIMARY KEY,
  org_id        UUID NOT NULL,
  content_type  TEXT NOT NULL CHECK (content_type IN ('image/png', 'image/jpeg', 'image/webp')),
  content       BYTEA NOT NULL CHECK (length(content) BETWEEN 1 AND 262144),
  sha256        TEXT NOT NULL,
  updated_at    TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}project_logos_org_project_fkey FOREIGN KEY (org_id, project_id) REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE
);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}project_logos ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}project_logos FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}project_logos_tenant ON {{.Schema}}.{{.TablePrefix}}project_logos
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}project_logos;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}projects;

-- +goose StatementEnd
