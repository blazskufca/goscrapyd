-- +goose Up
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY UNIQUE,
    name TEXT,
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    project TEXT NOT NULL,
    spider TEXT NOT NULL,
    jobid TEXT NOT NULL,
    settings_arguments TEXT NOT NULL,
    selected_nodes TEXT NOT NULL,
    cron_string TEXT NOT NULL,
    paused BOOL NOT NULL,
    created_by UUID,
    modified_by UUID,
    FOREIGN KEY (created_by) REFERENCES users(ID) ON DELETE SET NULL ON UPDATE CASCADE,
    FOREIGN KEY (modified_by) REFERENCES users(ID) ON DELETE SET NULL ON UPDATE CASCADE,
    FOREIGN KEY (selected_nodes) REFERENCES scrapyd_nodes(nodeName) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_task_name ON tasks(name);
CREATE INDEX IF NOT EXISTS idx_task_spider ON tasks(spider);

-- +goose Down
DROP INDEX IF EXISTS idx_task_name;
DROP INDEX IF EXISTS idx_task_spider;
DROP TABLE IF EXISTS tasks;
