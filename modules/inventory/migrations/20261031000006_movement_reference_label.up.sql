-- INV-021 D3: the document's name as the movement's creator wrote it, so the ledger joins nothing to show it.
-- No backfill: INV-028 emptied the older rows.
ALTER TABLE inventory.inventory_movements ADD COLUMN reference_label VARCHAR(120);
