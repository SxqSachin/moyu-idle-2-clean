#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${1:-/opt/moyu-idle}"
ENV_FILE="${2:-/etc/mysecondwebgame/server.env}"

if ! command -v go >/dev/null 2>&1; then
  echo "Missing command: go"
  exit 1
fi
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Missing server environment file: ${ENV_FILE}"
  exit 1
fi

database_url="$(grep -E '^DATABASE_URL=' "${ENV_FILE}" | head -n1 | cut -d= -f2- || true)"
if [[ -z "${database_url}" ]]; then
  echo "DATABASE_URL is missing in ${ENV_FILE}"
  exit 1
fi

migrations=(
  "${APP_DIR}/server/schema/migrations/20260826_player_specializations.sql"
  "${APP_DIR}/server/schema/migrations/20260828_battle_attr_exp_jobs.sql"
)
for migration in "${migrations[@]}"; do
  if [[ ! -f "${migration}" ]]; then
    echo "Missing runtime compatibility schema: ${migration}"
    exit 1
  fi
done

echo "Applying specialization and battle attribute reward schema migrations"
(
  cd "${APP_DIR}/server"
  DATABASE_URL="${database_url}" go run ./cmd/schema_apply "${migrations[@]}"
)
echo "Specialization and battle attribute reward schema migrations applied"
