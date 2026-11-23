#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT_DIR}/infra/scripts/apply-schema-sql.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

MOCK_BIN="${TMP_DIR}/bin"
MOCK_LOG="${TMP_DIR}/docker.log"
mkdir -p "${MOCK_BIN}"
cat > "${MOCK_BIN}/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >> "${MOCK_DOCKER_LOG}"
last_arg="${!#}"
if [[ "${last_arg}" == 'printf "%s" "${POSTGRES_DB:-game}"' ]]; then
  printf '%s' 'mock_game'
elif [[ "${last_arg}" == *'psql -v ON_ERROR_STOP=1'* ]]; then
  while IFS= read -r _line; do
    :
  done
fi
MOCK
chmod +x "${MOCK_BIN}/docker"

assert_contains() {
  local text="$1"
  local expected="$2"
  if [[ "${text}" != *"${expected}"* ]]; then
    echo "Expected output to contain: ${expected}" >&2
    echo "Actual output: ${text}" >&2
    exit 1
  fi
}

bash -n "${SCRIPT}"

: > "${MOCK_LOG}"
init_output="$(
  PATH="${MOCK_BIN}:${PATH}" MOCK_DOCKER_LOG="${MOCK_LOG}" \
    bash "${SCRIPT}" init.sql "${ROOT_DIR}/infra/docker-compose.yml"
)"
init_log="$(<"${MOCK_LOG}")"
assert_contains "${init_output}" "target database: postgres"
assert_contains "${init_log}" '-d postgres'

: > "${MOCK_LOG}"
touch "${TMP_DIR}/compose.env"
migration_output="$(
  PATH="${MOCK_BIN}:${PATH}" MOCK_DOCKER_LOG="${MOCK_LOG}" \
    bash "${SCRIPT}" \
      server/schema/migrations/20260827_player_username_changes.sql \
      "${ROOT_DIR}/infra/docker-compose.yml" \
      "${TMP_DIR}/compose.env"
)"
migration_log="$(<"${MOCK_LOG}")"
assert_contains "${migration_output}" "target database: mock_game"
assert_contains "${migration_log}" "--env-file ${TMP_DIR}/compose.env"
assert_contains "${migration_log}" '-d "${POSTGRES_DB:-game}"'

echo "apply-schema-sql test: PASS"
