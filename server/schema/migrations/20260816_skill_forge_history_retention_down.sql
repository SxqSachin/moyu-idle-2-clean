BEGIN;

-- These additive retention structures are folded into init.sql. Rolling back
-- application code can safely leave them in place, while dropping them here
-- could remove schema still owned by a fresh baseline installation.

COMMIT;
