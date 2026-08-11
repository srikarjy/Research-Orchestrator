#!/bin/sh
# Runs once, on first init, via docker-entrypoint-initdb.d in the shared
# postgres container. Creates a second database + role for the standalone
# workflow-Engine service (its own schema, migrated separately by
# cmd/migrate — see the workflow-engine-migrate compose service) inside the
# same Postgres instance orchestrator uses, so both services share one
# Postgres per the milestone-gate decision instead of two isolated ones.
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE USER workflow WITH PASSWORD 'workflow';
    CREATE DATABASE workflow OWNER workflow;
EOSQL
