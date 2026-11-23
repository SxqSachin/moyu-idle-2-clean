#!/usr/bin/env bash
set -euo pipefail

# Native role-based deployment for Debian/Ubuntu + systemd.
# Roles: pg | redis | nats | server | action-worker | battle-worker | kitty-worker | server-with-kitty-worker | web | lb | all

ROLE="${1:-}"
REPO_URL="${2:-}"
APP_DIR="${3:-/opt/moyu-idle}"
BRANCH="${BRANCH:-main}"
APP_USER="${APP_USER:-mysecondwebgame}"
APP_GROUP="${APP_GROUP:-mysecondwebgame}"
APP_PORT="${APP_PORT:-8080}"
PG_PORT="${PG_PORT:-5432}"
RAW_DB_HOST="${DB_HOST-}"
RAW_REDIS_ADDR="${REDIS_ADDR-}"
RAW_NATS_URL="${NATS_URL-}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
NATS_URL="${NATS_URL:-nats://127.0.0.1:4222}"
MQ_TOPIC_PREFIX="${MQ_TOPIC_PREFIX:-game}"
DB_NAME="${DB_NAME:-game}"
DB_USER="${DB_USER:-game}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_HOST="${DB_HOST:-127.0.0.1}"
JWT_SECRET="${JWT_SECRET:-RCIN}"
DEFAULT_NODE_IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
NODE_ADVERTISE_ADDR="${NODE_ADVERTISE_ADDR:-${SERVER_ADVERTISE_ADDR:-${DEFAULT_NODE_IP:+${DEFAULT_NODE_IP}:${APP_PORT}}}}"
SERVER_DB_POOL_MAX_CONNS="${SERVER_DB_POOL_MAX_CONNS:-24}"
AUTH_DB_POOL_MAX_CONNS="${AUTH_DB_POOL_MAX_CONNS:-4}"
AUTH_DB_TIMEOUT_MS="${AUTH_DB_TIMEOUT_MS:-800}"
ACTION_WORKER_DB_POOL_MAX_CONNS="${ACTION_WORKER_DB_POOL_MAX_CONNS:-4}"
KITTY_WORKER_DB_POOL_MAX_CONNS="${KITTY_WORKER_DB_POOL_MAX_CONNS:-4}"
BATTLE_WORKER_DB_POOL_MAX_CONNS="${BATTLE_WORKER_DB_POOL_MAX_CONNS:-16}"
BATTLE_WORKER_PPROF_ADDR="${BATTLE_WORKER_PPROF_ADDR:-}"
WS_BATTLE_BATCH_COMPRESSION_ENABLED="${WS_BATTLE_BATCH_COMPRESSION_ENABLED:-false}"
INSTALL_DEPS="${INSTALL_DEPS:-1}"
WEB_NODE_REQUIRED_MAJOR="${WEB_NODE_REQUIRED_MAJOR:-22}"
WEB_NODE_REQUIRED_MINOR="${WEB_NODE_REQUIRED_MINOR:-14}"
SPLIT_REQUIRE_EXPLICIT_ENDPOINTS="${SPLIT_REQUIRE_EXPLICIT_ENDPOINTS:-0}"
SPLIT_ALLOW_LOOPBACK_ENDPOINTS="${SPLIT_ALLOW_LOOPBACK_ENDPOINTS:-0}"
REAL_IP_PROXY_HEADER="${REAL_IP_PROXY_HEADER:-X-Forwarded-For}"
REAL_IP_TRUSTED_PROXY_CHECK="${REAL_IP_TRUSTED_PROXY_CHECK:-true}"
REAL_IP_TRUSTED_PROXIES="${REAL_IP_TRUSTED_PROXIES:-10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.1/32,::1/128}"
REAL_IP_ENABLE_VALIDATION="${REAL_IP_ENABLE_VALIDATION:-true}"

if [[ -z "${ROLE}" ]]; then
  echo "Usage: bash infra/scripts/deploy-linux-native-split.sh <role> [repo_url] [app_dir]"
  echo "role: pg | redis | nats | server | action-worker | battle-worker | kitty-worker | server-with-kitty-worker | web | lb | all"
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root."
  exit 1
fi

ensure_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Missing command: ${cmd}"
    exit 1
  fi
}

log_step() {
  echo "[native-split][${ROLE}] $*"
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
    echo "Missing command: ${cmd}. Set INSTALL_DEPS=1 for auto install or install it manually."
    exit 1
  fi
  log_step "Installing missing dependency for ${cmd}"
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

fetch_repo_if_needed() {
  case "${ROLE}" in
    server|action-worker|battle-worker|kitty-worker|server-with-kitty-worker|web|all)
      ensure_cmd_or_install git git ca-certificates
      mkdir -p "$(dirname "${APP_DIR}")"
      log_step "Preparing repository at ${APP_DIR}"
      if [[ -d "${APP_DIR}/.git" ]]; then
        git -C "${APP_DIR}" fetch --all --prune
        git -C "${APP_DIR}" checkout "${BRANCH}"
        git -C "${APP_DIR}" pull --ff-only origin "${BRANCH}"
      else
        if [[ -z "${REPO_URL}" ]]; then
          echo "REPO_URL is required for first-time deployment."
          exit 1
        fi
        git clone --branch "${BRANCH}" "${REPO_URL}" "${APP_DIR}"
      fi
      ensure_app_user
      chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
      ;;
    *)
      ;;
  esac
}

ensure_web_node_version() {
  ensure_cmd node
  ensure_cmd npm
  local node_ver node_major node_minor
  node_ver="$(node -v | sed -E 's/^v//')"
  node_major="$(echo "${node_ver}" | cut -d. -f1)"
  node_minor="$(echo "${node_ver}" | cut -d. -f2)"
  if [[ "${node_major}" != "${WEB_NODE_REQUIRED_MAJOR}" ]] || [[ "${node_minor}" -lt "${WEB_NODE_REQUIRED_MINOR}" ]]; then
    echo "Node.js v${WEB_NODE_REQUIRED_MAJOR}.${WEB_NODE_REQUIRED_MINOR}+ is required for build tasks, current: v${node_ver}"
    echo "Please install the required Node.js version or set INSTALL_DEPS=1 for auto install."
    exit 1
  fi
}

ensure_web_node_runtime() {
  local current_ver current_major current_minor desired_pkg_ver
  if command -v node >/dev/null 2>&1; then
    current_ver="$(node -v | sed -E 's/^v//')"
    current_major="$(echo "${current_ver}" | cut -d. -f1)"
    current_minor="$(echo "${current_ver}" | cut -d. -f2)"
    if [[ "${current_major}" == "${WEB_NODE_REQUIRED_MAJOR}" ]] && [[ "${current_minor}" -ge "${WEB_NODE_REQUIRED_MINOR}" ]]; then
      return 0
    fi
  fi

  if [[ "${INSTALL_DEPS}" != "1" ]]; then
    echo "Node.js v${WEB_NODE_REQUIRED_MAJOR}.${WEB_NODE_REQUIRED_MINOR}+ is required for build tasks."
    echo "Current node: $(node -v 2>/dev/null || echo missing). Set INSTALL_DEPS=1 for auto install."
    exit 1
  fi

  log_step "Installing Node.js ${WEB_NODE_REQUIRED_MAJOR}.x runtime for build tasks"
  apt_install ca-certificates curl gnupg
  ensure_cmd curl
  sed -i.bak '/deb .*deb\.nodesource\.com\/node_/d' /etc/apt/sources.list 2>/dev/null || true
  for f in /etc/apt/sources.list.d/*.list; do
    [[ -e "${f}" ]] || continue
    sed -i.bak '/deb .*deb\.nodesource\.com\/node_/d' "${f}" || true
  done
  rm -f /etc/apt/sources.list.d/nodesource.list
  mkdir -p /etc/apt/keyrings
  curl -fsSL "https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key" | gpg --dearmor --yes -o /etc/apt/keyrings/nodesource.gpg
  chmod a+r /etc/apt/keyrings/nodesource.gpg
  cat >/etc/apt/sources.list.d/nodesource.list <<EOF
deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_${WEB_NODE_REQUIRED_MAJOR}.x nodistro main
EOF
  log_step "Refreshing apt indexes for NodeSource"
  apt-get update -y
  desired_pkg_ver="$(apt-cache madison nodejs | awk -v major="${WEB_NODE_REQUIRED_MAJOR}" -v minor="${WEB_NODE_REQUIRED_MINOR}" '$3 ~ "^"major"\\."minor"\\." {print $3; exit}' || true)"
  if [[ -n "${desired_pkg_ver}" ]]; then
    log_step "Installing nodejs package version ${desired_pkg_ver}"
    apt-get install -y "nodejs=${desired_pkg_ver}"
  else
    log_step "Installing latest nodejs package from NodeSource ${WEB_NODE_REQUIRED_MAJOR}.x"
    apt-get install -y nodejs
  fi
  log_step "Node runtime ready: $(node -v)"
}

is_loopback_host() {
  local host="$1"
  case "${host}" in
    127.0.0.1|localhost|::1)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

parse_redis_addr() {
  local value="$1"
  local host port
  value="${value#redis://}"
  value="${value#rediss://}"
  value="${value%%/*}"
  host="${value%:*}"
  port="${value##*:}"
  if [[ -z "${host}" || -z "${port}" || "${host}" == "${value}" ]]; then
    return 1
  fi
  printf '%s %s\n' "${host}" "${port}"
}

parse_nats_url() {
  local value="$1"
  local endpoint host port
  endpoint="${value%%,*}"
  endpoint="${endpoint#nats://}"
  endpoint="${endpoint%%/*}"
  host="${endpoint%:*}"
  port="${endpoint##*:}"
  if [[ -z "${host}" || -z "${port}" || "${host}" == "${endpoint}" ]]; then
    return 1
  fi
  printf '%s %s\n' "${host}" "${port}"
}

check_tcp_endpoint() {
  local endpoint_name="$1"
  local host="$2"
  local port="$3"
  if timeout 3 bash -lc "cat < /dev/null > /dev/tcp/${host}/${port}" 2>/dev/null; then
    log_step "${endpoint_name} reachable at ${host}:${port}"
    return 0
  fi
  echo "${endpoint_name} is unreachable at ${host}:${port}"
  case "${endpoint_name}" in
    PostgreSQL)
      echo "Check DB_HOST/PG_PORT values and ensure PostgreSQL allows remote access from this host."
      echo "For split roles, DB_HOST must point to the PostgreSQL node (use PG_PORT, not DB_PORT)."
      echo "Validate PostgreSQL listen_addresses/pg_hba.conf and network security rules for port ${port}."
      ;;
    Redis)
      echo "Check REDIS_ADDR and ensure Redis is listening on a reachable interface from this host."
      ;;
    NATS)
      echo "Check NATS_URL and ensure NATS server is reachable from this host."
      ;;
    *)
      ;;
  esac
  return 1
}

preflight_backend_endpoints() {
  local redis_host redis_port nats_host nats_port
  local db_host db_port
  db_host="${DB_HOST}"
  db_port="${PG_PORT}"

  if [[ "${SPLIT_REQUIRE_EXPLICIT_ENDPOINTS}" == "1" ]]; then
    if [[ -z "${RAW_DB_HOST}" || -z "${RAW_REDIS_ADDR}" || -z "${RAW_NATS_URL}" ]]; then
      echo "For split role ${ROLE}, DB_HOST, REDIS_ADDR and NATS_URL must be set explicitly."
      echo "Example: DB_HOST=10.0.0.10 REDIS_ADDR=10.0.0.20:6379 NATS_URL=nats://10.0.0.30:4222"
      echo "Override only if intentional: SPLIT_REQUIRE_EXPLICIT_ENDPOINTS=0"
      exit 1
    fi
  fi

  if ! read -r redis_host redis_port < <(parse_redis_addr "${REDIS_ADDR}"); then
    echo "Invalid REDIS_ADDR format: ${REDIS_ADDR}. Expected host:port (or redis://host:port)."
    exit 1
  fi
  if ! read -r nats_host nats_port < <(parse_nats_url "${NATS_URL}"); then
    echo "Invalid NATS_URL format: ${NATS_URL}. Expected nats://host:port."
    exit 1
  fi

  if [[ "${SPLIT_ALLOW_LOOPBACK_ENDPOINTS}" != "1" ]]; then
    if is_loopback_host "${db_host}" || is_loopback_host "${redis_host}" || is_loopback_host "${nats_host}"; then
      echo "Loopback endpoints are blocked for split role ${ROLE}: DB_HOST=${db_host}, REDIS_ADDR=${REDIS_ADDR}, NATS_URL=${NATS_URL}"
      echo "Set remote service addresses for split topology."
      echo "Override only if intentional: SPLIT_ALLOW_LOOPBACK_ENDPOINTS=1"
      exit 1
    fi
  fi

  check_tcp_endpoint "PostgreSQL" "${db_host}" "${db_port}" || exit 1
  check_tcp_endpoint "Redis" "${redis_host}" "${redis_port}" || exit 1
  check_tcp_endpoint "NATS" "${nats_host}" "${nats_port}" || exit 1
}

deploy_lb() {
  apt_install nginx

  local lb_domain lb_www_domain lb_enable_tls lb_cert_fullchain lb_cert_privkey lb_web_upstreams lb_api_upstreams lb_ws_upstreams
  lb_domain="${LB_DOMAIN:-moyu-idle.com}"
  lb_www_domain="${LB_WWW_DOMAIN:-www.moyu-idle.com}"
  lb_enable_tls="${LB_ENABLE_TLS:-1}"
  lb_cert_fullchain="${LB_CERT_FULLCHAIN_PATH:-/etc/letsencrypt/live/${lb_domain}/fullchain.pem}"
  lb_cert_privkey="${LB_CERT_PRIVKEY_PATH:-/etc/letsencrypt/live/${lb_domain}/privkey.pem}"
  lb_web_upstreams="${LB_WEB_UPSTREAMS:-}"
  lb_api_upstreams="${LB_API_UPSTREAMS:-}"
  lb_ws_upstreams="${LB_WS_UPSTREAMS:-${lb_api_upstreams}}"

  if [[ -z "${lb_web_upstreams}" ]]; then
    echo "LB_WEB_UPSTREAMS is required, for example: LB_WEB_UPSTREAMS=10.0.0.11:80,10.0.0.12:80"
    exit 1
  fi
  if [[ -z "${lb_api_upstreams}" ]]; then
    echo "LB_API_UPSTREAMS is required, for example: LB_API_UPSTREAMS=10.0.0.21:8080,10.0.0.22:8080"
    echo "Optional: LB_WS_UPSTREAMS=10.0.0.21:8080,10.0.0.22:8080 (defaults to LB_API_UPSTREAMS)"
    exit 1
  fi

  write_upstream_servers() {
    local upstreams="$1"
    upstreams="${upstreams//,/ }"
    for upstream in ${upstreams}; do
      printf '    server %s;\n' "${upstream}"
    done
  }

  if [[ "${lb_enable_tls}" == "1" ]]; then
    cat >/etc/nginx/sites-available/mysecondwebgame-lb.conf <<EOF
map \$http_upgrade \$connection_upgrade {
    default upgrade;
    ''      close;
}

upstream mysecondwebgame_lb_web {
    hash \$remote_addr consistent;
$(write_upstream_servers "${lb_web_upstreams}")
    keepalive 128;
}

upstream mysecondwebgame_lb_api {
    hash \$remote_addr consistent;
$(write_upstream_servers "${lb_api_upstreams}")
    keepalive 128;
}

upstream mysecondwebgame_lb_ws {
    hash \$remote_addr consistent;
$(write_upstream_servers "${lb_ws_upstreams}")
    keepalive 128;
}

server {
    listen 80;
    listen 443 ssl;
    server_name ${lb_domain};

    ssl_certificate ${lb_cert_fullchain};
    ssl_certificate_key ${lb_cert_privkey};

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    return 301 https://${lb_www_domain}\$request_uri;
}

server {
    listen 80;
    server_name ${lb_www_domain};

    return 301 https://${lb_www_domain}\$request_uri;
}

server {
    listen 443 ssl;
    server_name ${lb_www_domain};

    ssl_certificate ${lb_cert_fullchain};
    ssl_certificate_key ${lb_cert_privkey};

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://mysecondwebgame_lb_web;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
    }

    location /assets/ {
        proxy_pass http://mysecondwebgame_lb_web;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
        expires 30d;
        add_header Cache-Control "public, max-age=2592000, immutable";
    }

    location /api/ {
        proxy_pass http://mysecondwebgame_lb_api;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
    }

    location /ws {
        proxy_pass http://mysecondwebgame_lb_ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
    }
}
EOF
  else
    cat >/etc/nginx/sites-available/mysecondwebgame-lb.conf <<EOF
map \$http_upgrade \$connection_upgrade {
    default upgrade;
    ''      close;
}

upstream mysecondwebgame_lb_web {
    hash \$remote_addr consistent;
$(write_upstream_servers "${lb_web_upstreams}")
    keepalive 128;
}

upstream mysecondwebgame_lb_api {
    hash \$remote_addr consistent;
$(write_upstream_servers "${lb_api_upstreams}")
    keepalive 128;
}

upstream mysecondwebgame_lb_ws {
    hash \$remote_addr consistent;
$(write_upstream_servers "${lb_ws_upstreams}")
    keepalive 128;
}

server {
    listen 80;
    server_name ${lb_domain};

    return 301 http://${lb_www_domain}\$request_uri;
}

server {
    listen 80;
    server_name ${lb_www_domain};

    location / {
        proxy_pass http://mysecondwebgame_lb_web;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
    }

    location /assets/ {
        proxy_pass http://mysecondwebgame_lb_web;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
        expires 30d;
        add_header Cache-Control "public, max-age=2592000, immutable";
    }

    location /api/ {
        proxy_pass http://mysecondwebgame_lb_api;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
    }

    location /ws {
        proxy_pass http://mysecondwebgame_lb_ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
    }
}
EOF
  fi

  ln -sf /etc/nginx/sites-available/mysecondwebgame-lb.conf /etc/nginx/sites-enabled/mysecondwebgame-lb.conf
  rm -f /etc/nginx/sites-enabled/default
  nginx -t
  systemctl enable nginx
  systemctl restart nginx
}

url_encode() {
  local value="$1"
  local out="" char hex
  local i
  LC_ALL=C
  for ((i = 0; i < ${#value}; i++)); do
    char="${value:i:1}"
    case "${char}" in
      [a-zA-Z0-9.~_-])
        out+="${char}"
        ;;
      *)
        printf -v hex '%%%02X' "'${char}"
        out+="${hex}"
        ;;
    esac
  done
  printf '%s' "${out}"
}

database_url_for_pool() {
  local pool_max="$1"
  local encoded_user encoded_password
  encoded_user="$(url_encode "${DB_USER}")"
  encoded_password="$(url_encode "${DB_PASSWORD}")"
  printf 'postgres://%s:%s@%s:%s/%s?sslmode=disable&pool_max_conns=%s' "${encoded_user}" "${encoded_password}" "${DB_HOST}" "${PG_PORT}" "${DB_NAME}" "${pool_max}"
}

write_common_env() {
  local output="$1"
  local database_url="$2"
  cat >"${output}" <<EOF
APP_PORT=${APP_PORT}
DATABASE_URL=${database_url}
AUTH_DB_POOL_MAX_CONNS=${AUTH_DB_POOL_MAX_CONNS}
AUTH_DB_TIMEOUT_MS=${AUTH_DB_TIMEOUT_MS}
REDIS_ADDR=${REDIS_ADDR}
REDIS_PASSWORD=
REDIS_DB=0
NATS_URL=${NATS_URL}
MQ_TOPIC_PREFIX=${MQ_TOPIC_PREFIX}
WS_TARGETED_ROUTING_ENABLED=false
WS_BATTLE_BATCH_COMPRESSION_ENABLED=${WS_BATTLE_BATCH_COMPRESSION_ENABLED}
SCHEDULER_SHARDS=8
SCHEDULER_ENGINE=minheap
SCHEDULER_TICK_MS=100
SCHEDULER_WHEEL_SIZE=256
NODE_ADVERTISE_ADDR=${NODE_ADVERTISE_ADDR}
NODE_REGISTRY_PREFIX=ws:backend:node:
NODE_HEARTBEAT_SECONDS=30
NODE_HEARTBEAT_INTERVAL_SECONDS=10
ACHIEVEMENTS_ENABLED=true
ACHIEVEMENT_WORKER_ENABLED=true
ACHIEVEMENT_WORKER_CONCURRENCY=2
GAME_DATA_PATH=${APP_DIR}/packages/game-data/generated/game-data.json
GLOBAL_CONFIG_PATH=${APP_DIR}/packages/game-config/source/global-config.json
SHARED_GLOBAL_CONFIG_PATH=${APP_DIR}/packages/game-config/source/shared-global-config.json
JWT_SECRET=${JWT_SECRET}
REAL_IP_PROXY_HEADER=${REAL_IP_PROXY_HEADER}
REAL_IP_TRUSTED_PROXY_CHECK=${REAL_IP_TRUSTED_PROXY_CHECK}
REAL_IP_TRUSTED_PROXIES=${REAL_IP_TRUSTED_PROXIES}
REAL_IP_ENABLE_VALIDATION=${REAL_IP_ENABLE_VALIDATION}
EOF
}

write_env_files() {
	mkdir -p /etc/mysecondwebgame
	write_server_env_file
	write_action_worker_env_file
	write_battle_worker_env_file
	write_kitty_worker_env_file
	chmod 600 /etc/mysecondwebgame/server.env /etc/mysecondwebgame/action-worker.env /etc/mysecondwebgame/battle-worker.env /etc/mysecondwebgame/kitty-worker.env
}

write_server_env_file() {
	mkdir -p /etc/mysecondwebgame
	write_common_env /etc/mysecondwebgame/server.env "$(database_url_for_pool "${SERVER_DB_POOL_MAX_CONNS}")"
	chmod 600 /etc/mysecondwebgame/server.env
}

write_action_worker_env_file() {
	mkdir -p /etc/mysecondwebgame
	write_common_env /etc/mysecondwebgame/action-worker.env "$(database_url_for_pool "${ACTION_WORKER_DB_POOL_MAX_CONNS}")"
	chmod 600 /etc/mysecondwebgame/action-worker.env
}

write_battle_worker_env_file() {
	mkdir -p /etc/mysecondwebgame
	write_common_env /etc/mysecondwebgame/battle-worker.env "$(database_url_for_pool "${BATTLE_WORKER_DB_POOL_MAX_CONNS}")"
	cat >>/etc/mysecondwebgame/battle-worker.env <<EOF
BATTLE_WORKER_PPROF_ADDR=${BATTLE_WORKER_PPROF_ADDR}
EOF
	chmod 600 /etc/mysecondwebgame/battle-worker.env
}

write_kitty_worker_env_file() {
	mkdir -p /etc/mysecondwebgame
	write_common_env /etc/mysecondwebgame/kitty-worker.env "$(database_url_for_pool "${KITTY_WORKER_DB_POOL_MAX_CONNS}")"
	chmod 600 /etc/mysecondwebgame/kitty-worker.env
}

deploy_pg() {
  apt_install postgresql postgresql-client
  systemctl enable postgresql
  systemctl restart postgresql
  local role_exists db_exists
  role_exists="$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" || true)"
  if [[ "${role_exists}" != "1" ]]; then
    sudo -u postgres psql -c "CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASSWORD}';"
  else
    sudo -u postgres psql -c "ALTER ROLE ${DB_USER} WITH PASSWORD '${DB_PASSWORD}';"
  fi
  db_exists="$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" || true)"
  if [[ "${db_exists}" != "1" ]]; then
    sudo -u postgres psql -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"
  fi
  if [[ -f "${APP_DIR}/server/schema/init.sql" ]]; then
		PGPASSWORD="${DB_PASSWORD}" psql "postgresql://$(url_encode "${DB_USER}"):$(url_encode "${DB_PASSWORD}")@127.0.0.1:${PG_PORT}/${DB_NAME}?sslmode=disable" -f "${APP_DIR}/server/schema/init.sql"
  else
    echo "Skip schema apply: ${APP_DIR}/server/schema/init.sql not found."
  fi
}

deploy_redis() {
  apt_install redis-server
  if [[ -f /etc/redis/redis.conf ]]; then
    sed -i "s/^bind .*/bind 0.0.0.0 ::0/" /etc/redis/redis.conf
    sed -i "s/^protected-mode .*/protected-mode no/" /etc/redis/redis.conf
  fi
  systemctl enable redis-server
  systemctl restart redis-server
}

deploy_nats() {
  apt_install nats-server
  systemctl enable nats-server
  systemctl restart nats-server
}

ensure_shared_game_data_artifacts() {
  local generated_dir="${APP_DIR}/packages/game-data/generated"
  local required_files=(
    "${generated_dir}/game-data.json"
    "${generated_dir}/game_data.go"
    "${generated_dir}/game-data.ts"
  )
  local missing=0
  local file
  for file in "${required_files[@]}"; do
    if [[ ! -f "${file}" ]]; then
      echo "Missing required shared game-data artifact: ${file}"
      missing=1
    fi
  done
  if [[ "${missing}" -ne 0 ]]; then
    echo "Generate artifacts in a build environment: node packages/game-data/scripts/build.mjs"
    echo "Commit the full directory: packages/game-data/generated"
    exit 1
  fi
  log_step "Shared game-data artifacts detected under ${generated_dir}"
}

deploy_server() {
  log_step "Deploying server role"
  apt_install golang-go
  preflight_backend_endpoints
  ensure_shared_game_data_artifacts
	write_server_env_file
  mkdir -p "${APP_DIR}/bin"
  chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go mod tidy && go build -o '${APP_DIR}/bin/mysecondwebgame-server' ./cmd/server"
  cat >/etc/systemd/system/mysecondwebgame-server.service <<EOF
[Unit]
Description=MySecondWebGame API/WS Server
After=network.target postgresql.service redis-server.service nats-server.service
Wants=postgresql.service redis-server.service nats-server.service

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${APP_DIR}/server
EnvironmentFile=/etc/mysecondwebgame/server.env
ExecStart=${APP_DIR}/bin/mysecondwebgame-server
Restart=always
RestartSec=3
LimitNOFILE=65535
MemoryMax=6G
TasksMax=2048
ExecStopPost=/bin/sh -c 'journalctl -u mysecondwebgame-server -n 200 --no-pager > /tmp/mysecondwebgame-server-last-stop.log || true'

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable mysecondwebgame-server
  systemctl restart mysecondwebgame-server
  log_step "Server role deployment completed"
}

deploy_action_worker() {
  log_step "Deploying action-worker role"
  apt_install golang-go
  preflight_backend_endpoints
  ensure_shared_game_data_artifacts
	write_action_worker_env_file
  mkdir -p "${APP_DIR}/bin"
  chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go mod tidy && go build -o '${APP_DIR}/bin/mysecondwebgame-action-worker' ./cmd/action_worker"
  cat >/etc/systemd/system/mysecondwebgame-action-worker.service <<EOF
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
ExecStopPost=/bin/sh -c 'journalctl -u mysecondwebgame-action-worker -n 200 --no-pager > /tmp/mysecondwebgame-action-worker-last-stop.log || true'

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable mysecondwebgame-action-worker
  systemctl restart mysecondwebgame-action-worker
  log_step "Action-worker role deployment completed"
}

deploy_battle_worker() {
  log_step "Deploying battle-worker role"
  apt_install golang-go
  preflight_backend_endpoints
  ensure_shared_game_data_artifacts
	write_battle_worker_env_file
  mkdir -p "${APP_DIR}/bin"
  chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go mod tidy && go build -o '${APP_DIR}/bin/mysecondwebgame-battle-worker' ./cmd/battle_worker"
  cat >/etc/systemd/system/mysecondwebgame-battle-worker.service <<EOF
[Unit]
Description=MySecondWebGame Battle Worker
After=network.target postgresql.service redis-server.service nats-server.service
Wants=postgresql.service redis-server.service nats-server.service

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${APP_DIR}/server
EnvironmentFile=/etc/mysecondwebgame/battle-worker.env
ExecStart=${APP_DIR}/bin/mysecondwebgame-battle-worker
Restart=always
RestartSec=3
LimitNOFILE=65535
MemoryMax=6G
TasksMax=2048
ExecStopPost=/bin/sh -c 'journalctl -u mysecondwebgame-battle-worker -n 200 --no-pager > /tmp/mysecondwebgame-battle-worker-last-stop.log || true'

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable mysecondwebgame-battle-worker
  systemctl restart mysecondwebgame-battle-worker
  log_step "Battle-worker role deployment completed"
}

deploy_kitty_worker() {
  log_step "Deploying kitty-worker role"
  apt_install golang-go
  preflight_backend_endpoints
  ensure_shared_game_data_artifacts
	write_kitty_worker_env_file
  mkdir -p "${APP_DIR}/bin"
  chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go mod tidy && go build -o '${APP_DIR}/bin/mysecondwebgame-kitty-worker' ./cmd/kitty_worker"
  cat >/etc/systemd/system/mysecondwebgame-kitty-worker.service <<EOF
[Unit]
Description=MySecondWebGame Kitty Worker
After=network.target postgresql.service redis-server.service
Wants=postgresql.service redis-server.service

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${APP_DIR}/server
EnvironmentFile=/etc/mysecondwebgame/kitty-worker.env
ExecStart=${APP_DIR}/bin/mysecondwebgame-kitty-worker
Restart=always
RestartSec=3
LimitNOFILE=65535
MemoryMax=2G
TasksMax=512
ExecStopPost=/bin/sh -c 'journalctl -u mysecondwebgame-kitty-worker -n 200 --no-pager > /tmp/mysecondwebgame-kitty-worker-last-stop.log || true'

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable mysecondwebgame-kitty-worker
  systemctl restart mysecondwebgame-kitty-worker
  log_step "Kitty-worker role deployment completed"
}

deploy_server_with_kitty_worker() {
	log_step "Deploying server-with-kitty-worker role"
	deploy_server
	deploy_action_worker
	deploy_kitty_worker
	health_check_server
	log_step "Server-with-kitty-worker role deployment completed"
}

deploy_web() {
  log_step "Deploying web static node"
  apt_install nginx
  ensure_web_node_runtime
  ensure_web_node_version
  log_step "Building web assets"
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
  log_step "Configuring nginx for static-only web serving"
  cat >/etc/nginx/sites-available/mysecondwebgame-web.conf <<EOF
map \$http_accept_encoding \$compressed_suffix {
    default "";
    "~*\\bbr\\b" ".br";
    "~*\\bgzip\\b" ".gz";
}

map \$http_accept_encoding \$compressed_encoding {
    default "";
    "~*\\bbr\\b" "br";
    "~*\\bgzip\\b" "gzip";
}

server {
    listen 80;
    server_name _;
    root ${APP_DIR}/web/dist;
    index index.html;

    gzip on;
    gzip_comp_level 6;
    gzip_min_length 1024;
    gzip_vary on;
    gzip_proxied any;
    gzip_types
        text/plain
        text/css
        text/xml
        application/json
        application/javascript
        application/xml+rss
        application/xml
        image/svg+xml
        font/ttf
        font/otf;

    location ~* ^/assets/.+\.js$ {
        default_type application/javascript;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        access_log off;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "public, max-age=2592000, immutable" always;
    }

    location ~* ^/assets/.+\.css$ {
        default_type text/css;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        access_log off;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "public, max-age=2592000, immutable" always;
    }

    location ~* ^/assets/.+\.json$ {
        default_type application/json;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        access_log off;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "public, max-age=2592000, immutable" always;
    }

    location ~* ^/assets/.+\.svg$ {
        default_type image/svg+xml;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        access_log off;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "public, max-age=2592000, immutable" always;
    }

    location /assets/ {
        try_files \$uri =404;
        access_log off;
        expires 30d;
        add_header Cache-Control "public, max-age=2592000, immutable";
    }

    location = /game-data/manifest.json {
        default_type application/json;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "no-cache, must-revalidate" always;
    }

    location ~* ^/game-data/icon-atlas/.+\.json$ {
        default_type application/json;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "no-cache, must-revalidate" always;
    }

    location ~* ^/game-data/assets/.+\.json$ {
        default_type application/json;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        access_log off;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "public, max-age=2592000, immutable" always;
    }

    location /game-data/assets/ {
        try_files \$uri =404;
        access_log off;
        expires 30d;
        add_header Cache-Control "public, max-age=2592000, immutable";
    }

    location = /index.html {
        default_type text/html;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "no-cache, must-revalidate" always;
    }

    location / {
        try_files \$uri \$uri/ /index.html;
    }
}
EOF
  ln -sf /etc/nginx/sites-available/mysecondwebgame-web.conf /etc/nginx/sites-enabled/mysecondwebgame-web.conf
  rm -f /etc/nginx/sites-enabled/default
  nginx -t
  systemctl enable nginx
  systemctl restart nginx
  log_step "Web role deployment completed"
}

health_check_server() {
  if curl -fsS "http://127.0.0.1:${APP_PORT}/readyz" >/dev/null; then
    echo "Server readiness check passed."
  else
    echo "Server readiness check failed."
    echo "---- mysecondwebgame-server service status ----"
    systemctl --no-pager --full status mysecondwebgame-server || true
    echo "---- recent mysecondwebgame-server logs ----"
    journalctl --no-pager -u mysecondwebgame-server -n 120 || true
    return 1
  fi
}

main() {
  fetch_repo_if_needed
  case "${ROLE}" in
    pg)
      deploy_pg
      ;;
    redis)
      deploy_redis
      ;;
    nats)
      deploy_nats
      ;;
    server)
      deploy_server
      deploy_action_worker
      health_check_server
      ;;
    action-worker)
      deploy_action_worker
      ;;
    battle-worker)
      deploy_battle_worker
      ;;
    kitty-worker)
      deploy_kitty_worker
      ;;
    server-with-kitty-worker)
      deploy_server_with_kitty_worker
      ;;
    web)
      deploy_web
      ;;
    lb|LB)
      deploy_lb
      ;;
    all)
      deploy_pg
      deploy_redis
      deploy_nats
      deploy_server
      deploy_action_worker
      deploy_battle_worker
      deploy_kitty_worker
      deploy_web
      health_check_server
      ;;
    *)
      echo "Unknown role: ${ROLE}"
      exit 1
      ;;
  esac
  echo "Native deployment complete for role: ${ROLE}"
}

main "$@"
