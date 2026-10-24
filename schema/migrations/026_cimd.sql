-- +goose Up
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients
  ADD COLUMN registration_kind TEXT NOT NULL DEFAULT 'registered',
  ADD COLUMN metadata_url TEXT,
  ADD COLUMN metadata_fetched_at TIMESTAMPTZ,
  ADD COLUMN metadata_expires_at TIMESTAMPTZ,
  ADD COLUMN client_uri TEXT,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_registration_kind_check CHECK (registration_kind IN ('registered', 'cimd')),
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_metadata_url_key UNIQUE (project_id, metadata_url),
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_metadata_url_https CHECK (metadata_url LIKE 'https://%' AND char_length(metadata_url) <= 2000),
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_client_uri_https CHECK (client_uri LIKE 'https://%' AND char_length(client_uri) <= 2000),
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_metadata CHECK (
    (registration_kind = 'cimd') = (metadata_url IS NOT NULL)
    AND (registration_kind = 'registered' OR (metadata_fetched_at IS NOT NULL AND metadata_expires_at IS NOT NULL))
  ),
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_public CHECK (
    registration_kind = 'registered'
    OR (client_type = 'public' AND application_type = 'native' AND token_endpoint_auth_method = 'none' AND consent_policy = 'require')
  );

CREATE INDEX {{.TablePrefix}}oauth_clients_metadata_expiry_idx ON {{.Schema}}.{{.TablePrefix}}oauth_clients (project_id, metadata_expires_at)
  WHERE registration_kind = 'cimd';

ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_servers
  ADD COLUMN accept_cimd_clients BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN cimd_client_hosts TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN cimd_scopes TEXT[],
  ADD COLUMN scope_descriptions JSONB NOT NULL DEFAULT '{}',
  ADD CONSTRAINT {{.TablePrefix}}resource_servers_cimd_hosts_count CHECK (cardinality(cimd_client_hosts) <= 64),
  ADD CONSTRAINT {{.TablePrefix}}resource_servers_cimd_scopes_count CHECK (cardinality(cimd_scopes) BETWEEN 1 AND 64),
  ADD CONSTRAINT {{.TablePrefix}}resource_servers_scope_descriptions_object CHECK (jsonb_typeof(scope_descriptions) = 'object');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}resource_servers
  DROP COLUMN IF EXISTS scope_descriptions,
  DROP COLUMN IF EXISTS cimd_scopes,
  DROP COLUMN IF EXISTS cimd_client_hosts,
  DROP COLUMN IF EXISTS accept_cimd_clients;

DROP INDEX IF EXISTS {{.Schema}}.{{.TablePrefix}}oauth_clients_metadata_expiry_idx;

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients
  DROP COLUMN IF EXISTS client_uri,
  DROP COLUMN IF EXISTS metadata_expires_at,
  DROP COLUMN IF EXISTS metadata_fetched_at,
  DROP COLUMN IF EXISTS metadata_url,
  DROP COLUMN IF EXISTS registration_kind;

-- +goose StatementEnd
