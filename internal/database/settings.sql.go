// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: settings.sql

package database

import (
	"context"
	"database/sql"
)

const checkSettingsExist = `-- name: CheckSettingsExist :one
SELECT EXISTS (SELECT 1 FROM settings LIMIT 1)
`

func (q *Queries) CheckSettingsExist(ctx context.Context) (int64, error) {
	row := q.queryRow(ctx, q.checkSettingsExistStmt, checkSettingsExist)
	var column_1 int64
	err := row.Scan(&column_1)
	return column_1, err
}

const getSettings = `-- name: GetSettings :one
SELECT default_project_path, default_project_name, persisted_spider_settings FROM settings LIMIT 1
`

func (q *Queries) GetSettings(ctx context.Context) (Setting, error) {
	row := q.queryRow(ctx, q.getSettingsStmt, getSettings)
	var i Setting
	err := row.Scan(&i.DefaultProjectPath, &i.DefaultProjectName, &i.PersistedSpiderSettings)
	return i, err
}

const insertSettings = `-- name: InsertSettings :one
INSERT INTO settings (default_project_path, default_project_name, persisted_spider_settings)
SELECT ?, ?, ?
    WHERE NOT EXISTS (SELECT 1 FROM settings) RETURNING default_project_path, default_project_name, persisted_spider_settings
`

type InsertSettingsParams struct {
	DefaultProjectPath      sql.NullString
	DefaultProjectName      sql.NullString
	PersistedSpiderSettings sql.NullString
}

func (q *Queries) InsertSettings(ctx context.Context, arg InsertSettingsParams) (Setting, error) {
	row := q.queryRow(ctx, q.insertSettingsStmt, insertSettings, arg.DefaultProjectPath, arg.DefaultProjectName, arg.PersistedSpiderSettings)
	var i Setting
	err := row.Scan(&i.DefaultProjectPath, &i.DefaultProjectName, &i.PersistedSpiderSettings)
	return i, err
}

const updateSettings = `-- name: UpdateSettings :exec
UPDATE settings
SET default_project_path = ?,
    default_project_name = ?,
    persisted_spider_settings = ?
WHERE EXISTS (SELECT 1 FROM settings LIMIT 1)
`

type UpdateSettingsParams struct {
	DefaultProjectPath      sql.NullString
	DefaultProjectName      sql.NullString
	PersistedSpiderSettings sql.NullString
}

func (q *Queries) UpdateSettings(ctx context.Context, arg UpdateSettingsParams) error {
	_, err := q.exec(ctx, q.updateSettingsStmt, updateSettings, arg.DefaultProjectPath, arg.DefaultProjectName, arg.PersistedSpiderSettings)
	return err
}
