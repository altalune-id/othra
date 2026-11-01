-- +goose Up
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients ADD COLUMN last_used_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE {{.Schema}}.{{.TablePrefix}}oauth_clients DROP COLUMN IF EXISTS last_used_at;

-- +goose StatementEnd
