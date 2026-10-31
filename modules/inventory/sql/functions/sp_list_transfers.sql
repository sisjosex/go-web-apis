CREATE OR REPLACE FUNCTION inventory.sp_list_transfers(p_tenant_id uuid, p_status character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, number character varying, status character varying, from_name character varying, to_name character varying, units numeric, prepared_at timestamp with time zone, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT t.id, t.number, t.status, wf.name, wt.name,
           (SELECT COALESCE(SUM(l.quantity), 0) FROM inventory.transfer_lines l WHERE l.transfer_id = t.id),
           t.prepared_at, COUNT(*) OVER ()
    FROM inventory.transfers t
    INNER JOIN inventory.warehouses wf ON wf.id = t.from_warehouse_id
    INNER JOIN inventory.warehouses wt ON wt.id = t.to_warehouse_id
    WHERE t.tenant_id = p_tenant_id AND (p_status IS NULL OR p_status = '' OR t.status = p_status)
    ORDER BY t.prepared_at DESC
    LIMIT p_page_size OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_transfers(uuid,character varying,integer,integer) IS 'A page of transfers, newest first, by status (INV-025).';
