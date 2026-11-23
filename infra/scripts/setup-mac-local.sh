#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found. Please install Docker Desktop first."
  exit 1
fi
if ! command -v node >/dev/null 2>&1; then
  echo "node not found. Please install Node.js first."
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "go not found. Please install Go first."
  exit 1
fi

cp -n infra/.env.example infra/.env || true
bash infra/scripts/pull-images.sh
docker compose -f infra/docker-compose.yml up -d
bash infra/scripts/init-db-local.sh
node packages/game-data/scripts/build.mjs

echo "Local infra is ready."
echo "Run gateway:  cd server && cp -n .env.example .env && go mod tidy && go run ./cmd/server"
echo "Run actions:  cd server && cp -n .env.example .env && go run ./cmd/action_worker"
echo "Run frontend: cd web && npm install && npm run dev"
