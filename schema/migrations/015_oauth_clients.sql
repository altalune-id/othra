-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients (
  id                          UUID PRIMARY KEY,
  public_id                   TEXT NOT NULL UNIQUE,
  org_id                      UUID NOT NULL,
  project_id                  UUID NOT NULL,
  name                        TEXT NOT NULL,
  client_type                 TEXT NOT NULL,
  application_type            TEXT NOT NULL,
  token_endpoint_auth_method  TEXT NOT NULL,
  grant_types                 TEXT[] NOT NULL,
  allowed_scopes              TEXT[] NOT NULL,
  redirect_uris               TEXT[] NOT NULL,
  post_logout_redirect_uris   TEXT[] NOT NULL DEFAULT '{}',
  consent_policy              TEXT NOT NULL DEFAULT 'require',
  pkce_required               BOOLEAN NOT NULL DEFAULT true,
  created_at                  TIMESTAMPTZ NOT NULL,
  updated_at                  TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}oauth_clients_org_project_fkey FOREIGN KEY (org_id, project_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}oauth_clients_org_project_id_key UNIQUE (org_id, project_id, id),
  CONSTRAINT {{.TablePrefix}}oauth_clients_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
  CONSTRAINT {{.TablePrefix}}oauth_clients_kind_check CHECK (
    (client_type = 'confidential' AND application_type = 'web' AND token_endpoint_auth_method IN ('client_secret_basic', 'client_secret_post'))
    OR (client_type = 'public' AND application_type = 'native' AND token_endpoint_auth_method = 'none')
  ),
  CONSTRAINT {{.TablePrefix}}oauth_clients_grant_types_check CHECK (
    'authorization_code' = ANY (grant_types) AND grant_types <@ ARRAY['authorization_code', 'refresh_token']::TEXT[]
  ),
  CONSTRAINT {{.TablePrefix}}oauth_clients_pkce_public CHECK (pkce_required OR client_type = 'confidential'),
  CONSTRAINT {{.TablePrefix}}oauth_clients_consent_policy_check CHECK (consent_policy IN ('require', 'implicit')),
  CONSTRAINT {{.TablePrefix}}oauth_clients_redirect_uris_count CHECK (cardinality(redirect_uris) BETWEEN 1 AND 20),
  CONSTRAINT {{.TablePrefix}}oauth_clients_post_logout_uris_check CHECK (
    cardinality(post_logout_redirect_uris) <= 20 AND (application_type = 'web' OR cardinality(post_logout_redirect_uris) = 0)
  )
);

CREATE INDEX {{.TablePrefix}}oauth_clients_project_idx ON {{.Schema}}.{{.TablePrefix}}oauth_clients (project_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}oauth_clients_tenant ON {{.Schema}}.{{.TablePrefix}}oauth_clients
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}oauth_client_secrets (
  id           UUID PRIMARY KEY,
  public_id    TEXT NOT NULL UNIQUE,
  org_id       UUID NOT NULL,
  project_id   UUID NOT NULL,
  client_id    UUID NOT NULL,
  -- SECURITY: "sha256$" and the hex SHA-256 of the secret; never the secret itself.
  secret_hash  TEXT NOT NULL,
  hint         TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL,
  expires_at   TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}oauth_client_secrets_client_fkey FOREIGN KEY (org_id, project_id, client_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}oauth_clients (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}oauth_client_secrets_hash_key UNIQUE (secret_hash),
  CONSTRAINT {{.TablePrefix}}oauth_client_secrets_hash_format CHECK (secret_hash ~ '^sha256\$[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}oauth_client_secrets_hint_length CHECK (char_length(hint) = 4)
);

CREATE UNIQUE INDEX {{.TablePrefix}}oauth_client_secrets_one_current ON {{.Schema}}.{{.TablePrefix}}oauth_client_secrets (client_id)
  WHERE expires_at IS NULL;
CREATE INDEX {{.TablePrefix}}oauth_client_secrets_client_idx ON {{.Schema}}.{{.TablePrefix}}oauth_client_secrets (client_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_client_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_client_secrets FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}oauth_client_secrets_tenant ON {{.Schema}}.{{.TablePrefix}}oauth_client_secrets
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}oauth_client_secrets;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}oauth_clients;

-- +goose StatementEnd
