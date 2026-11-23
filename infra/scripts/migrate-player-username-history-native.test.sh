#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT_DIR}/infra/scripts/migrate-player-username-history-native.sh"
UPDATE_SCRIPT="${ROOT_DIR}/infra/scripts/update-linux-native-split.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

MOCK_BIN="${TMP_DIR}/bin"
MOCK_LOG="${TMP_DIR}/psql.log"
APP_DIR="${TMP_DIR}/app"
ENV_FILE="${TMP_DIR}/server.env"
mkdir -p "${MOCK_BIN}" "${APP_DIR}/server/schema/migrations"
ln -s \
  "${ROOT_DIR}/server/schema/migrations/20260827_player_username_changes.sql" \
  "${APP_DIR}/server/schema/migrations/20260827_player_username_changes.sql"
printf '%s\n' \
  'DATABASE_URL=postgres://game:secret@db:5432/game?sslmode=disable&pool_max_conns=20&application_name=moyu&pool_min_conns=2' \
  > "${ENV_FILE}"

cat > "${MOCK_BIN}/psql" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${MOCK_PSQL_LOG}"
MOCK
chmod +x "${MOCK_BIN}/psql"

bash -n "${SCRIPT}" "${UPDATE_SCRIPT}"
PATH="${MOCK_BIN}:${PATH}" MOCK_PSQL_LOG="${MOCK_LOG}" \
  bash "${SCRIPT}" "${APP_DIR}" "${ENV_FILE}" >/dev/null

psql_args="$(<"${MOCK_LOG}")"
if [[ "${psql_args}" != *'postgres://game:secret@db:5432/game?sslmode=disable&application_name=moyu'* ]]; then
  echo "psql did not receive the normalized DATABASE_URL: ${psql_args}" >&2
  exit 1
fi
if [[ "${psql_args}" == *'pool_'* ]]; then
  echo "psql received unsupported pgx pool parameters: ${psql_args}" >&2
  exit 1
fi
if [[ "${psql_args}" != *'--single-transaction'* || "${psql_args}" != *'20260827_player_username_changes.sql'* ]]; then
  echo "psql did not receive the migration transaction arguments: ${psql_args}" >&2
  exit 1
fi

schema_call_count="$(grep -Ec '^[[:space:]]+apply_player_username_history_schema$' "${UPDATE_SCRIPT}")"
if [[ "${schema_call_count}" -ne 3 ]]; then
  echo "username history migration must run in exactly three backend update modes; found ${schema_call_count}" >&2
  exit 1
fi
specialization_block="$(sed -n '/^apply_player_specialization_schema()/,/^}/p' "${UPDATE_SCRIPT}")"
if [[ "${specialization_block}" != *'ensure_cmd_or_install go golang-go'* ]]; then
  echo "specialization migration must ensure Go before it runs" >&2
  exit 1
fi

echo "migrate-player-username-history-native test: PASS"
