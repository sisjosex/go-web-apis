-- No-op on purpose.
--
-- The up migration flipped is_active to FALSE for every already soft-deleted
-- user. Nothing records which rows it touched, so re-activating them here would
-- also re-activate users that were legitimately disabled before being deleted.
-- Reverting has to be done by hand, per user.

SELECT 1;
