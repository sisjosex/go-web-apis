-- INV-028: one-time purge of prior inventory, sales and purchase data, run by hand per database.
-- Not a migration: nothing runs it automatically. Without -v confirm=purge it only prints the counts.
--
--   psql -f scripts/purge_inventory_data.sql -d "$URL"                    counts, nothing changes
--   psql -v confirm=purge -f scripts/purge_inventory_data.sql -d "$URL"   counts, purge, counts
--
-- Keeps sales.customers, purchasing.suppliers, tenancy, users and tracking. TRUNCATE has no CASCADE on
-- purpose: a table outside this list referencing one of them fails and the whole purge rolls back.

\set ON_ERROR_STOP on
\set QUIET on
SET client_min_messages = notice;

CREATE TEMP TABLE purge_tables (name text PRIMARY KEY) ON COMMIT PRESERVE ROWS;
INSERT INTO purge_tables VALUES
    ('inventory.batch_movements'), ('inventory.inventory_movements'), ('inventory.product_batches'),
    ('inventory.product_categories'), ('inventory.product_category_mapping'), ('inventory.product_media'),
    ('inventory.product_sku_options'), ('inventory.product_skus'), ('inventory.product_stock'),
    ('inventory.product_variant_groups'), ('inventory.product_variant_options'), ('inventory.products'),
    ('sales.order_batch_assignments'), ('sales.order_items'), ('sales.payments'), ('sales.returns'),
    ('sales.sales_orders'),
    ('purchasing.product_batches'), ('purchasing.purchase_order_invoices'), ('purchasing.purchase_order_items'),
    ('purchasing.purchase_order_receipts'), ('purchasing.purchase_orders'), ('purchasing.request_for_quotes'),
    ('purchasing.rfq_items'), ('purchasing.rfq_responses'), ('purchasing.supplier_products'),
    ('purchasing.supplier_rates');

CREATE FUNCTION pg_temp.print_counts(p_title text) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_name  text;
    v_count bigint;
BEGIN
    RAISE NOTICE '--- % (database %)', p_title, current_database();
    FOR v_name IN SELECT name FROM purge_tables ORDER BY name LOOP
        IF to_regclass(v_name) IS NULL THEN
            RAISE NOTICE '% absent', rpad(v_name, 40);
        ELSE
            EXECUTE format('SELECT count(*) FROM %s', v_name) INTO v_count;
            RAISE NOTICE '% %', rpad(v_name, 40), v_count;
        END IF;
    END LOOP;
END;
$$;

SELECT pg_temp.print_counts('rows per table') \g /dev/null

\echo '--- tables outside the list that reference one of them (each one makes the TRUNCATE fail)'
SELECT c.conrelid::regclass AS referencing_table, c.confrelid::regclass AS referenced_table, c.conname AS constraint_name
  FROM pg_constraint c
 WHERE c.contype = 'f'
   AND c.confrelid::regclass::text IN (SELECT name FROM purge_tables)
   AND c.conrelid::regclass::text NOT IN (SELECT name FROM purge_tables)
 ORDER BY 1, 2;

\if :{?confirm}
BEGIN;

DO $$
DECLARE
    v_list text;
BEGIN
    SELECT string_agg(name, ', ') INTO v_list FROM purge_tables WHERE to_regclass(name) IS NOT NULL;
    IF v_list IS NULL THEN
        RAISE NOTICE 'none of the tables exist here, nothing to purge';
        RETURN;
    END IF;
    EXECUTE 'TRUNCATE ' || v_list;
END;
$$;

SELECT pg_temp.print_counts('rows after the purge') \g /dev/null

COMMIT;
\echo 'purged and committed'
\else
\echo 'counts only, nothing changed. Run with -v confirm=purge to purge.'
\endif
