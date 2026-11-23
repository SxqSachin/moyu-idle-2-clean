#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DIST_DIR="${ROOT_DIR}/web/dist"
NGINX_TEMPLATE="${ROOT_DIR}/infra/nginx/local-dist-proxy.conf.template"

CONTAINER_NAME="${CONTAINER_NAME:-mysecondwebgame-local-dist}"
NGINX_IMAGE="${NGINX_IMAGE:-nginx:1.27-alpine}"
LOCAL_DIST_PORT="${LOCAL_DIST_PORT:-8081}"
BACKEND_HOST="${BACKEND_HOST:-host.docker.internal}"
BACKEND_PORT="${BACKEND_PORT:-8080}"

usage() {
  cat <<'EOF'
Usage:
  bash infra/scripts/run-local-dist-nginx.sh start [--no-build]
  bash infra/scripts/run-local-dist-nginx.sh stop
  bash infra/scripts/run-local-dist-nginx.sh status
  bash infra/scripts/run-local-dist-nginx.sh logs

Environment variables:
  LOCAL_DIST_PORT  (default: 8081)
  BACKEND_HOST     (default: host.docker.internal)
  BACKEND_PORT     (default: 8080)
  CONTAINER_NAME   (default: mysecondwebgame-local-dist)
  NGINX_IMAGE      (default: nginx:1.27-alpine)
EOF
}

ensure_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "[local-dist] docker not found"
    exit 1
  fi
}

linux_host_alias_arg() {
  if [[ "$(uname -s)" == "Linux" ]]; then
    echo "--add-host=host.docker.internal:host-gateway"
    return
  fi
  echo ""
}

start() {
  local do_build=1
  if [[ "${1:-}" == "--no-build" ]]; then
    do_build=0
  fi

  ensure_docker

  if [[ "${do_build}" == "1" ]]; then
    echo "[local-dist] building web/dist"
    (cd "${ROOT_DIR}/web" && npm run build)
  fi

  if [[ ! -d "${DIST_DIR}" ]]; then
    echo "[local-dist] dist directory not found: ${DIST_DIR}"
    echo "[local-dist] run: cd web && npm run build"
    exit 1
  fi
  if [[ ! -f "${NGINX_TEMPLATE}" ]]; then
    echo "[local-dist] nginx template not found: ${NGINX_TEMPLATE}"
    exit 1
  fi

  docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true

  local host_alias
  host_alias="$(linux_host_alias_arg)"
  echo "[local-dist] starting nginx container: ${CONTAINER_NAME}"
  docker run -d \
    --name "${CONTAINER_NAME}" \
    -p "${LOCAL_DIST_PORT}:80" \
    ${host_alias} \
    -e BACKEND_HOST="${BACKEND_HOST}" \
    -e BACKEND_PORT="${BACKEND_PORT}" \
    -v "${DIST_DIR}:/usr/share/nginx/html:ro" \
    -v "${NGINX_TEMPLATE}:/etc/nginx/templates/default.conf.template:ro" \
    "${NGINX_IMAGE}" >/dev/null

  echo "[local-dist] ready: http://localhost:${LOCAL_DIST_PORT}"
  echo "[local-dist] proxy target: http://${BACKEND_HOST}:${BACKEND_PORT}"
}

stop() {
  ensure_docker
  if docker ps -a --format '{{.Names}}' | grep -Fxq "${CONTAINER_NAME}"; then
    docker rm -f "${CONTAINER_NAME}" >/dev/null
    echo "[local-dist] stopped: ${CONTAINER_NAME}"
    return
  fi
  echo "[local-dist] container not found: ${CONTAINER_NAME}"
}

status() {
  ensure_docker
  docker ps --filter "name=^${CONTAINER_NAME}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
}

logs() {
  ensure_docker
  docker logs --tail 200 "${CONTAINER_NAME}"
}

ACTION="${1:-start}"
case "${ACTION}" in
  start)
    shift || true
    start "${1:-}"
    ;;
  stop)
    stop
    ;;
  status)
    status
    ;;
  logs)
    logs
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
