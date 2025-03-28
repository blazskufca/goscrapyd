-- name: InsertTask :one
INSERT INTO tasks (
   id, name, project, spider, jobid, settings_arguments, selected_nodes, cron_string, paused, created_by
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetTasks :many
SELECT * FROM tasks;

-- name: GetTasksWithLatestJobMetadata :many
SELECT
    t.id AS task_id,
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
         LEFT JOIN (
    SELECT task_id, MAX(update_time) AS latest_update
    FROM jobs
    WHERE status = 'finished'
    GROUP BY task_id
) j_max ON j_max.task_id = t.id
         LEFT JOIN jobs j ON j.task_id = j_max.task_id
    AND j.update_time = j_max.latest_update
    AND j.node = t.selected_nodes
GROUP BY t.id
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
SELECT
    t.id AS task_id,
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
         LEFT JOIN (
    SELECT task_id, MAX(update_time) AS latest_update
    FROM jobs
    WHERE status = 'finished'
    GROUP BY task_id
) j_max ON j_max.task_id = t.id
         LEFT JOIN jobs j ON j.task_id = j_max.task_id
    AND j.update_time = j_max.latest_update
    AND j.node = t.selected_nodes
WHERE
    LOWER(t.name) LIKE '%' || LOWER(sqlc.arg(searchTerm)) || '%' OR
    LOWER(t.spider) LIKE '%' || LOWER(sqlc.arg(searchTerm)) || '%'
GROUP BY t.id
ORDER BY t.name;
