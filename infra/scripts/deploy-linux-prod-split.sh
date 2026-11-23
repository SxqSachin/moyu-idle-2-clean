#!/usr/bin/env bash
set -euo pipefail

ROLE="${1:-}"
REPO_URL="${2:-}"
APP_DIR="${3:-MySecondWebGame}"
ENV_FILE="${4:-infra/.env.prod}"

if [ -z "$ROLE" ]; then
  echo "Usage: bash infra/scripts/deploy-linux-prod-split.sh <role> [repo_url] [app_dir] [env_file]"
  echo "role: pg | redis | nats | server | action-worker | battle-worker | web | nginx | all"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found on server."
  exit 1
fi
if ! command -v git >/dev/null 2>&1; then
  echo "git not found on server."
  exit 1
fi

if [ -n "$REPO_URL" ]; then
  if [ -d "$APP_DIR/.git" ]; then
    cd "$APP_DIR"
    git pull --ff-only
  else
    git clone "$REPO_URL" "$APP_DIR"
    cd "$APP_DIR"
  fi
else
  cd "$(dirname "${BASH_SOURCE[0]}")/../.."
fi

if [ ! -f "$ENV_FILE" ]; then
  cp -n infra/.env.prod.example "$ENV_FILE" || true
  echo "Missing env file: $ENV_FILE"
  echo "Created from infra/.env.prod.example, please edit it first and rerun."
  exit 1
fi

compose_up() {
  local file="$1"
  docker compose --env-file "$ENV_FILE" -f "$file" up -d --build
}

apply_pg_schema() {
  bash infra/scripts/apply-schema-sql.sh init.sql infra/compose/pg.yml "$ENV_FILE"
}

case "$ROLE" in
  pg)
    compose_up infra/compose/pg.yml
    apply_pg_schema
    ;;
  redis)
    compose_up infra/compose/redis.yml
    ;;
  nats)
    compose_up infra/compose/nats.yml
    ;;
  server)
    compose_up infra/compose/server.yml
    compose_up infra/compose/action-worker.yml
    ;;
  action-worker)
    compose_up infra/compose/action-worker.yml
    ;;
  battle-worker)
    compose_up infra/compose/battle-worker.yml
    ;;
  web)
    compose_up infra/compose/web.yml
    ;;
  nginx)
    compose_up infra/compose/nginx-lb.yml
    echo "Nginx LB deployed."
    echo "Next, run upstream sync:"
    echo "  REDIS_ADDR=<redis_host:6379> NODE_KEY_PREFIX=ws:backend:node: SYNC_INTERVAL_SECONDS=15 \\"
    echo "  HEALTHCHECK_ENABLED=true HEALTHCHECK_PATH=/readyz \\"
    echo "  NGINX_RELOAD_CMD='docker exec game-nginx-lb nginx -t && docker exec game-nginx-lb nginx -s reload' \\"
    echo "  bash infra/scripts/sync-nginx-upstreams.sh"
    ;;
  all)
    compose_up infra/compose/pg.yml
    apply_pg_schema
    compose_up infra/compose/redis.yml
    compose_up infra/compose/nats.yml
    compose_up infra/compose/server.yml
    compose_up infra/compose/action-worker.yml
    compose_up infra/compose/battle-worker.yml
    compose_up infra/compose/web.yml
    compose_up infra/compose/nginx-lb.yml
    ;;
  *)
    echo "Unknown role: $ROLE"
    exit 1
    ;;
esac

echo "Deployment complete for role: $ROLE"
