#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="infra/docker-compose.yml"
PSQL_CMD=(docker compose -f "$COMPOSE_FILE" exec -T pg psql -v ON_ERROR_STOP=1 -U game -d game)

if [[ $# -gt 0 ]]; then
  SQL_PATHS=("$1")
else
  SQL_PATHS=(
    server/schema/migrations/20260814_player_custom_skill_forge.sql
    server/schema/migrations/20260814_bound_skillbook_forge.sql
    server/schema/migrations/20260814_skill_forge_book_count_config.sql
    server/schema/migrations/20260815_skill_forge_ruleset_version.sql
    server/schema/migrations/20260815_skill_forge_state_bounds.sql
  )
fi

for sql_path in "${SQL_PATHS[@]}"; do
  if [[ ! -f "$sql_path" ]]; then
    echo "migration sql not found: $sql_path"
    exit 1
  fi
done

if ! docker compose -f "$COMPOSE_FILE" ps pg >/dev/null 2>&1; then
  echo "postgres container not found. run: docker compose -f infra/docker-compose.yml up -d"
  exit 1
fi

for sql_path in "${SQL_PATHS[@]}"; do
  echo "Applying migration SQL: $sql_path"
  "${PSQL_CMD[@]}" < "$sql_path"
done
echo "Skill forge migrations applied."
