-- Revierte el backfill de INV-014 A1: saca del stock lo que la subida metió.
--
-- Se identifica por reference_type = 'BATCH_BACKFILL', que sólo escribe esa
-- migración, y se compensa con un movimiento inverso en lugar de borrar el
-- original: los movimientos son hechos y product_stock se deriva de su suma, así
-- que borrarlos dejaría el stock contradiciendo su propio ledger.
--
-- Un ADJUSTMENT OUT, no un DELETE. Es también la razón de que bajar y volver a
-- subir sea seguro: el backfill se salta los lotes que ya tienen movimiento, y
-- tras esta compensación el lote sigue teniéndolo, así que no se vuelve a sumar.
-- Si de verdad se quiere reejecutar hay que borrar ambos movimientos a mano.

DO $$
DECLARE
    v_movement RECORD;
BEGIN
    FOR v_movement IN
        SELECT im.id, im.tenant_id, im.product_id, im.sku_id, im.quantity,
               im.batch_id
        FROM inventory.inventory_movements im
        WHERE im.reference_type = 'BATCH_BACKFILL'
          AND NOT EXISTS (
              SELECT 1
              FROM inventory.inventory_movements c
              WHERE c.reference_type = 'BATCH_BACKFILL_REVERT'
                AND c.reference_id = im.id
          )
        ORDER BY im.created_at, im.id
    LOOP
        PERFORM inventory.sp_record_movement(
            v_movement.tenant_id,
            v_movement.product_id,
            'ADJUSTMENT',
            ABS(v_movement.quantity),
            'BATCH_BACKFILL_REVERT',
            v_movement.id,
            NULL,
            'INV-014 A1 — backfill revertido',
            NULL,
            v_movement.sku_id,
            'OUT'
        );
    END LOOP;
END;
$$;
