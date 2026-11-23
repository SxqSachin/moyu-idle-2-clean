#!/usr/bin/env bash
set -euo pipefail

REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
REDIS_DB="${REDIS_DB:-0}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
NODE_KEY_PREFIX="${NODE_KEY_PREFIX:-ws:backend:node:}"
SYNC_INTERVAL_SECONDS="${SYNC_INTERVAL_SECONDS:-15}"
API_OUTPUT="${API_OUTPUT:-/etc/nginx/conf.d/upstreams/game_backend_api.nodes.conf}"
WS_OUTPUT="${WS_OUTPUT:-/etc/nginx/conf.d/upstreams/game_backend_ws.nodes.conf}"
SERVER_FLAGS="${SERVER_FLAGS:-max_fails=3 fail_timeout=10s}"
HEALTHCHECK_ENABLED="${HEALTHCHECK_ENABLED:-true}"
HEALTHCHECK_PATH="${HEALTHCHECK_PATH:-/readyz}"
HEALTHCHECK_TIMEOUT_SECONDS="${HEALTHCHECK_TIMEOUT_SECONDS:-2}"
RUN_ONCE="${RUN_ONCE:-false}"
NGINX_RELOAD_CMD="${NGINX_RELOAD_CMD:-nginx -t && nginx -s reload}"

REDIS_HOST="${REDIS_ADDR%:*}"
REDIS_PORT="${REDIS_ADDR##*:}"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[sync-nginx-upstreams] missing command: $cmd"
    exit 1
  fi
}

require_cmd redis-cli
if [ "$HEALTHCHECK_ENABLED" = "true" ]; then
  require_cmd curl
fi

redis_cmd() {
  if [ -n "$REDIS_PASSWORD" ]; then
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -n "$REDIS_DB" -a "$REDIS_PASSWORD" --raw "$@"
  else
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -n "$REDIS_DB" --raw "$@"
  fi
}

fetch_nodes() {
  local key value
  while IFS= read -r key; do
    [ -z "$key" ] && continue
    value="$(redis_cmd GET "$key" || true)"
    [ -n "$value" ] && echo "$value"
  done < <(redis_cmd --scan --pattern "${NODE_KEY_PREFIX}*")
}

is_node_healthy() {
  local node="$1"
  if [ "$HEALTHCHECK_ENABLED" != "true" ]; then
    return 0
  fi
  curl -fsS --max-time "$HEALTHCHECK_TIMEOUT_SECONDS" "http://${node}${HEALTHCHECK_PATH}" >/dev/null
}

render_node_file() {
  local output="$1"
  local temp
  temp="$(mktemp)"
  mapfile -t nodes < <(fetch_nodes | sort -u)
  if [ "${#nodes[@]}" -eq 0 ]; then
    rm -f "$temp"
    return 1
  fi
  for node in "${nodes[@]}"; do
    if ! is_node_healthy "$node"; then
      echo "[sync-nginx-upstreams] skip unhealthy node ${node}"
      continue
    fi
    printf 'server %s %s;\n' "$node" "$SERVER_FLAGS" >> "$temp"
  done
  if [ ! -s "$temp" ]; then
    rm -f "$temp"
    return 1
  fi
  if [ ! -f "$output" ] || ! cmp -s "$temp" "$output"; then
    mkdir -p "$(dirname "$output")"
    mv "$temp" "$output"
    return 0
  fi
  rm -f "$temp"
  return 2
}

reload_if_changed() {
  local changed=false
  if render_node_file "$API_OUTPUT"; then
    changed=true
  elif [ "$?" -eq 1 ]; then
    echo "[sync-nginx-upstreams] no active backend nodes found; skip update"
  fi
  if render_node_file "$WS_OUTPUT"; then
    changed=true
  elif [ "$?" -eq 1 ]; then
    echo "[sync-nginx-upstreams] no active backend nodes found; skip update"
  fi
  if [ "$changed" = true ]; then
    bash -lc "$NGINX_RELOAD_CMD"
    echo "[sync-nginx-upstreams] upstream files updated and nginx reloaded"
  fi
}

echo "[sync-nginx-upstreams] starting redis=${REDIS_ADDR} prefix=${NODE_KEY_PREFIX} interval=${SYNC_INTERVAL_SECONDS}s"
if [ "$RUN_ONCE" = "true" ]; then
  reload_if_changed || true
  exit 0
fi
while true; do
  reload_if_changed || true
  sleep "$SYNC_INTERVAL_SECONDS"
done
