#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

SQL_PATH="${1:-server/schema/init.sql}"

PGHOST="${PGHOST:-}"
PGPORT="${PGPORT:-5432}"
PGSSLMODE="${PGSSLMODE:-disable}"
PGADMIN_USER="${PGADMIN_USER:-postgres}"
PGADMIN_PASSWORD="${PGADMIN_PASSWORD:-}"
PGADMIN_DB="${PGADMIN_DB:-postgres}"
PGAPP_USER="${PGAPP_USER:-game}"
PGAPP_PASSWORD="${PGAPP_PASSWORD:-}"
PGAPP_DB="${PGAPP_DB:-game}"

if [[ -z "$PGHOST" || -z "$PGADMIN_PASSWORD" || -z "$PGAPP_PASSWORD" ]]; then
  cat <<'EOF'
Usage:
  PGHOST=<pg_host> PGADMIN_PASSWORD=<admin_password> PGAPP_PASSWORD=<app_password> \
    bash infra/scripts/init-prod-pg-remote.sh [sql-file]

Optional env vars:
  PGPORT=5432
  PGSSLMODE=disable
  PGADMIN_USER=postgres
  PGADMIN_DB=postgres
  PGAPP_USER=game
  PGAPP_DB=game

Example:
  PGHOST=192.0.2.31 PGADMIN_PASSWORD='***' PGAPP_PASSWORD='***' \
    bash infra/scripts/init-prod-pg-remote.sh
EOF
  exit 1
fi

if [[ ! -f "$SQL_PATH" ]]; then
  echo "schema sql not found: $SQL_PATH"
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "psql not found. install postgresql-client first."
  exit 1
fi

ADMIN_CONN=(psql -v ON_ERROR_STOP=1 -h "$PGHOST" -p "$PGPORT" -U "$PGADMIN_USER" -d "$PGADMIN_DB")
APP_CONN=(psql -v ON_ERROR_STOP=1 -h "$PGHOST" -p "$PGPORT" -U "$PGAPP_USER" -d "$PGAPP_DB")

echo "Ensuring role and database exist on ${PGHOST}:${PGPORT}..."
PGPASSWORD="$PGADMIN_PASSWORD" "${ADMIN_CONN[@]}" -v app_user="$PGAPP_USER" -v app_password="$PGAPP_PASSWORD" -v app_db="$PGAPP_DB" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'app_user', :'app_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_user') \gexec

SELECT format('ALTER ROLE %I WITH PASSWORD %L', :'app_user', :'app_password')
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_user') \gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'app_db', :'app_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'app_db') \gexec
SQL

echo "Applying schema SQL: $SQL_PATH"
PGPASSWORD="$PGAPP_PASSWORD" "${APP_CONN[@]}" -f "$SQL_PATH" >/dev/null
echo "Remote PG initialization completed."
