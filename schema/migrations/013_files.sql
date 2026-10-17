-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}files (
  id             UUID PRIMARY KEY,
  public_id      TEXT NOT NULL UNIQUE,
  org_id         UUID NOT NULL REFERENCES {{.Schema}}.{{.TablePrefix}}orgs (id) ON DELETE CASCADE,
  owner_kind     TEXT NOT NULL,
  owner_id       UUID NOT NULL,
  purpose        TEXT NOT NULL,
  visibility     TEXT NOT NULL,
  object_key     TEXT NOT NULL,
  content_type   TEXT NOT NULL,
  size_bytes     BIGINT NOT NULL,
  original_name  TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL,
  completed_at   TIMESTAMPTZ,
  expires_at     TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}files_org_id_id_key UNIQUE (org_id, id),
  CONSTRAINT {{.TablePrefix}}files_object_key_key UNIQUE (object_key),
  CONSTRAINT {{.TablePrefix}}files_owner_kind_check CHECK (owner_kind IN ('orgs', 'projects', 'users')),
  CONSTRAINT {{.TablePrefix}}files_visibility_check CHECK (visibility IN ('public', 'private')),
  CONSTRAINT {{.TablePrefix}}files_status_check CHECK (status IN ('pending', 'ready', 'deleting')),
  CONSTRAINT {{.TablePrefix}}files_size_bytes_check CHECK (size_bytes > 0)
);

CREATE INDEX {{.TablePrefix}}files_status_expires_at_idx ON {{.Schema}}.{{.TablePrefix}}files (status, expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}files ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}files FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}files_tenant ON {{.Schema}}.{{.TablePrefix}}files
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id());

ALTER TABLE {{.Schema}}.{{.TablePrefix}}projects ADD COLUMN logo_file_id UUID;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}projects
  ADD CONSTRAINT {{.TablePrefix}}projects_logo_file_fkey FOREIGN KEY (org_id, logo_file_id)
  REFERENCES {{.Schema}}.{{.TablePrefix}}files (org_id, id) ON DELETE SET NULL (logo_file_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}projects DROP COLUMN IF EXISTS logo_file_id;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}files;

-- +goose StatementEnd
