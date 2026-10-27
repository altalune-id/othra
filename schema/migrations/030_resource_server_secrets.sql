-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}resource_server_secrets (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  org_id              UUID NOT NULL,
  project_id          UUID NOT NULL,
  resource_server_id  UUID NOT NULL,
  -- SECURITY: "sha256$" and the hex SHA-256 of the secret; never the secret itself.
  secret_hash         TEXT NOT NULL,
  hint                TEXT NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL,
  expires_at          TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}resource_server_secrets_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}resource_server_secrets_hash_key UNIQUE (secret_hash),
  CONSTRAINT {{.TablePrefix}}resource_server_secrets_hash_format CHECK (secret_hash ~ '^sha256\$[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}resource_server_secrets_hint_length CHECK (char_length(hint) = 4)
);

CREATE UNIQUE INDEX {{.TablePrefix}}resource_server_secrets_one_current ON {{.Schema}}.{{.TablePrefix}}resource_server_secrets (resource_server_id)
  WHERE expires_at IS NULL;
CREATE INDEX {{.TablePrefix}}resource_server_secrets_server_idx ON {{.Schema}}.{{.TablePrefix}}resource_server_secrets (resource_server_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_server_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_server_secrets FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}resource_server_secrets_tenant ON {{.Schema}}.{{.TablePrefix}}resource_server_secrets
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}resource_server_secrets;

-- +goose StatementEnd
