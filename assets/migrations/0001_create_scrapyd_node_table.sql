-- +goose Up
CREATE TABLE IF NOT EXISTS scrapyd_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nodeName TEXT NOT NULL UNIQUE,
    URL TEXT NOT NULL,
    username TEXT,
    password BLOB,
    CONSTRAINT unique_node UNIQUE (nodeName, URL)
);
CREATE INDEX IF NOT EXISTS idx_nodename ON scrapyd_nodes(nodeName);

-- +goose Down
DROP INDEX IF EXISTS idx_nodename;
DROP TABLE IF EXISTS scrapyd_nodes;
