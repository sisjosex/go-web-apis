-- Reverse of TRACK-017 step 2: the guardian scope did not exist before it. The SPs that call it are
-- reverted by their own down migrations, which run first.

DROP FUNCTION IF EXISTS tracking.fn_guardian_scope(UUID, UUID);
