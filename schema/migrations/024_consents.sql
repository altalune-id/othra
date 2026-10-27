-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}scope_consents (
  id                  UUID PRIMARY KEY,
  public_id           TEXT NOT NULL UNIQUE,
  org_id              UUID NOT NULL,
  project_id          UUID NOT NULL,
  end_user_id         UUID NOT NULL,
  client_id           UUID NOT NULL,
  resource_server_id  UUID,
  scopes              TEXT[] NOT NULL,
  granted_at          TIMESTAMPTZ NOT NULL,
  updated_at          TIMESTAMPTZ NOT NULL,
  granted_ip          INET,
  granted_user_agent  TEXT,
  CONSTRAINT {{.TablePrefix}}scope_consents_end_user_fkey FOREIGN KEY (org_id, project_id, end_user_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}end_users (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}scope_consents_client_fkey FOREIGN KEY (org_id, project_id, client_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}oauth_clients (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}scope_consents_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}scope_consents_grant_key UNIQUE NULLS NOT DISTINCT (project_id, end_user_id, client_id, resource_server_id),
  CONSTRAINT {{.TablePrefix}}scope_consents_scopes_count CHECK (cardinality(scopes) BETWEEN 1 AND 72),
  CONSTRAINT {{.TablePrefix}}scope_consents_updated CHECK (updated_at >= granted_at),
  CONSTRAINT {{.TablePrefix}}scope_consents_user_agent_length CHECK (char_length(granted_user_agent) <= 512)
);

CREATE INDEX {{.TablePrefix}}scope_consents_client_idx ON {{.Schema}}.{{.TablePrefix}}scope_consents (project_id, client_id);
CREATE INDEX {{.TablePrefix}}scope_consents_resource_server_idx ON {{.Schema}}.{{.TablePrefix}}scope_consents (project_id, resource_server_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}scope_consents ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}scope_consents FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}scope_consents_tenant ON {{.Schema}}.{{.TablePrefix}}scope_consents
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}scope_consents;

-- +goose StatementEnd
