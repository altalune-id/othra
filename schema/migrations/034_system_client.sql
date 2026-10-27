-- +goose Up
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_registration_kind_check,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_registration_kind_check CHECK (registration_kind IN ('registered', 'cimd', 'system')),
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_kind_check,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_kind_check CHECK (
    (client_type = 'confidential' AND application_type = 'web' AND token_endpoint_auth_method IN ('client_secret_basic', 'client_secret_post'))
    OR (client_type = 'public' AND application_type = 'native' AND token_endpoint_auth_method = 'none')
    OR (registration_kind = 'system' AND client_type = 'public' AND application_type = 'web' AND token_endpoint_auth_method = 'none')
  ),
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_metadata,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_metadata CHECK (
    (registration_kind = 'cimd') = (metadata_url IS NOT NULL)
    AND (registration_kind <> 'cimd' OR (metadata_fetched_at IS NOT NULL AND metadata_expires_at IS NOT NULL))
  ),
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_public,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_public CHECK (
    registration_kind <> 'cimd'
    OR (client_type = 'public' AND application_type = 'native' AND token_endpoint_auth_method = 'none' AND consent_policy = 'require')
  ),
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_system_shape CHECK (
    registration_kind <> 'system'
    OR (pkce_required AND consent_policy = 'implicit' AND grant_types = '{authorization_code}')
  );

CREATE UNIQUE INDEX {{.TablePrefix}}oauth_clients_system_idx ON {{.Schema}}.{{.TablePrefix}}oauth_clients (project_id)
  WHERE registration_kind = 'system';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS {{.Schema}}.{{.TablePrefix}}oauth_clients_system_idx;

DELETE FROM {{.Schema}}.{{.TablePrefix}}oauth_clients WHERE registration_kind = 'system';

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients
  DROP CONSTRAINT IF EXISTS {{.TablePrefix}}oauth_clients_system_shape,
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_public,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_public CHECK (
    registration_kind = 'registered'
    OR (client_type = 'public' AND application_type = 'native' AND token_endpoint_auth_method = 'none' AND consent_policy = 'require')
  ),
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_metadata,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_cimd_metadata CHECK (
    (registration_kind = 'cimd') = (metadata_url IS NOT NULL)
    AND (registration_kind = 'registered' OR (metadata_fetched_at IS NOT NULL AND metadata_expires_at IS NOT NULL))
  ),
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_kind_check,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_kind_check CHECK (
    (client_type = 'confidential' AND application_type = 'web' AND token_endpoint_auth_method IN ('client_secret_basic', 'client_secret_post'))
    OR (client_type = 'public' AND application_type = 'native' AND token_endpoint_auth_method = 'none')
  ),
  DROP CONSTRAINT {{.TablePrefix}}oauth_clients_registration_kind_check,
  ADD CONSTRAINT {{.TablePrefix}}oauth_clients_registration_kind_check CHECK (registration_kind IN ('registered', 'cimd'));

-- +goose StatementEnd
