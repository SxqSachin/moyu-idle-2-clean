#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="infra/docker-compose.yml"
SQL_PATH="${1:-server/schema/migrations/20260514_user_settings_language_expand.sql}"
PSQL_CMD=(docker compose -f "$COMPOSE_FILE" exec -T pg psql -v ON_ERROR_STOP=1 -U game -d game)

if [[ ! -f "$SQL_PATH" ]]; then
  echo "migration sql not found: $SQL_PATH"
  exit 1
fi

if ! docker compose -f "$COMPOSE_FILE" ps pg >/dev/null 2>&1; then
  echo "postgres container not found. run: docker compose -f infra/docker-compose.yml up -d"
  exit 1
fi

echo "Applying migration SQL: $SQL_PATH"
cat "$SQL_PATH" | "${PSQL_CMD[@]}"
echo "Migration applied."
