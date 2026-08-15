-- Reverses 20260811000001. sp_list_movements is new in that migration — there is no
-- earlier signature to restore, so a plain drop is the full reversal.

DROP FUNCTION IF EXISTS inventory.sp_list_movements(UUID, UUID, VARCHAR, DATE, DATE, INT, INT);
