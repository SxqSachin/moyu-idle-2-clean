BEGIN;

-- Book-count policy is configuration-owned. Reintroducing the retired 500-book
-- database constraint would reject valid drafts created by the current rules.

COMMIT;
