-- name: GetInstanceSettings :one
SELECT instance_name, timezone, revision, updated_at
FROM sakuraops.instance_settings
WHERE id;

-- name: SchemaVersion :one
SELECT sakuraops.schema_version()::bigint AS version;
