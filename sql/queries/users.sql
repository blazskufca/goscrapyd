-- name: CreateNewUser :one
INSERT INTO users (ID, username, hashed_password, has_admin_privileges) VALUES (?, ?, ?, ?) RETURNING *;

-- name: GetUserWithID :one
/* @sqlc.returns *users */
SELECT * FROM users WHERE ID = ? LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ? LIMIT 1;

-- name: UpdateUsersPasswordWhereID :exec
UPDATE users SET hashed_password=? WHERE ID = ?;

-- name: GetAllUsers :many
SELECT * FROM users;

-- name: DeleteUserByUUID :exec
DELETE FROM users WHERE ID = ?;

-- name: UpdateUserWhereUUID :exec
UPDATE users SET username=?, hashed_password=?, has_admin_privileges = ? WHERE ID =?;
