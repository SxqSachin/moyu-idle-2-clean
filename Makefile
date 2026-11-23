.PHONY: harness-pr game-data schema-doc-check server-test web-build web-test-unit web-test-e2e local-init-db dev-reset-db local-cleanup-db admin-up admin-init-db admin-refresh-config admin-reset-password admin-sync-schema

game-data:
	node packages/game-data/scripts/build.mjs

schema-doc-check:
	bash infra/scripts/check-schema-doc-sync.sh

server-test:
	cd server && go mod tidy && go test ./...

web-build:
	cd web && npm run build

web-test-unit:
	cd web && npm run test:unit

web-test-e2e:
	cd web && npm run test:e2e

harness-pr: game-data schema-doc-check server-test web-build web-test-unit web-test-e2e

local-init-db:
	bash infra/scripts/init-db-local.sh

dev-reset-db:
	bash infra/scripts/reset-dev-db.sh --yes

local-cleanup-db: dev-reset-db
