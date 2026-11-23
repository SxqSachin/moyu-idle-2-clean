#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
UPDATE_SCRIPT="${ROOT_DIR}/infra/scripts/update-linux-native-split.sh"

bash -n "${UPDATE_SCRIPT}"

wait_function="$(sed -n '/^wait_for_server_ready() {$/,/^}$/p' "${UPDATE_SCRIPT}")"
if [[ -z "${wait_function}" ]]; then
  echo "wait_for_server_ready function is missing" >&2
  exit 1
fi
eval "${wait_function}"
restart_function="$(sed -n '/^restart_server() {$/,/^}$/p' "${UPDATE_SCRIPT}")"
if [[ -z "${restart_function}" ]]; then
  echo "restart_server function is missing" >&2
  exit 1
fi
eval "${restart_function}"

TARGET="server"
APP_PORT="8080"
SERVER_READY_MAX_ATTEMPTS="3"
curl_attempts=0
sleep_attempts=0

log_step() {
  :
}

curl() {
  curl_attempts=$((curl_attempts + 1))
  ((curl_attempts >= 3))
}

sleep() {
  sleep_attempts=$((sleep_attempts + 1))
}

wait_for_server_ready

if [[ "${curl_attempts}" -ne 3 ]]; then
  echo "readiness probe did not retry until success: attempts=${curl_attempts}" >&2
  exit 1
fi
if [[ "${sleep_attempts}" -ne 2 ]]; then
  echo "readiness probe used an unexpected retry delay count: sleeps=${sleep_attempts}" >&2
  exit 1
fi

SERVER_READY_MAX_ATTEMPTS="0"
if wait_for_server_ready >/dev/null 2>&1; then
  echo "readiness probe accepted an invalid attempt count" >&2
  exit 1
fi

SERVER_SERVICE="mysecondwebgame-server"
systemctl() {
  if [[ "${1:-}" == "restart" ]]; then
    return 1
  fi
  return 0
}

if restart_output="$(restart_server 2>&1)"; then
  echo "restart_server accepted a failed systemctl restart" >&2
  exit 1
fi
if [[ "${restart_output}" != *"Service restart command failed: ${SERVER_SERVICE}"* ]]; then
  echo "restart_server did not explain the systemctl failure: ${restart_output}" >&2
  exit 1
fi

echo "update-linux-native-split test: PASS"
