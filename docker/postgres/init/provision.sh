#!/usr/bin/env bash
# Runs once, on the compose Postgres's first start: provisions the othra roles with the dev-only passwords from compose.yaml.
set -euo pipefail

ADMIN_URL="postgresql://${POSTGRES_USER}@/?host=/var/run/postgresql" \
  DB_NAME="${POSTGRES_DB}" PREFIX=othra SCHEMA=public \
  MIG_PW="${OTHRA_DB_MIGRATOR_PASSWORD:?set OTHRA_DB_MIGRATOR_PASSWORD}" \
  SVC_PW="${OTHRA_DB_SERVICE_PASSWORD:?set OTHRA_DB_SERVICE_PASSWORD}" \
  /othra/scripts/db/provision.sh
