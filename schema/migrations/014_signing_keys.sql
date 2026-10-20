-- +goose Up
-- +goose StatementBegin

CREATE TABLE {{.Schema}}.{{.TablePrefix}}signing_keys (
  id               UUID PRIMARY KEY,
  public_id        TEXT NOT NULL UNIQUE,
  org_id           UUID NOT NULL,
  project_id       UUID NOT NULL,
  kid              TEXT NOT NULL,
  alg              TEXT NOT NULL,
  public_jwk       JSONB NOT NULL,
  -- SECURITY: the private key, PKCS#8 DER sealed with the AAD "signing_key:<project_id>:<kid>"; never the raw key, and NULL once retired.
  private_key_enc  BYTEA,
  status           TEXT NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL,
  activates_at     TIMESTAMPTZ,
  activated_at     TIMESTAMPTZ,
  retire_after     TIMESTAMPTZ,
  retired_at       TIMESTAMPTZ,
  CONSTRAINT {{.TablePrefix}}signing_keys_org_project_fkey FOREIGN KEY (org_id, project_id)
    REFERENCES {{.Schema}}.{{.TablePrefix}}projects (org_id, id) ON DELETE CASCADE,
  CONSTRAINT {{.TablePrefix}}signing_keys_kid_key UNIQUE (kid),
  CONSTRAINT {{.TablePrefix}}signing_keys_alg_check CHECK (alg IN ('RS256')),
  CONSTRAINT {{.TablePrefix}}signing_keys_status_check CHECK (status IN ('next', 'active', 'retiring', 'retired')),
  CONSTRAINT {{.TablePrefix}}signing_keys_sealed_while_live CHECK (status = 'retired' OR coalesce(octet_length(private_key_enc), 0) > 0),
  CONSTRAINT {{.TablePrefix}}signing_keys_next_activates CHECK (status <> 'next' OR activates_at IS NOT NULL),
  CONSTRAINT {{.TablePrefix}}signing_keys_retiring_after CHECK (status <> 'retiring' OR retire_after IS NOT NULL)
);

CREATE UNIQUE INDEX {{.TablePrefix}}signing_keys_one_active ON {{.Schema}}.{{.TablePrefix}}signing_keys (project_id, alg)
  WHERE status = 'active';
CREATE UNIQUE INDEX {{.TablePrefix}}signing_keys_one_next ON {{.Schema}}.{{.TablePrefix}}signing_keys (project_id, alg)
  WHERE status = 'next';
CREATE INDEX {{.TablePrefix}}signing_keys_project_status_idx ON {{.Schema}}.{{.TablePrefix}}signing_keys (project_id, status);

ALTER TABLE {{.Schema}}.{{.TablePrefix}}signing_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE {{.Schema}}.{{.TablePrefix}}signing_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY {{.TablePrefix}}signing_keys_tenant ON {{.Schema}}.{{.TablePrefix}}signing_keys
  FOR ALL
  USING (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id())
  WITH CHECK (org_id = {{.Schema}}.{{.TablePrefix}}current_org_id() AND project_id = {{.Schema}}.{{.TablePrefix}}current_project_id());

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS {{.Schema}}.{{.TablePrefix}}signing_keys;

-- +goose StatementEnd
