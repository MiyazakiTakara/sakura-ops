-- sqlc-only. Describes what db/bootstrap/roles.sql and goose create outside the
-- migration files, so sqlc can resolve the schema. Never applied to a database.
CREATE SCHEMA sakuraops;

CREATE TABLE sakuraops.goose_db_version (
    id         serial PRIMARY KEY,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp     timestamp NOT NULL DEFAULT now()
);
