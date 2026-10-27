-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}end_user_identities (
  id                    UUID PRIMARY KEY,
  public_id             TEXT NOT NULL UNIQUE,
  org_id                UUID NOT NULL,
  project_id            UUID NOT NULL,
  end_user_id           UUID NOT NULL,
  upstream_provider_id  UUID NOT NULL,
  issuer                TEXT NOT NULL,
  subject               TEXT NOT NULL,
  email                 TEXT,
  email_verified        BOOLEAN NOT NULL DEFAULT false,
  linked_at             TIMESTAMPTZ NOT NULL,
  last_used_at          TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}end_user_identities_end_user_fkey FOREIGN KEY (org_id, project_id, end_user_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}end_users (org_id, project_id, id) ON DELETE CASCADE,
  -- NOTE: NO ACTION, not RESTRICT: an upstream with identities can't be deleted alone, but a project delete still cascades to both.
  CONSTRAINT {{.TablePrefix}}end_user_identities_upstream_fkey FOREIGN KEY (org_id, project_id, upstream_provider_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}upstream_providers (org_id, project_id, id) ON DELETE NO ACTION,
  CONSTRAINT {{.TablePrefix}}end_user_identities_subject_key UNIQUE (project_id, issuer, subject),
  CONSTRAINT {{.TablePrefix}}end_user_identities_one_per_upstream UNIQUE (project_id, end_user_id, upstream_provider_id),
  CONSTRAINT {{.TablePrefix}}end_user_identities_issuer_https CHECK (issuer LIKE 'https://%' AND char_length(issuer) <= 512),
  CONSTRAINT {{.TablePrefix}}end_user_identities_subject_length CHECK (char_length(subject) BETWEEN 1 AND 255),
  CONSTRAINT {{.TablePrefix}}end_user_identities_email_length CHECK (char_length(email) <= 320),
  CONSTRAINT {{.TablePrefix}}end_user_identities_last_used CHECK (last_used_at >= linked_at)
);

CREATE INDEX {{.TablePrefix}}end_user_identities_upstream_idx ON {{.Schema}}.{{.TablePrefix}}end_user_identities (project_id, upstream_provider_id);
CREATE INDEX {{.TablePrefix}}end_user_identities_end_user_idx ON {{.Schema}}.{{.TablePrefix}}end_user_identities (project_id, end_user_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}end_user_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}end_user_identities FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}end_user_identities_tenant ON {{.Schema}}.{{.TablePrefix}}end_user_identities
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

ALTER TABLE {{.Schema}}.{{.TablePrefix}}user_sessions
  DROP CONSTRAINT {{.TablePrefix}}user_sessions_method_check,
  ADD CONSTRAINT {{.TablePrefix}}user_sessions_method_check CHECK (method IN ('magic_link', 'google'));

ALTER TABLE {{.Schema}}.{{.TablePrefix}}magic_links
  ADD COLUMN pending_identity JSONB,
  ADD CONSTRAINT {{.TablePrefix}}magic_links_pending_identity_object CHECK (jsonb_typeof(pending_identity) = 'object');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}magic_links
  DROP CONSTRAINT IF EXISTS {{.TablePrefix}}magic_links_pending_identity_object,
  DROP COLUMN IF EXISTS pending_identity;

ALTER TABLE {{.Schema}}.{{.TablePrefix}}user_sessions
  DROP CONSTRAINT {{.TablePrefix}}user_sessions_method_check,
  ADD CONSTRAINT {{.TablePrefix}}user_sessions_method_check CHECK (method IN ('magic_link'));

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}end_user_identities;

-- +goose StatementEnd
