#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="infra/docker-compose.yml"
INIT_SQL="${1:-server/schema/init.sql}"
PSQL_CHECK_CMD=(docker compose -f "$COMPOSE_FILE" exec -T pg psql -v ON_ERROR_STOP=1 -U game -d postgres)

if ! docker compose -f "$COMPOSE_FILE" ps pg >/dev/null 2>&1; then
  echo "postgres container not found. run: docker compose -f infra/docker-compose.yml up -d"
  exit 1
fi

db_exists="$("${PSQL_CHECK_CMD[@]}" -Atc "SELECT 1 FROM pg_database WHERE datname = 'game' LIMIT 1;" || true)"
if [[ "$db_exists" == "1" ]]; then
  echo "database check: game exists, applying schema updates."
else
  echo "database check: game does not exist yet, init.sql will create it."
fi

echo "apply baseline init: $INIT_SQL"
bash infra/scripts/apply-schema-sql.sh "$INIT_SQL"
echo "baseline init applied."
