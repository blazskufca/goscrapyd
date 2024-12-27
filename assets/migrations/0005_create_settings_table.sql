-- +goose Up
CREATE TABLE IF NOT EXISTS settings (
    default_project_path TEXT,
    default_project_name TEXT,
    persisted_spider_settings TEXT
);

-- +goose Down
DROP TABLE IF EXISTS settings;
