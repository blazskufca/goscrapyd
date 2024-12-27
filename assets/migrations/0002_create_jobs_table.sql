-- +goose Up
CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    spider TEXT NOT NULL,
    job TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('scheduled', 'pending', 'running', 'finished', 'error')),
    deleted BOOL NOT NULL DEFAULT false,
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    pages INTEGER,
    items INTEGER,
    pid INTEGER,
    start DATETIME,
    runtime TEXT,
    finish DATETIME,
    href_log TEXT,
    href_items TEXT,
    node TEXT NOT NULL,
    task_id UUID,
    error TEXT,
    started_by UUID,
    stopped_by UUID,
    CONSTRAINT uniqueRow UNIQUE (project, spider, job),
    FOREIGN KEY (started_by) REFERENCES users(ID) ON DELETE SET NULL ON UPDATE CASCADE,
    FOREIGN KEY (stopped_by) REFERENCES users(ID) ON DELETE SET NULL ON UPDATE CASCADE,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL ON UPDATE CASCADE,
    FOREIGN KEY (node) REFERENCES scrapyd_nodes(nodeName) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_job ON jobs(job);
CREATE INDEX IF NOT EXISTS idx_spider ON jobs(spider);

-- +goose Down
DROP INDEX IF EXISTS idx_job ON jobs(job);
DROP INDEX IF EXISTS idx_spider ON jobs(spider);
DROP TABLE IF EXISTS jobs;
