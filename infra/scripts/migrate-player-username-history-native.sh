#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${1:-/opt/moyu-idle}"
ENV_FILE="${2:-/etc/mysecondwebgame/server.env}"

if ! command -v psql >/dev/null 2>&1; then
  echo "Missing command: psql (install postgresql-client before applying player username history migration)."
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

# pgx pool_* query parameters are application settings, not PostgreSQL
# connection parameters accepted by psql.
strip_pool_query_params() {
  local url="$1"
  local base query rebuilt part key
  if [[ "${url}" != *"?"* ]]; then
    printf '%s' "${url}"
    return
  fi
  base="${url%%\?*}"
  query="${url#*\?}"
  rebuilt=""
  IFS='&' read -ra parts <<< "${query}"
  for part in "${parts[@]}"; do
    [[ -z "${part}" ]] && continue
    key="${part%%=*}"
    [[ "${key}" == pool_* ]] && continue
    if [[ -n "${rebuilt}" ]]; then
      rebuilt="${rebuilt}&"
    fi
    rebuilt="${rebuilt}${part}"
  done
  if [[ -n "${rebuilt}" ]]; then
    printf '%s?%s' "${base}" "${rebuilt}"
  else
    printf '%s' "${base}"
  fi
}

database_url="$(strip_pool_query_params "${database_url}")"
migration="${APP_DIR}/server/schema/migrations/20260827_player_username_changes.sql"
if [[ ! -f "${migration}" ]]; then
  echo "Missing player username history migration: ${migration}"
  exit 1
fi

echo "Applying player username history schema migration"
psql "${database_url}" -v ON_ERROR_STOP=1 --single-transaction -f "${migration}"
echo "Player username history schema migration applied"
