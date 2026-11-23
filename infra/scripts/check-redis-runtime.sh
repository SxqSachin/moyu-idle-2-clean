#!/usr/bin/env bash
set -euo pipefail

HOST="${REDIS_HOST:-127.0.0.1}"
PORT="${REDIS_PORT:-6379}"
DB="${REDIS_DB:-0}"
PASSWORD="${REDIS_PASSWORD:-}"

MEM_WARN_BYTES="${MEM_WARN_BYTES:-3221225472}" # 3 GiB
OPS_WARN="${OPS_WARN:-20000}"
SLOWLOG_WARN="${SLOWLOG_WARN:-128}"
FRAG_WARN="${FRAG_WARN:-1.8}"

usage() {
  cat <<'EOF'
Usage: bash infra/scripts/check-redis-runtime.sh [options]

Read-only / low-impact Redis runtime check:
- Uses only INFO / DBSIZE / SLOWLOG LEN / PING
- Does NOT run SCAN / KEYS / write commands

Options:
  --host <host>         Redis host (default: REDIS_HOST or 127.0.0.1)
  --port <port>         Redis port (default: REDIS_PORT or 6379)
  --db <db>             Redis db index (default: REDIS_DB or 0)
  --password <pwd>      Redis password (default: REDIS_PASSWORD)
  -h, --help            Show this help

Env thresholds (optional):
  MEM_WARN_BYTES        used_memory warning threshold (default: 3221225472)
  OPS_WARN              instantaneous ops/sec warning threshold (default: 20000)
  SLOWLOG_WARN          slowlog len warning threshold (default: 128)
  FRAG_WARN             memory fragmentation warning threshold (default: 1.8)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      HOST="$2"
      shift 2
      ;;
    --port)
      PORT="$2"
      shift 2
      ;;
    --db)
      DB="$2"
      shift 2
      ;;
    --password)
      PASSWORD="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if ! command -v redis-cli >/dev/null 2>&1; then
  echo "redis-cli not found. Install redis tools first."
  exit 1
fi

redis_cli=(redis-cli --raw -h "$HOST" -p "$PORT" -n "$DB" --no-auth-warning)
if [[ -n "$PASSWORD" ]]; then
  redis_cli+=(-a "$PASSWORD")
fi

run_redis() {
  "${redis_cli[@]}" "$@"
}

info_val() {
  local blob="$1"
  local key="$2"
  printf '%s\n' "$blob" | awk -F: -v k="$key" '$1==k {gsub(/\r/, "", $2); print $2; exit}'
}

parse_cmd_calls() {
  local blob="$1"
  local cmd="$2"
  printf '%s\n' "$blob" | awk -F'[:,=]' -v k="cmdstat_${cmd}" '$1==k {for(i=1;i<=NF;i++){if($i=="calls"){print $(i+1); exit}}}'
}

float_gt() {
  awk -v a="$1" -v b="$2" 'BEGIN { exit !(a > b) }'
}

int_gt() {
  awk -v a="$1" -v b="$2" 'BEGIN { exit !(a > b) }'
}

pong="$(run_redis PING 2>/dev/null || true)"
if [[ "$pong" != "PONG" ]]; then
  echo "CRITICAL: cannot reach redis at ${HOST}:${PORT} db=${DB}"
  exit 2
fi

mem_info="$(run_redis INFO memory)"
stats_info="$(run_redis INFO stats)"
clients_info="$(run_redis INFO clients)"
keyspace_info="$(run_redis INFO keyspace)"
cmdstats_info="$(run_redis INFO commandstats)"
dbsize="$(run_redis DBSIZE)"
slowlog_len="$(run_redis SLOWLOG LEN)"

used_memory="$(info_val "$mem_info" used_memory)"
used_memory_human="$(info_val "$mem_info" used_memory_human)"
mem_fragmentation_ratio="$(info_val "$mem_info" mem_fragmentation_ratio)"
ops_per_sec="$(info_val "$stats_info" instantaneous_ops_per_sec)"
evicted_keys="$(info_val "$stats_info" evicted_keys)"
expired_keys="$(info_val "$stats_info" expired_keys)"
keyspace_hits="$(info_val "$stats_info" keyspace_hits)"
keyspace_misses="$(info_val "$stats_info" keyspace_misses)"
rejected_connections="$(info_val "$stats_info" rejected_connections)"
blocked_clients="$(info_val "$clients_info" blocked_clients)"
connected_clients="$(info_val "$clients_info" connected_clients)"

db_line="$(printf '%s\n' "$keyspace_info" | grep "^db${DB}:" || true)"
db_keys=""
db_expires=""
if [[ -n "$db_line" ]]; then
  db_keys="$(printf '%s\n' "$db_line" | sed -E 's/.*keys=([0-9]+).*/\1/')"
  db_expires="$(printf '%s\n' "$db_line" | sed -E 's/.*expires=([0-9]+).*/\1/')"
fi

setnx_calls="$(parse_cmd_calls "$cmdstats_info" setnx)"
incr_calls="$(parse_cmd_calls "$cmdstats_info" incr)"
eval_calls="$(parse_cmd_calls "$cmdstats_info" eval)"
evalsha_calls="$(parse_cmd_calls "$cmdstats_info" evalsha)"

keyspace_total=$(( ${keyspace_hits:-0} + ${keyspace_misses:-0} ))
hit_rate="N/A"
if [[ "$keyspace_total" -gt 0 ]]; then
  hit_rate="$(awk -v h="${keyspace_hits:-0}" -v t="$keyspace_total" 'BEGIN { printf "%.2f%%", (h/t)*100 }')"
fi

status="OK"
warnings=()

if int_gt "${evicted_keys:-0}" 0; then
  status="CRITICAL"
  warnings+=("evicted_keys=${evicted_keys} (>0)")
fi
if int_gt "${rejected_connections:-0}" 0; then
  [[ "$status" == "OK" ]] && status="WARN"
  warnings+=("rejected_connections=${rejected_connections} (>0)")
fi
if int_gt "${blocked_clients:-0}" 0; then
  [[ "$status" == "OK" ]] && status="WARN"
  warnings+=("blocked_clients=${blocked_clients} (>0)")
fi
if int_gt "${used_memory:-0}" "$MEM_WARN_BYTES"; then
  [[ "$status" == "OK" ]] && status="WARN"
  warnings+=("used_memory=${used_memory} (> ${MEM_WARN_BYTES})")
fi
if float_gt "${mem_fragmentation_ratio:-0}" "$FRAG_WARN"; then
  [[ "$status" == "OK" ]] && status="WARN"
  warnings+=("mem_fragmentation_ratio=${mem_fragmentation_ratio} (> ${FRAG_WARN})")
fi
if int_gt "${ops_per_sec:-0}" "$OPS_WARN"; then
  [[ "$status" == "OK" ]] && status="WARN"
  warnings+=("instantaneous_ops_per_sec=${ops_per_sec} (> ${OPS_WARN})")
fi
if int_gt "${slowlog_len:-0}" "$SLOWLOG_WARN"; then
  [[ "$status" == "OK" ]] && status="WARN"
  warnings+=("slowlog_len=${slowlog_len} (> ${SLOWLOG_WARN})")
fi

echo "Redis runtime check (read-only)"
echo "target=${HOST}:${PORT} db=${DB}"
echo "status=${status}"
echo
echo "== Core metrics =="
echo "connected_clients=${connected_clients:-N/A}"
echo "blocked_clients=${blocked_clients:-N/A}"
echo "dbsize=${dbsize:-N/A}"
echo "db${DB}.keys=${db_keys:-N/A}"
echo "db${DB}.expires=${db_expires:-N/A}"
echo "used_memory=${used_memory:-N/A} (${used_memory_human:-N/A})"
echo "mem_fragmentation_ratio=${mem_fragmentation_ratio:-N/A}"
echo "instantaneous_ops_per_sec=${ops_per_sec:-N/A}"
echo "expired_keys=${expired_keys:-N/A}"
echo "evicted_keys=${evicted_keys:-N/A}"
echo "rejected_connections=${rejected_connections:-N/A}"
echo "slowlog_len=${slowlog_len:-N/A}"
echo "keyspace_hit_rate=${hit_rate}"
echo
echo "== Guard command activity (lifetime calls) =="
echo "cmdstat_setnx.calls=${setnx_calls:-0}"
echo "cmdstat_incr.calls=${incr_calls:-0}"
echo "cmdstat_eval.calls=${eval_calls:-0}"
echo "cmdstat_evalsha.calls=${evalsha_calls:-0}"
echo
echo "note=No key scan performed (non-intrusive mode)."

if [[ "${#warnings[@]}" -gt 0 ]]; then
  echo
  echo "== Findings =="
  for item in "${warnings[@]}"; do
    echo "- ${item}"
  done
fi

if [[ "$status" == "CRITICAL" ]]; then
  exit 2
fi
if [[ "$status" == "WARN" ]]; then
  exit 1
fi
exit 0
