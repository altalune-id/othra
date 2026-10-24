-- +goose Up
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}grant_families
  DROP CONSTRAINT {{.TablePrefix}}grant_families_resource_server_fkey,
  ADD CONSTRAINT {{.TablePrefix}}grant_families_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE CASCADE,
  DROP CONSTRAINT {{.TablePrefix}}grant_families_revoked_reason_check,
  ADD CONSTRAINT {{.TablePrefix}}grant_families_revoked_reason_check CHECK (
    revoked_reason IN ('reuse', 'code_replay', 'client_mismatch', 'end_session', 'sign_out', 'revoked', 'consent_revoked')
  );

ALTER TABLE {{.Schema}}.{{.TablePrefix}}authorization_codes
  DROP CONSTRAINT {{.TablePrefix}}authorization_codes_resource_server_fkey,
  ADD CONSTRAINT {{.TablePrefix}}authorization_codes_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE CASCADE;

ALTER TABLE {{.Schema}}.{{.TablePrefix}}access_tokens
  ADD COLUMN resource_server_id UUID,
  ADD COLUMN revoked_at TIMESTAMPTZ,
  ADD CONSTRAINT {{.TablePrefix}}access_tokens_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}access_tokens
  DROP CONSTRAINT IF EXISTS {{.TablePrefix}}access_tokens_resource_server_fkey,
  DROP COLUMN IF EXISTS revoked_at,
  DROP COLUMN IF EXISTS resource_server_id;

ALTER TABLE {{.Schema}}.{{.TablePrefix}}authorization_codes
  DROP CONSTRAINT {{.TablePrefix}}authorization_codes_resource_server_fkey,
  ADD CONSTRAINT {{.TablePrefix}}authorization_codes_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE SET NULL (resource_server_id);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}grant_families
  DROP CONSTRAINT {{.TablePrefix}}grant_families_revoked_reason_check,
  ADD CONSTRAINT {{.TablePrefix}}grant_families_revoked_reason_check CHECK (
    revoked_reason IN ('reuse', 'code_replay', 'client_mismatch', 'end_session', 'sign_out')
  ),
  DROP CONSTRAINT {{.TablePrefix}}grant_families_resource_server_fkey,
  ADD CONSTRAINT {{.TablePrefix}}grant_families_resource_server_fkey FOREIGN KEY (org_id, project_id, resource_server_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}resource_servers (org_id, project_id, id) ON DELETE SET NULL (resource_server_id);

-- +goose StatementEnd
