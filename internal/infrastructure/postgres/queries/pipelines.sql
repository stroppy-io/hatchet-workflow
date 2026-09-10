-- name: PipelinePushes :many
SELECT namespace, revision, status, error, pushed_at, updated_at FROM pipeline_pushes ORDER BY namespace;

-- name: PipelinePush :one
SELECT namespace, revision, status, error, pushed_at, updated_at FROM pipeline_pushes WHERE namespace = @namespace;

-- name: SetPipelinePush :exec
INSERT INTO pipeline_pushes (namespace, revision, status, error, pushed_at, updated_at)
VALUES (@namespace, @revision, @status, @error, @pushed_at, now())
ON CONFLICT (namespace) DO UPDATE SET revision = EXCLUDED.revision, status = EXCLUDED.status, error = EXCLUDED.error, pushed_at = COALESCE(EXCLUDED.pushed_at, pipeline_pushes.pushed_at), updated_at = now();

-- name: DeletePipelinePush :exec
DELETE FROM pipeline_pushes WHERE namespace = @namespace;
