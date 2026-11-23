BEGIN;

-- These objects were folded into 20260814_player_custom_skill_forge.sql before
-- release. That base migration exclusively owns their rollback; this upgrade
-- shim must not remove a table or columns that may contain live forge data.

COMMIT;
