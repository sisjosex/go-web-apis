-- No-op on purpose.
--
-- The up migration deactivated the memberships of already soft-deleted users.
-- Nothing records which rows it touched, so re-activating them here would also
-- re-activate memberships that had been removed on purpose before the user was
-- deleted. Reverting has to be done by hand, per membership.

SELECT 1;
