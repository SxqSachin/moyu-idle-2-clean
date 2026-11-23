BEGIN;

-- The bounded state-read indexes are folded into the base forge schema. They
-- remain useful and harmless across an incremental application rollback.

COMMIT;
