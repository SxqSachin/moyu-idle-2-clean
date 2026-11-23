# MySecondWebGame

## Collaboration

Use only the primary agent unless the user explicitly requests subagents in the current conversation. This applies to every model, reasoning level, and descendant directory. Task complexity, time savings, and generic skill recommendations do not grant delegation permission.

## Project Contracts

- Source game data lives in `packages/game-data/source/json`; generated files are not the editing authority. Follow the existing schema and validators before extending DSL or table support.
- Trace data -> runtime owner -> player UI -> persistence/migration for cross-layer changes. Preserve settlement, replay, rewards, frozen snapshots, ordering, and idempotency unless the task explicitly includes a compatible change.
- Keep changes scoped to the requested behavior. Use existing framework and owner boundaries rather than adding parallel implementations or speculative processing chains.

## Focused Verification

Choose checks for the changed behavior; do not run this as a mandatory full checklist:

- Game-data source changes: `node packages/game-data/scripts/build.mjs` from the repository root.
- Server: `go -C server test ./internal/<affected-package> -run '<relevant-test>'`; the Go module is `server/`, not the repository root.
- Web logic: `npm --prefix web run test:unit -- <test-file>`; type changes may also need `npm --prefix web run typecheck`.
- Meaningful UI changes: inspect the affected player flow and rendered states with the existing browser harness. Headless logic tests alone do not prove visual correctness.

Read the current package scripts before broader builds. Review the final diff and affected call paths, fix concrete findings, and state validation limits. Documentation-only changes need instruction/link checks, not game runtime tests.
