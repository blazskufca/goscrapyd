-- name: InsertJob :one
INSERT INTO jobs (
    project, spider, job, status, deleted, create_time, update_time,
    pages, items, pid, start, runtime, finish, href_log, href_items, node, task_id, started_by, stopped_by
)
VALUES (
    sqlc.arg('project'),
    sqlc.arg('spider'),
    sqlc.arg('job'),
    sqlc.arg('status'),
    sqlc.arg('deleted'),
    sqlc.arg('create_time'),
    sqlc.arg('update_time'),
    sqlc.narg('pages'),
    sqlc.narg('items'),
    sqlc.narg('pid'),
    sqlc.narg('start'),
    sqlc.narg('runtime'),
    sqlc.narg('finish'),
    sqlc.narg('href_log'),
    sqlc.narg('href_items'),
    sqlc.arg('node'),
    sqlc.arg('task_id'),
    sqlc.narg('started_by'),
        sqlc.narg('stopped_by')
       )
    ON CONFLICT(project, spider, job)
DO UPDATE SET
    status = EXCLUDED.status,
    update_time = EXCLUDED.update_time,
    pages = COALESCE(EXCLUDED.pages, jobs.pages),
    items = COALESCE(EXCLUDED.items, jobs.items),
    pid = COALESCE(EXCLUDED.pid, jobs.pid),
    start = COALESCE(EXCLUDED.start, jobs.start),
    runtime = COALESCE(EXCLUDED.runtime, jobs.runtime),
    finish = COALESCE(EXCLUDED.finish, jobs.finish),
    href_log = COALESCE(EXCLUDED.href_log, jobs.href_log),
    href_items = COALESCE(EXCLUDED.href_items, jobs.href_items),
    started_by = COALESCE(EXCLUDED.started_by, jobs.started_by),
    stopped_by = COALESCE(EXCLUDED.stopped_by, jobs.stopped_by)
WHERE jobs.deleted = 0
AND EXCLUDED.update_time >= jobs.update_time
RETURNING *;

-- name: StartFinishRuntimeLogsItemsForJobWithJobID :one
SELECT jobs.Start, jobs.Runtime, jobs.Finish, jobs.href_log, jobs.href_items, jobs.spider, jobs.Project, jobs.job, jobs.node FROM jobs WHERE job = ? LIMIT 1;

-- name: GetJobsForNode :many
SELECT j.id, j.project, j.spider, j.job, j.status, j.deleted, j.create_time, j.update_time, j.pages, j.items, j.pid,
       j.start, j.runtime, j.finish, j.href_log, j.href_items, j.node, j.error, u1.username AS started_by_username,
       u2.username AS stopped_by_username
FROM jobs j
         LEFT JOIN users u1 ON j.started_by = u1.ID
         LEFT JOIN users u2 ON j.stopped_by = u2.ID
WHERE j.node = ? AND j.deleted = 0
ORDER BY CASE
             WHEN j.finish IS NULL THEN j.runtime
             ELSE j.finish
             END DESC
LIMIT ? OFFSET ?;

-- name: GetTotalJobCountForNode :one
SELECT COUNT(*) FROM jobs WHERE node = ? AND deleted = 0;

-- name: SoftDeleteJob :exec
UPDATE jobs SET deleted = ? WHERE job = ?;

-- name: SetErrorWhereJobId :exec
UPDATE jobs
SET error = ?, status = 'error'
WHERE jobs.job = sqlc.arg('job_id') AND jobs.project=sqlc.arg('project') AND jobs.node=sqlc.arg('node');

-- name: SetStoppedByOnJob :exec
UPDATE jobs SET stopped_by=? WHERE job=? AND project=? AND node=?;

-- name: SearchNodeJobs :many
SELECT j.id, j.project, j.spider, j.job, j.status, j.deleted, j.create_time, j.update_time, j.pages, j.items, j.pid,
       j.start, j.runtime, j.finish, j.href_log, j.href_items, j.node, j.error, u1.username AS started_by_username,
       u2.username AS stopped_by_username
FROM jobs j
         LEFT JOIN users u1 ON j.started_by = u1.ID
         LEFT JOIN users u2 ON j.stopped_by = u2.ID
WHERE
    (LOWER(j.spider) LIKE '%' || LOWER(@search_term) || '%' OR
     LOWER(j.job) LIKE '%' || LOWER(@search_term) || '%')
  AND j.node = @node
  AND j.deleted = 0
ORDER BY CASE
             WHEN j.finish IS NULL THEN j.runtime
             ELSE j.finish
             END DESC;
