#!/usr/bin/env bash
set -euo pipefail

# Fast update script for existing native-split deployments.
# Targets:
# - server: rebuild + restart mysecondwebgame-server + mysecondwebgame-action-worker
# - action-worker: rebuild + restart mysecondwebgame-action-worker
# - battle-worker: rebuild + restart mysecondwebgame-battle-worker
# - kitty-worker: rebuild + restart mysecondwebgame-kitty-worker
# - server-with-kitty-worker: rebuild + restart server + action-worker + kitty-worker
# - backend: rebuild + restart server + action-worker + battle-worker + kitty-worker
# - web: git pull + npm build only (no nginx restart)
# - all: backend + web

TARGET="${1:-}"
APP_DIR="${2:-/opt/moyu-idle}"
BRANCH="${BRANCH:-main}"
APP_USER="${APP_USER:-mysecondwebgame}"
APP_GROUP="${APP_GROUP:-mysecondwebgame}"
APP_PORT="${APP_PORT:-8080}"
SERVER_SERVICE="${SERVER_SERVICE:-mysecondwebgame-server}"
BATTLE_WORKER_SERVICE="${BATTLE_WORKER_SERVICE:-mysecondwebgame-battle-worker}"
ACTION_WORKER_SERVICE="${ACTION_WORKER_SERVICE:-mysecondwebgame-action-worker}"
KITTY_WORKER_SERVICE="${KITTY_WORKER_SERVICE:-mysecondwebgame-kitty-worker}"
INSTALL_DEPS="${INSTALL_DEPS:-0}"
WEB_NODE_REQUIRED_MAJOR="${WEB_NODE_REQUIRED_MAJOR:-22}"
WEB_NODE_REQUIRED_MINOR="${WEB_NODE_REQUIRED_MINOR:-14}"
ACTION_WORKER_DB_POOL_MAX_CONNS="${ACTION_WORKER_DB_POOL_MAX_CONNS:-4}"
SERVER_READY_MAX_ATTEMPTS="${SERVER_READY_MAX_ATTEMPTS:-30}"

if [[ -z "${TARGET}" ]]; then
  echo "Usage: bash infra/scripts/update-linux-native-split.sh <target> [app_dir]"
  echo "target: server | action-worker | battle-worker | kitty-worker | server-with-kitty-worker | backend | web | all"
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root."
  exit 1
fi

log_step() {
  echo "[native-update][${TARGET}] $*"
}

upsert_url_query_param() {
  local url="$1"
  local key="$2"
  local value="$3"
  local base query rebuilt part current_key current_value
  if [[ "${url}" == *"?"* ]]; then
    base="${url%%\?*}"
    query="${url#*\?}"
  else
    base="${url}"
    query=""
  fi
  rebuilt=""
  IFS='&' read -ra parts <<< "${query}"
  for part in "${parts[@]}"; do
    [[ -z "${part}" ]] && continue
    current_key="${part%%=*}"
    current_value="${part#*=}"
    if [[ "${current_key}" == "${key}" ]]; then
      continue
    fi
    if [[ -n "${rebuilt}" ]]; then
      rebuilt="${rebuilt}&"
    fi
    if [[ "${part}" == "${current_key}" ]]; then
      rebuilt="${rebuilt}${current_key}"
    else
      rebuilt="${rebuilt}${current_key}=${current_value}"
    fi
  done
  if [[ -n "${rebuilt}" ]]; then
    rebuilt="${rebuilt}&"
  fi
  rebuilt="${rebuilt}${key}=${value}"
  echo "${base}?${rebuilt}"
}

apt_install() {
  if [[ "${INSTALL_DEPS}" != "1" ]]; then
    return 0
  fi
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  apt-get install -y "$@"
}

ensure_cmd_or_install() {
  local cmd="$1"
  shift
  if command -v "${cmd}" >/dev/null 2>&1; then
    return 0
  fi
  if [[ "${INSTALL_DEPS}" != "1" ]]; then
    echo "Missing command: ${cmd}. Set INSTALL_DEPS=1 for auto install."
    exit 1
  fi
  apt_install "$@"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Dependency install did not provide command: ${cmd}"
    exit 1
  fi
}

ensure_app_user() {
  if ! getent group "${APP_GROUP}" >/dev/null; then
    groupadd --system "${APP_GROUP}"
  fi
  if ! id -u "${APP_USER}" >/dev/null 2>&1; then
    useradd --system --gid "${APP_GROUP}" --create-home --home-dir "/home/${APP_USER}" "${APP_USER}"
  fi
}

prepare_repo() {
  ensure_cmd_or_install git git ca-certificates
  if [[ ! -d "${APP_DIR}/.git" ]]; then
    echo "Repository not found at ${APP_DIR}. Run native split deploy first."
    exit 1
  fi
  ensure_app_user
  chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
  log_step "Updating repository (${BRANCH})"
  git -C "${APP_DIR}" fetch --all --prune
  git -C "${APP_DIR}" checkout "${BRANCH}"
  git -C "${APP_DIR}" pull --ff-only origin "${BRANCH}"
}

ensure_web_node_version() {
  ensure_cmd_or_install node nodejs
  ensure_cmd_or_install npm npm
  local node_ver node_major node_minor
  node_ver="$(node -v | sed -E 's/^v//')"
  node_major="$(echo "${node_ver}" | cut -d. -f1)"
  node_minor="$(echo "${node_ver}" | cut -d. -f2)"
  if [[ "${node_major}" != "${WEB_NODE_REQUIRED_MAJOR}" ]] || [[ "${node_minor}" -lt "${WEB_NODE_REQUIRED_MINOR}" ]]; then
    echo "Node.js v${WEB_NODE_REQUIRED_MAJOR}.${WEB_NODE_REQUIRED_MINOR}+ is required, current: v${node_ver}"
    exit 1
  fi
}

build_server_binary() {
  ensure_cmd_or_install go golang-go
  log_step "Building server binary"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go build -o '${APP_DIR}/bin/mysecondwebgame-server' ./cmd/server"
}

build_action_worker_binary() {
  ensure_cmd_or_install go golang-go
  log_step "Building action-worker binary"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go build -o '${APP_DIR}/bin/mysecondwebgame-action-worker' ./cmd/action_worker"
}

build_battle_worker_binary() {
  ensure_cmd_or_install go golang-go
  log_step "Building battle-worker binary"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go build -o '${APP_DIR}/bin/mysecondwebgame-battle-worker' ./cmd/battle_worker"
}

build_kitty_worker_binary() {
  ensure_cmd_or_install go golang-go
  log_step "Building kitty-worker binary"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go build -o '${APP_DIR}/bin/mysecondwebgame-kitty-worker' ./cmd/kitty_worker"
}

replace_env_value() {
  local env_file="$1"
  local key="$2"
  local value="$3"
  local tmp_file
  tmp_file="$(mktemp)"
  awk -v key="${key}" -v newline="${key}=${value}" '
    BEGIN { replaced = 0 }
    index($0, key "=") == 1 {
      print newline
      replaced = 1
      next
    }
    { print }
    END {
      if (replaced == 0) {
        print newline
      }
    }
  ' "${env_file}" > "${tmp_file}"
  install -m 600 "${tmp_file}" "${env_file}"
  rm -f "${tmp_file}"
}

ensure_action_worker_env_file() {
  local env_dir="/etc/mysecondwebgame"
  local action_env="${env_dir}/action-worker.env"
  local server_env="${env_dir}/server.env"
  mkdir -p "${env_dir}"
  if [[ ! -f "${action_env}" ]]; then
    if [[ ! -f "${server_env}" ]]; then
      echo "Missing ${action_env} and ${server_env}; run deploy-linux-native-split.sh action-worker once with backend endpoint env."
      exit 1
    fi
    install -m 600 "${server_env}" "${action_env}"
  fi
  local current_url tuned_url
  current_url="$(grep -E '^DATABASE_URL=' "${action_env}" | head -n1 | cut -d= -f2- || true)"
  if [[ -n "${current_url}" ]]; then
    tuned_url="$(upsert_url_query_param "${current_url}" "pool_max_conns" "${ACTION_WORKER_DB_POOL_MAX_CONNS}")"
    replace_env_value "${action_env}" "DATABASE_URL" "${tuned_url}"
  fi
  log_step "Applied action-worker DB pool max=${ACTION_WORKER_DB_POOL_MAX_CONNS}"
}

write_action_worker_service_unit() {
  cat >/etc/systemd/system/${ACTION_WORKER_SERVICE}.service <<EOF
[Unit]
Description=MySecondWebGame Action Worker
After=network.target postgresql.service redis-server.service nats-server.service
Wants=postgresql.service redis-server.service nats-server.service

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${APP_DIR}/server
EnvironmentFile=/etc/mysecondwebgame/action-worker.env
ExecStart=${APP_DIR}/bin/mysecondwebgame-action-worker
Restart=always
RestartSec=3
LimitNOFILE=65535
MemoryMax=2G
TasksMax=512
ExecStopPost=/bin/sh -c 'journalctl -u ${ACTION_WORKER_SERVICE} -n 200 --no-pager > /tmp/mysecondwebgame-action-worker-last-stop.log || true'

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable "${ACTION_WORKER_SERVICE}"
}

ensure_action_worker_service() {
  ensure_action_worker_env_file
  if systemctl cat "${ACTION_WORKER_SERVICE}" >/dev/null 2>&1; then
    return 0
  fi
  log_step "Action-worker service missing; installing systemd unit before update"
  write_action_worker_service_unit
}

wait_for_server_ready() {
  local ready_url="http://127.0.0.1:${APP_PORT}/readyz"
  local attempt
  if [[ ! "${SERVER_READY_MAX_ATTEMPTS}" =~ ^[1-9][0-9]*$ ]]; then
    echo "SERVER_READY_MAX_ATTEMPTS must be a positive integer: ${SERVER_READY_MAX_ATTEMPTS}"
    return 1
  fi
  for ((attempt = 1; attempt <= SERVER_READY_MAX_ATTEMPTS; attempt++)); do
    if curl --connect-timeout 1 --max-time 5 -fsS "${ready_url}" >/dev/null 2>&1; then
      log_step "Server readiness check passed (${attempt}/${SERVER_READY_MAX_ATTEMPTS})"
      return 0
    fi
    if ((attempt < SERVER_READY_MAX_ATTEMPTS)); then
      if ((attempt == 1)); then
        log_step "Waiting for server readiness (up to ${SERVER_READY_MAX_ATTEMPTS} attempts)"
      fi
      sleep 1
    fi
  done
  echo "Readiness check failed after ${SERVER_READY_MAX_ATTEMPTS} attempts: ${ready_url}"
  curl --connect-timeout 1 --max-time 5 -fsS "${ready_url}" >/dev/null || true
  return 1
}

restart_server() {
  log_step "Restarting ${SERVER_SERVICE}"
  if ! systemctl restart "${SERVER_SERVICE}"; then
    echo "Service restart command failed: ${SERVER_SERVICE}"
    systemctl --no-pager --full status "${SERVER_SERVICE}" || true
    exit 1
  fi
  if ! systemctl is-active --quiet "${SERVER_SERVICE}"; then
    echo "Service restart failed: ${SERVER_SERVICE}"
    systemctl --no-pager --full status "${SERVER_SERVICE}" || true
    exit 1
  fi
  if ! wait_for_server_ready; then
    systemctl --no-pager --full status "${SERVER_SERVICE}" || true
    exit 1
  fi
}

restart_action_worker() {
  log_step "Restarting ${ACTION_WORKER_SERVICE}"
  if ! systemctl restart "${ACTION_WORKER_SERVICE}"; then
    echo "Service restart command failed: ${ACTION_WORKER_SERVICE}"
    systemctl --no-pager --full status "${ACTION_WORKER_SERVICE}" || true
    exit 1
  fi
  if ! systemctl is-active --quiet "${ACTION_WORKER_SERVICE}"; then
    echo "Service restart failed: ${ACTION_WORKER_SERVICE}"
    systemctl --no-pager --full status "${ACTION_WORKER_SERVICE}" || true
    exit 1
  fi
}

restart_battle_worker() {
  log_step "Restarting ${BATTLE_WORKER_SERVICE}"
  if ! systemctl restart "${BATTLE_WORKER_SERVICE}"; then
    echo "Service restart command failed: ${BATTLE_WORKER_SERVICE}"
    systemctl --no-pager --full status "${BATTLE_WORKER_SERVICE}" || true
    exit 1
  fi
  if ! systemctl is-active --quiet "${BATTLE_WORKER_SERVICE}"; then
    echo "Service restart failed: ${BATTLE_WORKER_SERVICE}"
    systemctl --no-pager --full status "${BATTLE_WORKER_SERVICE}" || true
    exit 1
  fi
}

restart_kitty_worker() {
  log_step "Restarting ${KITTY_WORKER_SERVICE}"
  if ! systemctl restart "${KITTY_WORKER_SERVICE}"; then
    echo "Service restart command failed: ${KITTY_WORKER_SERVICE}"
    systemctl --no-pager --full status "${KITTY_WORKER_SERVICE}" || true
    exit 1
  fi
  if ! systemctl is-active --quiet "${KITTY_WORKER_SERVICE}"; then
    echo "Service restart failed: ${KITTY_WORKER_SERVICE}"
    systemctl --no-pager --full status "${KITTY_WORKER_SERVICE}" || true
    exit 1
  fi
}

update_server() {
	prepare_repo
	ensure_action_worker_service
	build_server_binary
	build_action_worker_binary
	restart_server
	restart_action_worker
}

update_action_worker() {
  prepare_repo
  ensure_action_worker_service
  build_action_worker_binary
  restart_action_worker
}

update_battle_worker() {
  prepare_repo
  build_battle_worker_binary
  restart_battle_worker
}

update_backend() {
  prepare_repo
  ensure_action_worker_service
  build_server_binary
  build_action_worker_binary
  build_battle_worker_binary
  build_kitty_worker_binary
  restart_server
  restart_action_worker
  restart_battle_worker
  restart_kitty_worker
}

update_kitty_worker() {
  prepare_repo
  build_kitty_worker_binary
  restart_kitty_worker
}

update_server_with_kitty_worker() {
  prepare_repo
  ensure_action_worker_service
  build_server_binary
  build_action_worker_binary
  build_kitty_worker_binary
  restart_server
  restart_action_worker
  restart_kitty_worker
}

update_web() {
  prepare_repo
  ensure_web_node_version
  log_step "Building web assets only (no nginx restart)"
  sudo -u "${APP_USER}" bash -lc "
    set -e
    cd '${APP_DIR}/web'
    if ! npm ci --include=optional --no-audit --no-fund; then
      npm install --include=optional --no-audit --no-fund --no-package-lock
    fi
    if ! node -e \"require('@tailwindcss/oxide')\" >/dev/null 2>&1; then
      npm install --include=optional --no-audit --no-fund --no-package-lock
    fi
    npm run build
  "
}

case "${TARGET}" in
  server)
    update_server
    ;;
  action-worker)
    update_action_worker
    ;;
  battle-worker)
    update_battle_worker
    ;;
  kitty-worker)
    update_kitty_worker
    ;;
  server-with-kitty-worker)
    update_server_with_kitty_worker
    ;;
  backend)
    update_backend
    ;;
  web)
    update_web
    ;;
  all)
    update_backend
    update_web
    ;;
  *)
    echo "Unknown target: ${TARGET}"
    echo "target: server | action-worker | battle-worker | kitty-worker | server-with-kitty-worker | backend | web | all"
    exit 1
    ;;
esac

echo "Native split update complete: ${TARGET}"
