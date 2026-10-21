-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}magic_links (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  org_id              UUID NOT NULL,
  project_id          UUID NOT NULL,
  -- SECURITY: only the SHA-256 of the token is stored; the token lives in the email alone.
  token_hash          TEXT NOT NULL,
  email               TEXT NOT NULL,
  return_to           TEXT NOT NULL,
  request_ip          INET,
  created_at          TIMESTAMPTZ NOT NULL,
  expires_at          TIMESTAMPTZ NOT NULL,
  used_at             TIMESTAMPTZ,
  browser_session_id  UUID,
  CONSTRAINT {{.TablePrefix}}magic_links_org_project_fkey FOREIGN KEY (org_id, project_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}magic_links_browser_session_fkey FOREIGN KEY (org_id, project_id, browser_session_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}browser_sessions (org_id, project_id, id) ON DELETE SET NULL (browser_session_id),
  CONSTRAINT {{.TablePrefix}}magic_links_token_key UNIQUE (project_id, token_hash),
  CONSTRAINT {{.TablePrefix}}magic_links_token_format CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}magic_links_email_normalized CHECK (email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254),
  CONSTRAINT {{.TablePrefix}}magic_links_return_to_path CHECK (left(return_to, 1) = '/' AND char_length(return_to) <= 4096),
  CONSTRAINT {{.TablePrefix}}magic_links_expiry CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX {{.TablePrefix}}magic_links_one_live ON {{.Schema}}.{{.TablePrefix}}magic_links (project_id, email)
  WHERE used_at IS NULL;
CREATE INDEX {{.TablePrefix}}magic_links_expiry_idx ON {{.Schema}}.{{.TablePrefix}}magic_links (project_id, expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}magic_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}magic_links FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}magic_links_tenant ON {{.Schema}}.{{.TablePrefix}}magic_links
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}magic_links;

-- +goose StatementEnd
