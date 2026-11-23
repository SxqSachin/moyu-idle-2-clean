#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

SERVER_IMAGE="${SERVER_IMAGE:-mysecondwebgame-server:latest}"
BATTLE_WORKER_IMAGE="${BATTLE_WORKER_IMAGE:-mysecondwebgame-battle-worker:latest}"
WEB_IMAGE="${WEB_IMAGE:-mysecondwebgame-web:latest}"

echo "[build] server image: $SERVER_IMAGE"
docker build -f server/Dockerfile -t "$SERVER_IMAGE" server

echo "[build] battle worker image: $BATTLE_WORKER_IMAGE"
docker build -f server/Dockerfile.battle-worker -t "$BATTLE_WORKER_IMAGE" server

echo "[build] web image: $WEB_IMAGE"
docker build -t "$WEB_IMAGE" web

echo "Build complete."
