#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

SQL_INPUT="${1:-}"
COMPOSE_FILE="${2:-infra/docker-compose.yml}"
ENV_FILE="${3:-}"

DOCKER_COMPOSE_CMD=(docker compose)
if [[ -n "$ENV_FILE" ]]; then
  DOCKER_COMPOSE_CMD+=(--env-file "$ENV_FILE")
fi
DOCKER_COMPOSE_CMD+=(-f "$COMPOSE_FILE")

if [[ -z "$SQL_INPUT" ]]; then
  echo "Usage: bash infra/scripts/apply-schema-sql.sh <sql-file-or-name> [compose-file] [env-file]"
  echo "Examples:"
  echo "  bash infra/scripts/apply-schema-sql.sh init.sql"
  echo "  bash infra/scripts/apply-schema-sql.sh init.sql infra/compose/pg.yml infra/.env.prod"
  exit 1
fi

if [[ "$SQL_INPUT" = /* || "$SQL_INPUT" == *"/"* ]]; then
  SQL_PATH="$SQL_INPUT"
else
  SQL_PATH="server/schema/$SQL_INPUT"
fi

if [[ ! -f "$SQL_PATH" ]]; then
  echo "schema sql not found: $SQL_PATH"
  exit 1
fi
SQL_ABS_PATH="$(cd "$(dirname "$SQL_PATH")" && pwd -P)/$(basename "$SQL_PATH")"
MIGRATIONS_DIR="$ROOT_DIR/server/schema/migrations"

if ! "${DOCKER_COMPOSE_CMD[@]}" ps pg >/dev/null 2>&1; then
  echo "postgres container not found. run: docker compose -f $COMPOSE_FILE up -d"
  exit 1
fi

if [[ "$SQL_ABS_PATH" == "$MIGRATIONS_DIR/"* ]]; then
  TARGET_DATABASE="$("${DOCKER_COMPOSE_CMD[@]}" exec -T pg sh -lc 'printf "%s" "${POSTGRES_DB:-game}"')"
  TARGET_DATABASE="${TARGET_DATABASE:-game}"
  PSQL_CMD=("${DOCKER_COMPOSE_CMD[@]}" exec -T pg sh -lc 'PGPASSWORD="${POSTGRES_PASSWORD:-}" psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-game}" -d "${POSTGRES_DB:-game}"')
else
  TARGET_DATABASE="postgres"
  PSQL_CMD=("${DOCKER_COMPOSE_CMD[@]}" exec -T pg sh -lc 'PGPASSWORD="${POSTGRES_PASSWORD:-}" psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-game}" -d postgres')
fi

echo "Applying schema SQL: $SQL_PATH (target database: $TARGET_DATABASE)"
cat "$SQL_PATH" | "${PSQL_CMD[@]}" >/dev/null
echo "Schema SQL applied to database: $TARGET_DATABASE"
