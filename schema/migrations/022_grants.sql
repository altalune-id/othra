-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}grant_families (
  id                      UUID PRIMARY KEY,
  public_id               TEXT NOT NULL UNIQUE,
  org_id                  UUID NOT NULL,
  project_id              UUID NOT NULL,
  client_id               UUID NOT NULL,
  end_user_id             UUID NOT NULL,
  user_session_id         UUID,
  user_session_public_id  TEXT NOT NULL,
  scopes                  TEXT[] NOT NULL,
  auth_time               TIMESTAMPTZ NOT NULL,
  created_at              TIMESTAMPTZ NOT NULL,
  last_used_at            TIMESTAMPTZ NOT NULL,
  absolute_expires_at     TIMESTAMPTZ NOT NULL,
  revoked_at              TIMESTAMPTZ,
  revoked_reason          TEXT,
  resource_server_id      UUID,
  CONSTRAINT {{.TablePrefix}}grant_families_client_fkey FOREIGN KEY (org_id, project_id, client_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}oauth_clients (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}grant_families_end_user_fkey FOREIGN KEY (org_id, project_id, end_user_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}end_users (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}grant_families_user_session_fkey FOREIGN KEY (org_id, project_id, user_session_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}user_sessions (org_id, project_id, id) ON DELETE SET NULL (user_session_id),
  CONSTRAINT {{.TablePrefix}}grant_families_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE SET NULL (resource_server_id),
  CONSTRAINT {{.TablePrefix}}grant_families_org_project_id_key UNIQUE (org_id, project_id, id),
  CONSTRAINT {{.TablePrefix}}grant_families_revoked_check CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL)),
  CONSTRAINT {{.TablePrefix}}grant_families_revoked_reason_check CHECK (
    revoked_reason IN ('reuse', 'code_replay', 'client_mismatch', 'end_session', 'sign_out')
  ),
  CONSTRAINT {{.TablePrefix}}grant_families_expiry CHECK (absolute_expires_at > created_at)
);

CREATE INDEX {{.TablePrefix}}grant_families_client_end_user_idx ON {{.Schema}}.{{.TablePrefix}}grant_families (project_id, client_id, end_user_id);
CREATE INDEX {{.TablePrefix}}grant_families_user_session_idx ON {{.Schema}}.{{.TablePrefix}}grant_families (project_id, user_session_id);
CREATE INDEX {{.TablePrefix}}grant_families_expiry_idx ON {{.Schema}}.{{.TablePrefix}}grant_families (project_id, absolute_expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}grant_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}grant_families FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}grant_families_tenant ON {{.Schema}}.{{.TablePrefix}}grant_families
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}authorization_codes (
  id                     UUID PRIMARY KEY,
  org_id                 UUID NOT NULL,
  project_id             UUID NOT NULL,
  -- SECURITY: only the SHA-256 of the code is stored.
  code_hash              TEXT NOT NULL,
  client_id              UUID NOT NULL,
  end_user_id            UUID NOT NULL,
  user_session_id        UUID,
  redirect_uri           TEXT NOT NULL,
  scopes                 TEXT[] NOT NULL,
  code_challenge         TEXT,
  code_challenge_method  TEXT,
  nonce                  TEXT,
  auth_time              TIMESTAMPTZ NOT NULL,
  created_at             TIMESTAMPTZ NOT NULL,
  expires_at             TIMESTAMPTZ NOT NULL,
  used_at                TIMESTAMPTZ,
  family_id              UUID,
  resource_server_id     UUID,
  CONSTRAINT {{.TablePrefix}}authorization_codes_client_fkey FOREIGN KEY (org_id, project_id, client_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}oauth_clients (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}authorization_codes_end_user_fkey FOREIGN KEY (org_id, project_id, end_user_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}end_users (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}authorization_codes_user_session_fkey FOREIGN KEY (org_id, project_id, user_session_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}user_sessions (org_id, project_id, id) ON DELETE SET NULL (user_session_id),
  CONSTRAINT {{.TablePrefix}}authorization_codes_family_fkey FOREIGN KEY (org_id, project_id, family_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}grant_families (org_id, project_id, id) ON DELETE SET NULL (family_id),
  CONSTRAINT {{.TablePrefix}}authorization_codes_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE SET NULL (resource_server_id),
  CONSTRAINT {{.TablePrefix}}authorization_codes_hash_key UNIQUE (project_id, code_hash),
  CONSTRAINT {{.TablePrefix}}authorization_codes_hash_format CHECK (code_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}authorization_codes_redirect_uri_length CHECK (char_length(redirect_uri) BETWEEN 1 AND 2000),
  CONSTRAINT {{.TablePrefix}}authorization_codes_pkce_check CHECK (
    (code_challenge IS NULL) = (code_challenge_method IS NULL)
    AND (code_challenge IS NULL OR (code_challenge ~ '^[A-Za-z0-9_-]{43}$' AND code_challenge_method = 'S256'))
  ),
  CONSTRAINT {{.TablePrefix}}authorization_codes_nonce_length CHECK (char_length(nonce) <= 512),
  CONSTRAINT {{.TablePrefix}}authorization_codes_expiry CHECK (expires_at > created_at),
  CONSTRAINT {{.TablePrefix}}authorization_codes_used_family CHECK (used_at IS NOT NULL OR family_id IS NULL)
);

CREATE INDEX {{.TablePrefix}}authorization_codes_expiry_idx ON {{.Schema}}.{{.TablePrefix}}authorization_codes (project_id, expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}authorization_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}authorization_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}authorization_codes_tenant ON {{.Schema}}.{{.TablePrefix}}authorization_codes
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}access_tokens (
  id          UUID PRIMARY KEY,
  org_id      UUID NOT NULL,
  project_id  UUID NOT NULL,
  family_id   UUID NOT NULL,
  -- SECURITY: only the SHA-256 of the token is stored.
  token_hash  TEXT NOT NULL,
  scopes      TEXT[] NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL,
  expires_at  TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}access_tokens_family_fkey FOREIGN KEY (org_id, project_id, family_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}grant_families (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}access_tokens_hash_key UNIQUE (project_id, token_hash),
  CONSTRAINT {{.TablePrefix}}access_tokens_hash_format CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}access_tokens_expiry CHECK (expires_at > created_at)
);

CREATE INDEX {{.TablePrefix}}access_tokens_expiry_idx ON {{.Schema}}.{{.TablePrefix}}access_tokens (project_id, expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}access_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}access_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}access_tokens_tenant ON {{.Schema}}.{{.TablePrefix}}access_tokens
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}refresh_tokens (
  id          UUID PRIMARY KEY,
  org_id      UUID NOT NULL,
  project_id  UUID NOT NULL,
  family_id   UUID NOT NULL,
  -- SECURITY: only the SHA-256 of the token is stored.
  token_hash  TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL,
  expires_at  TIMESTAMPTZ NOT NULL,
  used_at     TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}refresh_tokens_family_fkey FOREIGN KEY (org_id, project_id, family_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}grant_families (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}refresh_tokens_hash_key UNIQUE (project_id, token_hash),
  CONSTRAINT {{.TablePrefix}}refresh_tokens_hash_format CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}refresh_tokens_expiry CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX {{.TablePrefix}}refresh_tokens_one_live ON {{.Schema}}.{{.TablePrefix}}refresh_tokens (family_id) WHERE used_at IS NULL;
CREATE INDEX {{.TablePrefix}}refresh_tokens_expiry_idx ON {{.Schema}}.{{.TablePrefix}}refresh_tokens (project_id, expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}refresh_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}refresh_tokens_tenant ON {{.Schema}}.{{.TablePrefix}}refresh_tokens
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}refresh_tokens;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}access_tokens;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}authorization_codes;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}grant_families;

-- +goose StatementEnd
