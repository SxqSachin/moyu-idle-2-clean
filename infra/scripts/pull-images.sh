#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

cp -n infra/.env.example infra/.env || true
docker compose -f infra/docker-compose.yml pull pg redis nats
echo "Base images pulled."
