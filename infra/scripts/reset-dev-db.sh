#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="infra/docker-compose.yml"
PSQL_CMD=(docker compose -f "$COMPOSE_FILE" exec -T pg psql -v ON_ERROR_STOP=1 -U game -d postgres)
CONFIRM="${1:-}"

if [[ "$CONFIRM" != "--yes" ]]; then
  echo "Refusing to run without confirmation."
  echo "Usage: bash infra/scripts/reset-dev-db.sh --yes"
  exit 1
fi

if [[ -f "server/.env" ]]; then
  app_env="$(grep -E '^APP_ENV=' server/.env | tail -n1 | cut -d'=' -f2- || true)"
  app_env="${app_env//\"/}"
  app_env="${app_env//\'/}"
  if [[ -n "$app_env" && "$app_env" != "development" && "$app_env" != "local" ]]; then
    echo "Refusing to run: APP_ENV=$app_env (expected development/local)."
    exit 1
  fi
fi

if ! docker compose -f "$COMPOSE_FILE" ps pg >/dev/null 2>&1; then
  echo "postgres container not found. run: docker compose -f infra/docker-compose.yml up -d"
  exit 1
fi

echo "Resetting development database..."
"${PSQL_CMD[@]}" <<'SQL'
DROP DATABASE IF EXISTS game;
CREATE DATABASE game OWNER game;
SQL

echo "Applying baseline init schema..."
bash infra/scripts/init-db-local.sh

echo "Development database reset complete."
