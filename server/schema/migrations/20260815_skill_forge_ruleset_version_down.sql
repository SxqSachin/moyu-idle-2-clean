BEGIN;

-- ruleset_version is part of the base forge schema and is required to disable
-- unsafe generated skills. Only the base forge rollback may remove it.

COMMIT;
