-- INV-014 A1 — backfill: los lotes que ya existían entran al stock.
--
-- Antes de A1 crear un lote no movía product_stock, así que todo lote anterior a
-- esta migración tiene saldo y no aparece en existencias. Medido en la base
-- local: los lotes suman 265 unidades que el stock no ve, mientras el stock
-- suma 279 que entraron por movimientos manuales. Los dos conjuntos no se
-- solapan — Polera 0001 tiene 40 en lotes y 0 en stock; Hamburguesa, 0 en lotes
-- y 50 en stock — así que son inventarios distintos, no el mismo contado dos
-- veces, y sumarlos es lo correcto (decisión del usuario, 2026-08-21).
--
-- Se registra por sp_record_movement, la misma puerta que usa A1, para que el
-- ledger siga reproduciendo product_stock. No se escribe product_stock a mano.
--
-- Se toma current_quantity y no initial_quantity: lo ya consumido no está.
--
-- IDEMPOTENTE. La condición es "el lote no tiene ningún movimiento asociado", así
-- que reejecutarla no vuelve a sumar, y un lote creado después de A1 — que nace
-- con su PURCHASE — nunca se toca aquí.

DO $$
DECLARE
    v_batch       RECORD;
    v_movement_id UUID;
    v_count       INT := 0;
    v_units       DECIMAL := 0;
BEGIN
    FOR v_batch IN
        SELECT pb.id, pb.tenant_id, pb.product_id, pb.sku_id,
               pb.current_quantity, pb.unit_cost, pb.lot_number
        FROM inventory.product_batches pb
        WHERE pb.current_quantity > 0
          AND pb.status <> 'void'
          AND NOT EXISTS (
              SELECT 1
              FROM inventory.inventory_movements im
              WHERE im.batch_id = pb.id
          )
        ORDER BY pb.created_at, pb.id
    LOOP
        -- unit_cost es NOT NULL y sp_create_batch lo exige > 0, pero un lote
        -- cargado por SQL directo podría no cumplirlo y PURCHASE lo requiere
        -- (INV-013 D3). Se salta en vez de abortar el backfill entero.
        IF v_batch.unit_cost IS NULL OR v_batch.unit_cost <= 0 THEN
            RAISE WARNING 'INV-014 backfill: lote % omitido, unit_cost invalido (%)',
                v_batch.lot_number, v_batch.unit_cost;
            CONTINUE;
        END IF;

        -- La fila de stock tiene que existir para que el movimiento aterrice;
        -- sp_record_movement actualiza sin insertar y no avisaria.
        IF NOT EXISTS (
            SELECT 1 FROM inventory.product_stock ps WHERE ps.sku_id = v_batch.sku_id
        ) THEN
            RAISE WARNING 'INV-014 backfill: lote % omitido, el SKU % no tiene fila de stock',
                v_batch.lot_number, v_batch.sku_id;
            CONTINUE;
        END IF;

        SELECT m.movement_id
        INTO v_movement_id
        FROM inventory.sp_record_movement(
            v_batch.tenant_id,
            v_batch.product_id,
            'PURCHASE',
            v_batch.current_quantity,
            'BATCH_BACKFILL',
            v_batch.id,
            v_batch.unit_cost,
            'INV-014 A1 — saldo del lote ' || v_batch.lot_number
                || ' incorporado a existencias',
            NULL,
            v_batch.sku_id
        ) m;

        UPDATE inventory.inventory_movements im
        SET batch_id = v_batch.id
        WHERE im.id = v_movement_id;

        v_count := v_count + 1;
        v_units := v_units + v_batch.current_quantity;
    END LOOP;

    RAISE NOTICE 'INV-014 backfill: % lotes incorporados, % unidades', v_count, v_units;
END;
$$;
