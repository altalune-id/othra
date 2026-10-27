-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}browser_sessions (
  id                   UUID PRIMARY KEY,
  org_id               UUID NOT NULL,
  project_id           UUID NOT NULL,
  -- SECURITY: only the SHA-256 of the cookie secret is stored, so a database leak yields no live cookie.
  secret_hash          TEXT NOT NULL,
  user_agent           TEXT NOT NULL DEFAULT '',
  created_at           TIMESTAMPTZ NOT NULL,
  last_seen_at         TIMESTAMPTZ NOT NULL,
  absolute_expires_at  TIMESTAMPTZ NOT NULL,
  ended_at             TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}browser_sessions_org_project_fkey FOREIGN KEY (org_id, project_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}browser_sessions_org_project_id_key UNIQUE (org_id, project_id, id),
  CONSTRAINT {{.TablePrefix}}browser_sessions_secret_key UNIQUE (project_id, secret_hash),
  CONSTRAINT {{.TablePrefix}}browser_sessions_secret_format CHECK (secret_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT {{.TablePrefix}}browser_sessions_user_agent_length CHECK (char_length(user_agent) <= 512),
  CONSTRAINT {{.TablePrefix}}browser_sessions_expiry CHECK (absolute_expires_at > created_at)
);

CREATE INDEX {{.TablePrefix}}browser_sessions_expiry_idx ON {{.Schema}}.{{.TablePrefix}}browser_sessions (project_id, absolute_expires_at);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}browser_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}browser_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}browser_sessions_tenant ON {{.Schema}}.{{.TablePrefix}}browser_sessions
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}user_sessions (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  org_id              UUID NOT NULL,
  project_id          UUID NOT NULL,
  browser_session_id  UUID NOT NULL,
  end_user_id         UUID NOT NULL,
  method              TEXT NOT NULL,
  auth_time           TIMESTAMPTZ NOT NULL,
  selected_at         TIMESTAMPTZ NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL,
  last_seen_at        TIMESTAMPTZ NOT NULL,
  ended_at            TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}user_sessions_browser_session_fkey FOREIGN KEY (org_id, project_id, browser_session_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}browser_sessions (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}user_sessions_end_user_fkey FOREIGN KEY (org_id, project_id, end_user_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}end_users (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}user_sessions_org_project_id_key UNIQUE (org_id, project_id, id),
  CONSTRAINT {{.TablePrefix}}user_sessions_method_check CHECK (method IN ('magic_link'))
);

CREATE UNIQUE INDEX {{.TablePrefix}}user_sessions_one_live ON {{.Schema}}.{{.TablePrefix}}user_sessions (browser_session_id, end_user_id)
  WHERE ended_at IS NULL;
CREATE INDEX {{.TablePrefix}}user_sessions_end_user_idx ON {{.Schema}}.{{.TablePrefix}}user_sessions (project_id, end_user_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}user_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}user_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}user_sessions_tenant ON {{.Schema}}.{{.TablePrefix}}user_sessions
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}end_user_terms (
  org_id           UUID NOT NULL,
  project_id       UUID NOT NULL,
  end_user_id      UUID NOT NULL,
  terms_version    INTEGER NOT NULL,
  terms_url        TEXT NOT NULL,
  privacy_url      TEXT NOT NULL DEFAULT '',
  accepted_at      TIMESTAMPTZ NOT NULL,
  user_session_id         UUID,
  user_session_public_id  TEXT NOT NULL,
  ip                      INET,
  user_agent       TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (end_user_id, terms_version),
  CONSTRAINT {{.TablePrefix}}end_user_terms_end_user_fkey FOREIGN KEY (org_id, project_id, end_user_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}end_users (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}end_user_terms_user_session_fkey FOREIGN KEY (org_id, project_id, user_session_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}user_sessions (org_id, project_id, id) ON DELETE SET NULL (user_session_id),
  CONSTRAINT {{.TablePrefix}}end_user_terms_version_positive CHECK (terms_version > 0),
  CONSTRAINT {{.TablePrefix}}end_user_terms_user_agent_length CHECK (char_length(user_agent) <= 512)
);

-- SECURITY: consent is evidence, so the service role may read and append it but never change or delete it.
ALTER TABLE {{.Schema}}.{{.TablePrefix}}end_user_terms ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}end_user_terms FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}end_user_terms_read ON {{.Schema}}.{{.TablePrefix}}end_user_terms
  FOR SELECT
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());
CREATE POLICY {{.TablePrefix}}end_user_terms_append ON {{.Schema}}.{{.TablePrefix}}end_user_terms
  FOR INSERT
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}end_user_terms;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}user_sessions;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}browser_sessions;

-- +goose StatementEnd
