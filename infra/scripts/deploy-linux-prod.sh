#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${1:-}"
APP_DIR="${2:-MySecondWebGame}"

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

cp -n infra/.env.example infra/.env || true
docker compose -f infra/docker-compose.prod.yml up -d --build

echo "Deployment complete."
echo "Remember to set strong secrets in infra/docker-compose.prod.yml (JWT_SECRET, DB password)."
