-- name: CheckSettingsExist :one
SELECT EXISTS (SELECT 1 FROM settings LIMIT 1);

-- name: InsertSettings :one
INSERT INTO settings (default_project_path, default_project_name, persisted_spider_settings)
SELECT ?, ?, ?
    WHERE NOT EXISTS (SELECT 1 FROM settings) RETURNING *;

-- name: GetSettings :one
SELECT * FROM settings LIMIT 1;

-- name: UpdateSettings :exec
UPDATE settings
SET default_project_path = ?,
    default_project_name = ?,
    persisted_spider_settings = ?
WHERE EXISTS (SELECT 1 FROM settings LIMIT 1);
