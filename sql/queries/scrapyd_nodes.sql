-- name: NewScrapydNode :one
INSERT INTO scrapyd_nodes (
    nodeName, URL, username, password
) VALUES (?, ?, ?, ?) RETURNING *;

-- name: ListScrapydNodes :many
SELECT * FROM scrapyd_nodes;

-- name: DeleteScrapydNodes :exec
DELETE FROM scrapyd_nodes WHERE nodeName = ?;

-- name: GetNodeWithName :one
SELECT * FROM scrapyd_nodes WHERE nodeName = ? LIMIT 1;

-- name: UpdateNodeWhereName :exec
UPDATE scrapyd_nodes SET nodeName = sqlc.arg('new_node_name'), URL = sqlc.arg('new_URL'), username = sqlc.arg('new_username'),
                         password = sqlc.arg('new_password') WHERE nodeName = sqlc.arg('old_node_name');
