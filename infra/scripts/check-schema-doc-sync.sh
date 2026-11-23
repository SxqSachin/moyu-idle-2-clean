#!/usr/bin/env bash
set -euo pipefail

BASE_REF="${1:-origin/${GITHUB_BASE_REF:-main}}"

if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
  git fetch --no-tags --depth=1 origin "${GITHUB_BASE_REF:-main}:${GITHUB_BASE_REF:-main}" >/dev/null 2>&1 || true
  if git rev-parse --verify "${GITHUB_BASE_REF:-main}" >/dev/null 2>&1; then
    BASE_REF="${GITHUB_BASE_REF:-main}"
  fi
fi

if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
  if git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
    BASE_REF="HEAD~1"
  else
    echo "schema-doc-check: skip (unable to resolve base ref and no previous commit)."
    exit 0
  fi
fi

SCHEMA_CHANGED="$(git diff --name-only "$BASE_REF"...HEAD -- 'server/schema/init.sql' | wc -l | tr -d ' ')"
DOC_CHANGED="$(git diff --name-only "$BASE_REF"...HEAD -- 'docs/backend/database-schema.md' | wc -l | tr -d ' ')"

if [[ "$SCHEMA_CHANGED" -gt 0 && "$DOC_CHANGED" -eq 0 ]]; then
  echo "schema-doc-check: schema changed but docs/backend/database-schema.md was not updated."
  echo "Please update docs/backend/database-schema.md in the same PR."
  exit 1
fi

echo "schema-doc-check: pass"
