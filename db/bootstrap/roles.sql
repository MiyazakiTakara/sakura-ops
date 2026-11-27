-- One-time (idempotent) bootstrap run by a PostgreSQL superuser against the
-- target database, before the first migration. Creates the two login roles and
-- the "sakuraops" schema. It contains no passwords.
--
--   psql -v ON_ERROR_STOP=1 -d <database> \
--        -v migrator_password=... -v app_password=... -f db/bootstrap/roles.sql
--
-- If a password variable is not set, it is read from the file named by
-- SAKURAOPS_MIGRATOR_PASSWORD_FILE / SAKURAOPS_APP_PASSWORD_FILE (preferred: the
-- value then never appears on a command line).
--
-- Roles:
--   sakuraops_migrator  owns the schema and all objects, runs migrations, the only
--                       role that can CREATE/ALTER and write the goose table.
--   sakuraops_app       the hub's runtime role: USAGE on the schema plus the exact
--                       grants each migration gives it; no DDL, no goose table access.
\set ON_ERROR_STOP on

\if :{?migrator_password}
\else
  \set migrator_password `cat "$SAKURAOPS_MIGRATOR_PASSWORD_FILE"`
\endif
\if :{?app_password}
\else
  \set app_password `cat "$SAKURAOPS_APP_PASSWORD_FILE"`
\endif

-- Refuse empty passwords (a missing file leaves the variable empty).
SELECT (length(:'migrator_password') = 0 OR length(:'app_password') = 0) AS pw_empty \gset
\if :pw_empty
  DO $$ BEGIN RAISE EXCEPTION 'bootstrap: migrator and app passwords must be non-empty'; END $$;
\endif

SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS', r)
FROM (VALUES ('sakuraops_migrator'), ('sakuraops_app')) AS t(r)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) \gexec

-- Refuse to continue if either role is a member (direct or inherited) of a
-- privileged or predefined role, or if the two roles are members of each other.
-- Memberships are never revoked silently: the operator must look at them.
DO $$
DECLARE bad text;
BEGIN
  WITH RECURSIVE m(member, roleid) AS (
    SELECT am.member, am.roleid FROM pg_auth_members am
    WHERE am.member IN (SELECT oid FROM pg_roles WHERE rolname IN ('sakuraops_migrator', 'sakuraops_app'))
    UNION
    SELECT m.member, am.roleid FROM m JOIN pg_auth_members am ON am.member = m.roleid
  )
  SELECT string_agg(DISTINCT format('%s is a member of %s', mr.rolname, gr.rolname), '; ') INTO bad
  FROM m
  JOIN pg_roles mr ON mr.oid = m.member
  JOIN pg_roles gr ON gr.oid = m.roleid
  WHERE gr.rolsuper OR gr.rolcreaterole OR gr.rolcreatedb OR gr.rolreplication OR gr.rolbypassrls
     OR gr.rolname LIKE 'pg\_%'
     OR gr.rolname IN ('sakuraops_migrator', 'sakuraops_app');
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'bootstrap: refusing to run, unexpected role membership (%): revoke it manually', bad;
  END IF;
END $$;

-- Harden roles that already existed: reset every privilege attribute on each run.
-- A superuser that is the bootstrap's own session user is left alone.
SELECT format('ALTER ROLE %I NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS LOGIN', rolname)
FROM pg_roles
WHERE rolname IN ('sakuraops_migrator', 'sakuraops_app')
  AND NOT (rolsuper AND rolname = session_user) \gexec

-- (Re)set passwords; psql quotes the variables as literals.
ALTER ROLE sakuraops_migrator WITH LOGIN PASSWORD :'migrator_password';
ALTER ROLE sakuraops_app WITH LOGIN PASSWORD :'app_password';

-- Nothing for PUBLIC: only the two roles may connect, and nobody creates in public.
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', current_database()) \gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO sakuraops_migrator, sakuraops_app', current_database()) \gexec
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

CREATE SCHEMA IF NOT EXISTS sakuraops AUTHORIZATION sakuraops_migrator;
REVOKE ALL ON SCHEMA sakuraops FROM PUBLIC;
GRANT USAGE ON SCHEMA sakuraops TO sakuraops_app;
