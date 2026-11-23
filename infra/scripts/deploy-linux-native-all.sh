#!/usr/bin/env bash
set -euo pipefail

# Native single-node deployment for:
# - nginx (static web + reverse proxy)
# - mysecondwebgame server (HTTP/WS gateway)
# - mysecondwebgame battle-worker
# - postgresql
# - redis
# - nats
#
# Target OS: Debian/Ubuntu with systemd + apt

REPO_URL="${1:-}"
APP_DIR="${2:-/opt/moyu-idle}"
BRANCH="${BRANCH:-main}"
APP_USER="${APP_USER:-mysecondwebgame}"
APP_GROUP="${APP_GROUP:-mysecondwebgame}"
APP_PORT="${APP_PORT:-8080}"
BATTLE_WORKER_PPROF_ADDR="${BATTLE_WORKER_PPROF_ADDR:-}"
WS_BATTLE_BATCH_COMPRESSION_ENABLED="${WS_BATTLE_BATCH_COMPRESSION_ENABLED:-false}"
PG_PORT="${PG_PORT:-5432}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
NATS_URL="${NATS_URL:-nats://127.0.0.1:4222}"
MQ_TOPIC_PREFIX="${MQ_TOPIC_PREFIX:-game}"
DB_NAME="${DB_NAME:-game}"
DB_USER="${DB_USER:-game}"
DB_PASSWORD="${DB_PASSWORD:-game}"
JWT_SECRET="${JWT_SECRET:-}"
GO_VERSION_CHECK="${GO_VERSION_CHECK:-1.22}"
INSTALL_DEPS="${INSTALL_DEPS:-1}"
WEB_NODE_REQUIRED_MAJOR="${WEB_NODE_REQUIRED_MAJOR:-22}"
WEB_NODE_REQUIRED_MINOR="${WEB_NODE_REQUIRED_MINOR:-14}"
REAL_IP_PROXY_HEADER="${REAL_IP_PROXY_HEADER:-X-Forwarded-For}"
REAL_IP_TRUSTED_PROXY_CHECK=true
REAL_IP_TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.1/32,::1/128
REAL_IP_ENABLE_VALIDATION=true

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root."
  exit 1
fi

if [[ -z "${JWT_SECRET}" ]]; then
  if command -v openssl >/dev/null 2>&1; then
    JWT_SECRET="$(openssl rand -hex 32)"
  else
    JWT_SECRET="replace-this-secret-before-production"
  fi
fi

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Missing command: ${cmd}"
    exit 1
  fi
}

log_step() {
  echo "[native-all] $*"
}

maybe_install_deps() {
  if [[ "${INSTALL_DEPS}" != "1" ]]; then
    return 0
  fi
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  apt-get install -y \
    ca-certificates \
    curl \
    git \
    jq \
    build-essential \
    golang-go \
    postgresql \
    postgresql-client \
    redis-server \
    nats-server \
    nginx
}

check_versions() {
  require_cmd git
  require_cmd go
  require_cmd psql
  require_cmd systemctl

  local go_ver
  go_ver="$(go version | awk '{print $3}' | sed 's/go//')"
  local go_major go_minor need_major need_minor
  go_major="$(echo "${go_ver}" | cut -d. -f1)"
  go_minor="$(echo "${go_ver}" | cut -d. -f2)"
  need_major="$(echo "${GO_VERSION_CHECK}" | cut -d. -f1)"
  need_minor="$(echo "${GO_VERSION_CHECK}" | cut -d. -f2)"
  if [[ "${go_major}" -lt "${need_major}" ]] || { [[ "${go_major}" -eq "${need_major}" ]] && [[ "${go_minor}" -lt "${need_minor}" ]]; }; then
    echo "Go ${GO_VERSION_CHECK}+ required, current: ${go_ver}"
    exit 1
  fi
}

ensure_web_node_version() {
  require_cmd node
  require_cmd npm
  local node_ver node_major node_minor
  node_ver="$(node -v | sed -E 's/^v//')"
  node_major="$(echo "${node_ver}" | cut -d. -f1)"
  node_minor="$(echo "${node_ver}" | cut -d. -f2)"
  if [[ "${node_major}" != "${WEB_NODE_REQUIRED_MAJOR}" ]] || [[ "${node_minor}" -lt "${WEB_NODE_REQUIRED_MINOR}" ]]; then
    echo "Node.js v${WEB_NODE_REQUIRED_MAJOR}.${WEB_NODE_REQUIRED_MINOR}+ is required for web build, current: v${node_ver}"
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
    echo "Node.js v${WEB_NODE_REQUIRED_MAJOR}.${WEB_NODE_REQUIRED_MINOR}+ is required for web build."
    echo "Current node: $(node -v 2>/dev/null || echo missing). Set INSTALL_DEPS=1 for auto install."
    exit 1
  fi

  log_step "Installing Node.js ${WEB_NODE_REQUIRED_MAJOR}.x runtime for web build"
  apt-get update -y
  apt-get install -y ca-certificates curl gnupg
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
  apt-get update -y
  desired_pkg_ver="$(apt-cache madison nodejs | awk -v major="${WEB_NODE_REQUIRED_MAJOR}" -v minor="${WEB_NODE_REQUIRED_MINOR}" '$3 ~ "^"major"\\."minor"\\." {print $3; exit}')"
  if [[ -n "${desired_pkg_ver}" ]]; then
    apt-get install -y "nodejs=${desired_pkg_ver}"
  else
    apt-get install -y nodejs
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

fetch_repo() {
  mkdir -p "$(dirname "${APP_DIR}")"
  if [[ -d "${APP_DIR}/.git" ]]; then
    git -C "${APP_DIR}" fetch --all --prune
    git -C "${APP_DIR}" checkout "${BRANCH}"
    git -C "${APP_DIR}" pull --ff-only origin "${BRANCH}"
  else
    if [[ -z "${REPO_URL}" ]]; then
      echo "REPO_URL is required for first-time deployment."
      echo "Usage: bash infra/scripts/deploy-linux-native-all.sh <repo_url> [app_dir]"
      exit 1
    fi
    git clone --branch "${BRANCH}" "${REPO_URL}" "${APP_DIR}"
  fi
  chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
}

configure_postgres() {
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
  PGPASSWORD="${DB_PASSWORD}" psql "postgresql://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${PG_PORT}/${DB_NAME}?sslmode=disable" -f "${APP_DIR}/server/schema/init.sql"
}

configure_infra_services() {
  if [[ -f /etc/redis/redis.conf ]]; then
    sed -i "s/^bind .*/bind 0.0.0.0 ::0/" /etc/redis/redis.conf
    sed -i "s/^protected-mode .*/protected-mode no/" /etc/redis/redis.conf
  fi
  systemctl enable redis-server nats-server
  systemctl restart redis-server nats-server
}

build_artifacts() {
  ensure_shared_game_data_artifacts
  ensure_web_node_runtime
  ensure_web_node_version
  log_step "Building web artifacts"
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
  log_step "Building server and battle-worker binaries"
  sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}/server' && go mod tidy && go build -o '${APP_DIR}/bin/mysecondwebgame-server' ./cmd/server && go build -o '${APP_DIR}/bin/mysecondwebgame-battle-worker' ./cmd/battle_worker"
}

write_env_files() {
  mkdir -p /etc/mysecondwebgame
  cat >/etc/mysecondwebgame/server.env <<EOF
APP_PORT=${APP_PORT}
DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${PG_PORT}/${DB_NAME}?sslmode=disable
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
NODE_ADVERTISE_ADDR=127.0.0.1:${APP_PORT}
NODE_REGISTRY_PREFIX=ws:backend:node:
NODE_HEARTBEAT_SECONDS=30
NODE_HEARTBEAT_INTERVAL_SECONDS=10
GAME_DATA_PATH=${APP_DIR}/packages/game-data/generated/game-data.json
GLOBAL_CONFIG_PATH=${APP_DIR}/packages/game-config/source/global-config.json
SHARED_GLOBAL_CONFIG_PATH=${APP_DIR}/packages/game-config/source/shared-global-config.json
JWT_SECRET=${JWT_SECRET}
REAL_IP_PROXY_HEADER=${REAL_IP_PROXY_HEADER}
REAL_IP_TRUSTED_PROXY_CHECK=${REAL_IP_TRUSTED_PROXY_CHECK}
REAL_IP_TRUSTED_PROXIES=${REAL_IP_TRUSTED_PROXIES}
REAL_IP_ENABLE_VALIDATION=${REAL_IP_ENABLE_VALIDATION}
EOF

  cp /etc/mysecondwebgame/server.env /etc/mysecondwebgame/battle-worker.env
  cat >>/etc/mysecondwebgame/battle-worker.env <<EOF
BATTLE_WORKER_PPROF_ADDR=${BATTLE_WORKER_PPROF_ADDR}
EOF
  chmod 600 /etc/mysecondwebgame/server.env /etc/mysecondwebgame/battle-worker.env
}

write_systemd_units() {
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

[Install]
WantedBy=multi-user.target
EOF

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

[Install]
WantedBy=multi-user.target
EOF
}

write_nginx_site() {
  cat >/etc/nginx/sites-available/mysecondwebgame.conf <<EOF
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

    location /api/ {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /ws {
        proxy_pass http://127.0.0.1:${APP_PORT}/ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location / {
        try_files \$uri \$uri/ /index.html;
    }

    location = /index.html {
        default_type text/html;
        gzip off;
        try_files \$uri\$compressed_suffix \$uri =404;
        add_header Content-Encoding \$compressed_encoding always;
        add_header Vary "Accept-Encoding" always;
        add_header Cache-Control "no-cache, must-revalidate" always;
    }
}
EOF

  ln -sf /etc/nginx/sites-available/mysecondwebgame.conf /etc/nginx/sites-enabled/mysecondwebgame.conf
  rm -f /etc/nginx/sites-enabled/default
  nginx -t
  systemctl enable nginx
  systemctl restart nginx
}

start_services() {
  systemctl daemon-reload
  systemctl enable mysecondwebgame-server mysecondwebgame-battle-worker
  systemctl restart mysecondwebgame-server mysecondwebgame-battle-worker
}

health_check() {
  sleep 2
  if curl -fsS "http://127.0.0.1:${APP_PORT}/readyz" >/dev/null; then
    echo "Readiness check passed: http://127.0.0.1:${APP_PORT}/readyz"
  else
    echo "Readiness check failed. Check logs:"
    echo "  journalctl -u mysecondwebgame-server -n 200 --no-pager"
    exit 1
  fi
}

main() {
  maybe_install_deps
  check_versions
  ensure_app_user
  fetch_repo
  configure_postgres
  configure_infra_services
  mkdir -p "${APP_DIR}/bin"
  chown -R "${APP_USER}:${APP_GROUP}" "${APP_DIR}"
  build_artifacts
  write_env_files
  write_systemd_units
  write_nginx_site
  start_services
  health_check

  cat <<EOF
Native single-node deployment complete.

Useful commands:
  systemctl status mysecondwebgame-server
  systemctl status mysecondwebgame-battle-worker
  journalctl -u mysecondwebgame-server -f
  journalctl -u mysecondwebgame-battle-worker -f
EOF
}

main "$@"
