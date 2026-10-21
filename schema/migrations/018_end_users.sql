-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}end_users (
  id                 UUID PRIMARY KEY,
  public_id          TEXT NOT NULL UNIQUE,
  org_id             UUID NOT NULL,
  project_id         UUID NOT NULL,
  email              TEXT NOT NULL,
  email_verified_at  TIMESTAMPTZ,
  name               TEXT NOT NULL DEFAULT '',
  last_sign_in_at    TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL,
  updated_at         TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}end_users_org_project_fkey FOREIGN KEY (org_id, project_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}end_users_org_project_id_key UNIQUE (org_id, project_id, id),
  CONSTRAINT {{.TablePrefix}}end_users_email_key UNIQUE (project_id, email),
  CONSTRAINT {{.TablePrefix}}end_users_email_normalized CHECK (email = lower(btrim(email)) AND char_length(email) BETWEEN 3 AND 254),
  CONSTRAINT {{.TablePrefix}}end_users_name_length CHECK (char_length(name) <= 200)
);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}end_users ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}end_users FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}end_users_tenant ON {{.Schema}}.{{.TablePrefix}}end_users
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}end_users;

-- +goose StatementEnd
