-- +goose Up
-- Baseline schema (NEXT-004). Forward-only: there is deliberately no Down section.
-- Rollback is restore from backup (docs/07-operations.md, docs/dev/database.md).
-- The schema "sakuraops" and the roles are created by db/bootstrap/roles.sql
-- (run once by a superuser); this migration runs as sakuraops_migrator.

CREATE TABLE sakuraops.instance_settings (
    id            boolean     PRIMARY KEY DEFAULT true CHECK (id),
    instance_name text        NOT NULL CHECK (char_length(instance_name) BETWEEN 1 AND 100),
    timezone      text        NOT NULL DEFAULT 'UTC' CHECK (char_length(timezone) BETWEEN 1 AND 64),
    revision      bigint      NOT NULL DEFAULT 1 CHECK (revision >= 1),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE sakuraops.instance_settings IS
    'Singleton row (id is always true) with typed instance configuration; see docs/22-data-model.md.';

INSERT INTO sakuraops.instance_settings (instance_name) VALUES ('SakuraOps');

-- The application role gets DML on exactly this table and no DDL.
-- Every later migration must grant explicitly; there are no default privileges.
GRANT SELECT, UPDATE ON sakuraops.instance_settings TO sakuraops_app;

-- The application must know which schema version it runs against without being
-- able to read or write the goose bookkeeping table, so it gets this function
-- only. SECURITY DEFINER runs as the owner (sakuraops_migrator).
-- +goose StatementBegin
CREATE FUNCTION sakuraops.schema_version() RETURNS bigint
    LANGUAGE sql STABLE SECURITY DEFINER
    SET search_path = pg_catalog, pg_temp
AS $$
    SELECT COALESCE(max(version_id), 0)::bigint FROM sakuraops.goose_db_version WHERE is_applied
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION sakuraops.schema_version() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION sakuraops.schema_version() TO sakuraops_app;
