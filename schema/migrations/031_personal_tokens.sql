-- +goose Up
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_servers
  ADD COLUMN accept_personal_tokens BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE {{.Schema}}.{{.TablePrefix}}personal_tokens (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  org_id              UUID NOT NULL,
  project_id          UUID NOT NULL,
  end_user_id         UUID NOT NULL,
  resource_server_id  UUID NOT NULL,
  name                TEXT NOT NULL,
  -- SECURITY: "sha256$" and the hex SHA-256 of the token; the token is shown once and never stored.
  token_hash          TEXT NOT NULL,
  hint                TEXT NOT NULL,
  scopes              TEXT[] NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL,
  expires_at          TIMESTAMPTZ NOT NULL,
  last_used_at        TIMESTAMPTZ,
  revoked_at          TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}personal_tokens_end_user_fkey FOREIGN KEY (org_id, project_id, end_user_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}end_users (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}personal_tokens_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}personal_tokens_hash_key UNIQUE (project_id, token_hash),
  CONSTRAINT {{.TablePrefix}}personal_tokens_hash_format CHECK (token_hash ~ '^sha256\$[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}personal_tokens_hint_length CHECK (char_length(hint) = 4),
  CONSTRAINT {{.TablePrefix}}personal_tokens_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
  CONSTRAINT {{.TablePrefix}}personal_tokens_scopes_count CHECK (cardinality(scopes) BETWEEN 1 AND 64),
  -- SECURITY: profile §2.5: a personal token lives at most 90 days, whatever the caller asked for.
  CONSTRAINT {{.TablePrefix}}personal_tokens_lifetime CHECK (expires_at > created_at AND expires_at <= created_at + interval '90 days')
);

CREATE INDEX {{.TablePrefix}}personal_tokens_end_user_idx ON {{.Schema}}.{{.TablePrefix}}personal_tokens (project_id, end_user_id);
CREATE INDEX {{.TablePrefix}}personal_tokens_expiry_idx ON {{.Schema}}.{{.TablePrefix}}personal_tokens (project_id, expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}personal_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}personal_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}personal_tokens_tenant ON {{.Schema}}.{{.TablePrefix}}personal_tokens
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}personal_tokens;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_servers DROP COLUMN IF EXISTS accept_personal_tokens;

-- +goose StatementEnd
