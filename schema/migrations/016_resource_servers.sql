-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}resource_servers (
  id                    UUID PRIMARY KEY,
  public_id             TEXT NOT NULL UNIQUE,
  org_id                UUID NOT NULL,
  project_id            UUID NOT NULL,
  name                  TEXT NOT NULL,
  identifier            TEXT NOT NULL,
  scopes                TEXT[] NOT NULL,
  verification_state    TEXT NOT NULL DEFAULT 'pending',
  verified_at           TIMESTAMPTZ,
  last_checked_at       TIMESTAMPTZ,
  last_error            JSONB,
  consecutive_failures  INTEGER NOT NULL DEFAULT 0,
  trusted               BOOLEAN NOT NULL DEFAULT false,
  trusted_at            TIMESTAMPTZ,
  trusted_by            UUID,
  created_at            TIMESTAMPTZ NOT NULL,
  updated_at            TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}resource_servers_org_project_fkey FOREIGN KEY (org_id, project_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}resource_servers_org_project_id_key UNIQUE (org_id, project_id, id),
  CONSTRAINT {{.TablePrefix}}resource_servers_identifier_key UNIQUE (project_id, identifier),
  CONSTRAINT {{.TablePrefix}}resource_servers_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
  CONSTRAINT {{.TablePrefix}}resource_servers_identifier_length CHECK (char_length(identifier) BETWEEN 1 AND 512),
  CONSTRAINT {{.TablePrefix}}resource_servers_scopes_count CHECK (cardinality(scopes) BETWEEN 1 AND 64),
  CONSTRAINT {{.TablePrefix}}resource_servers_state_check CHECK (verification_state IN ('pending', 'verified', 'failed')),
  CONSTRAINT {{.TablePrefix}}resource_servers_verified_at CHECK (verification_state <> 'verified' OR verified_at IS NOT NULL),
  CONSTRAINT {{.TablePrefix}}resource_servers_last_error_object CHECK (last_error IS NULL OR jsonb_typeof(last_error) = 'object'),
  CONSTRAINT {{.TablePrefix}}resource_servers_failures_nonnegative CHECK (consecutive_failures >= 0),
  CONSTRAINT {{.TablePrefix}}resource_servers_trusted_at CHECK (trusted = (trusted_at IS NOT NULL)),
  CONSTRAINT {{.TablePrefix}}resource_servers_https_or_trusted CHECK (identifier LIKE 'https://%' OR trusted)
);

CREATE INDEX {{.TablePrefix}}resource_servers_recheck_idx ON {{.Schema}}.{{.TablePrefix}}resource_servers (project_id, last_checked_at)
  WHERE NOT trusted;

ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_servers ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_servers FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}resource_servers_tenant ON {{.Schema}}.{{.TablePrefix}}resource_servers
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

CREATE TABLE {{.Schema}}.{{.TablePrefix}}oauth_client_resources (
  org_id              UUID NOT NULL,
  project_id          UUID NOT NULL,
  client_id           UUID NOT NULL,
  resource_server_id  UUID NOT NULL,
  scopes              TEXT[] NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL,
  updated_at          TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (client_id, resource_server_id),
  CONSTRAINT {{.TablePrefix}}oauth_client_resources_client_fkey FOREIGN KEY (org_id, project_id, client_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}oauth_clients (org_id, project_id, id) ON DELETE CASCADE,
  -- NOTE: NO ACTION, not RESTRICT, so a project delete that cascades to both sides still succeeds.
  CONSTRAINT {{.TablePrefix}}oauth_client_resources_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE NO ACTION,
  CONSTRAINT {{.TablePrefix}}oauth_client_resources_scopes_count CHECK (cardinality(scopes) BETWEEN 1 AND 64)
);

CREATE INDEX {{.TablePrefix}}oauth_client_resources_resource_server_idx ON {{.Schema}}.{{.TablePrefix}}oauth_client_resources (resource_server_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_client_resources ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_client_resources FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}oauth_client_resources_tenant ON {{.Schema}}.{{.TablePrefix}}oauth_client_resources
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}oauth_client_resources;
DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}resource_servers;

-- +goose StatementEnd
