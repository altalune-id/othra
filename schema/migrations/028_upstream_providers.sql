-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}upstream_providers (
  id                    UUID PRIMARY KEY,
  public_id             TEXT NOT NULL UNIQUE,
  org_id                UUID NOT NULL,
  project_id            UUID NOT NULL,
  kind                  TEXT NOT NULL,
  client_id             TEXT NOT NULL,
  -- SECURITY: sealed with the AAD upstream_provider:<project_id>:<id>:client_secret; never selected by a list or get.
  client_secret_sealed  BYTEA NOT NULL,
  enabled               BOOLEAN NOT NULL DEFAULT true,
  created_at            TIMESTAMPTZ NOT NULL,
  updated_at            TIMESTAMPTZ NOT NULL,
  CONSTRAINT {{.TablePrefix}}upstream_providers_org_project_fkey FOREIGN KEY (org_id, project_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}upstream_providers_org_project_id_key UNIQUE (org_id, project_id, id),
  CONSTRAINT {{.TablePrefix}}upstream_providers_kind_key UNIQUE (project_id, kind),
  CONSTRAINT {{.TablePrefix}}upstream_providers_kind_check CHECK (kind IN ('google')),
  CONSTRAINT {{.TablePrefix}}upstream_providers_client_id_length CHECK (char_length(client_id) BETWEEN 1 AND 255),
  CONSTRAINT {{.TablePrefix}}upstream_providers_secret_sealed CHECK (octet_length(client_secret_sealed) >= 28)
);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}upstream_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}upstream_providers FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}upstream_providers_tenant ON {{.Schema}}.{{.TablePrefix}}upstream_providers
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}upstream_providers;

-- +goose StatementEnd
