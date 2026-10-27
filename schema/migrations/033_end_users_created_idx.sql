-- +goose Up
-- +goose StatementBegin

CREATE INDEX {{.TablePrefix}}end_users_created_idx ON {{.Schema}}.{{.TablePrefix}}end_users (project_id, created_at DESC, id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS {{.Schema}}.{{.TablePrefix}}end_users_created_idx;

-- +goose StatementEnd
