-- name: InsertTask :one
INSERT INTO tasks (
   id, name, project, spider, jobid, settings_arguments, selected_nodes, cron_string, paused, created_by
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetTasks :many
SELECT * FROM tasks;

-- name: GetTasksWithLatestJobMetadata :many
SELECT t.id AS task_id,
       t.name,
       t.create_time AS task_create_time,
       t.update_time AS task_update_time,
       t.project,
       t.spider,
       t.jobid,
       t.settings_arguments,
       t.selected_nodes,
       t.cron_string,
       t.paused,
       creator.username AS created_by_username,
       modifier.username AS modified_by_username,
       j.id AS job_id,
       j.create_time AS job_create_time,
       j.pages AS job_pages,
       j.items AS job_items,
       j.start AS job_start,
       j.runtime AS job_runtime,
       j.finish AS job_finish,
       j.href_log,
       j.href_items,
       j.node AS job_node
FROM tasks t
         LEFT JOIN users creator ON t.created_by = creator.ID
         LEFT JOIN users modifier ON t.modified_by = modifier.ID
         LEFT JOIN jobs j ON j.task_id = t.id
    AND j.status = 'finished'
    AND j.update_time = (
        SELECT MAX(update_time)
        FROM jobs
        WHERE task_id = t.id AND status = 'finished' AND j.node = t.selected_nodes LIMIT 1
    )
ORDER BY t.name DESC;

-- name: UpdateTaskPaused :exec
UPDATE tasks SET paused=? WHERE id = ?;

-- name: GetTaskWithUUID :one
SELECT * FROM tasks WHERE id = ?;

-- name: DeleteTaskWhereUUID :exec
DELETE FROM tasks WHERE id = ?;

-- name: UpdateTask :exec
UPDATE tasks
SET
    name = ?,
    project = ?,
    spider = ?,
    jobid = ?,
    settings_arguments = ?,
    selected_nodes = ?,
    cron_string = ?,
    paused = ?,
    modified_by = ?
WHERE id = ?;

-- name: SearchTasksTable :many
SELECT t.id AS task_id,
       t.name,
       t.create_time AS task_create_time,
       t.update_time AS task_update_time,
       t.project,
       t.spider,
       t.jobid,
       t.settings_arguments,
       t.selected_nodes,
       t.cron_string,
       t.paused,
       creator.username AS created_by_username,
       modifier.username AS modified_by_username,
       j.id AS job_id,
       j.create_time AS job_create_time,
       j.pages AS job_pages,
       j.items AS job_items,
       j.start AS job_start,
       j.runtime AS job_runtime,
       j.finish AS job_finish,
       j.href_log,
       j.href_items,
       j.node AS job_node
FROM tasks t
         LEFT JOIN users creator ON t.created_by = creator.ID
         LEFT JOIN users modifier ON t.modified_by = modifier.ID
         LEFT JOIN jobs j ON j.task_id = t.id
    AND j.status = 'finished'
    AND j.update_time = (
        SELECT MAX(update_time)
        FROM jobs
        WHERE task_id = t.id AND status = 'finished' AND j.node = t.selected_nodes LIMIT 1
    )
WHERE
    LOWER(t.name) LIKE '%' || LOWER(sqlc.arg(searchTerm)) || '%' OR
    LOWER(t.spider) LIKE '%' || LOWER(sqlc.arg(searchTerm)) || '%'
ORDER BY t.name;
