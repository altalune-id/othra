-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}api_keys (
  id                  UUID PRIMARY KEY,
  org_id              UUID NOT NULL REFERENCES {{.Schema}}.{{.TablePrefix}}orgs (id) ON DELETE CASCADE,
  project_id          UUID,
  kind                TEXT NOT NULL,
  all_projects        BOOLEAN NOT NULL DEFAULT false,
  name                TEXT NOT NULL,
  secret_hash         BYTEA NOT NULL,
  secret_hint         TEXT NOT NULL,
  scopes              TEXT[] NOT NULL DEFAULT '{}',
  resource_ids        UUID[] NOT NULL DEFAULT '{}',
  created_by          UUID,
  created_at          TIMESTAMPTZ NOT NULL,
  expires_at          TIMESTAMPTZ,
  revoked_at          TIMESTAMPTZ,
  last_used_at        TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}api_keys_org_id_id_key UNIQUE (org_id, id),
  CONSTRAINT {{.TablePrefix}}api_keys_kind_shape CHECK (
    (kind = 'project' AND project_id IS NOT NULL AND NOT all_projects)
    OR (kind IN ('org', 'personal') AND project_id IS NULL)
  ),
  CONSTRAINT {{.TablePrefix}}api_keys_org_project_fkey FOREIGN KEY (org_id, project_id) REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}api_keys_created_by_fkey FOREIGN KEY (created_by) REFERENCES {{.Schema}}.{{.TablePrefix}}users (id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX {{.TablePrefix}}api_keys_secret_hash_key
  ON {{.Schema}}.{{.TablePrefix}}api_keys (secret_hash);

CREATE UNIQUE INDEX {{.TablePrefix}}api_keys_live_hint_key
  ON {{.Schema}}.{{.TablePrefix}}api_keys (org_id, secret_hint)
  WHERE revoked_at IS NULL;

CREATE INDEX {{.TablePrefix}}api_keys_org_idx
  ON {{.Schema}}.{{.TablePrefix}}api_keys (org_id, kind, created_at DESC);

CREATE INDEX {{.TablePrefix}}api_keys_personal_idx
  ON {{.Schema}}.{{.TablePrefix}}api_keys (org_id, created_by, created_at DESC)
  WHERE kind = 'personal';

CREATE TABLE {{.Schema}}.{{.TablePrefix}}api_key_projects (
  org_id      UUID NOT NULL REFERENCES {{.Schema}}.{{.TablePrefix}}orgs (id) ON DELETE CASCADE,
  key_id      UUID NOT NULL,
  project_id  UUID NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (key_id, project_id),
  FOREIGN KEY (org_id, key_id) REFERENCES {{.Schema}}.{{.TablePrefix}}api_keys (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, project_id) REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE
);

CREATE INDEX {{.TablePrefix}}api_key_projects_org_project_idx
  ON {{.Schema}}.{{.TablePrefix}}api_key_projects (org_id, project_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}api_key_projects;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}api_keys;

-- +goose StatementEnd
