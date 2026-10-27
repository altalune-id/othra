-- +goose Up
-- +goose StatementBegin

CREATE INDEX {{.TablePrefix}}grant_families_end_user_idx ON {{.Schema}}.{{.TablePrefix}}grant_families (project_id, end_user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS {{.Schema}}.{{.TablePrefix}}grant_families_end_user_idx;

-- +goose StatementEnd
