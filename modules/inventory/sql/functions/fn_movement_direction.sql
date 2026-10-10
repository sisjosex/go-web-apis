CREATE OR REPLACE FUNCTION inventory.fn_movement_direction(p_movement_type character varying, p_direction character varying DEFAULT NULL::character varying)
 RETURNS integer
 LANGUAGE plpgsql
 IMMUTABLE
AS $function$
DECLARE
    v_decided VARCHAR;
BEGIN
    -- The five types whose direction is not a question.
    v_decided := CASE p_movement_type
        WHEN 'PURCHASE'   THEN 'IN'
        WHEN 'RETURN'     THEN 'IN'
        WHEN 'PRODUCTION' THEN 'IN'
        WHEN 'SALE'       THEN 'OUT'
        WHEN 'WASTE'      THEN 'OUT'
        ELSE NULL
    END;

    IF v_decided IS NOT NULL THEN
        -- A direction that agrees is accepted rather than refused: the modal
        -- shows the decided one as read-only text and may well send it back.
        IF p_direction IS NOT NULL AND p_direction <> v_decided THEN
            RAISE EXCEPTION 'inventory.movement.direction-conflict'
                USING ERRCODE = 'P0001';
        END IF;
        RETURN CASE WHEN v_decided = 'IN' THEN 1 ELSE -1 END;
    END IF;

    -- ADJUSTMENT and TRANSFER: both readings are legitimate, so the caller says
    -- which. Defaulting either one is how a stock count silently goes the wrong
    -- way, so there is no default.
    IF p_direction IS NULL THEN
        RAISE EXCEPTION 'inventory.movement.direction-required'
            USING ERRCODE = 'P0001';
    END IF;

    IF p_direction NOT IN ('IN', 'OUT') THEN
        RAISE EXCEPTION 'inventory.movement.direction-conflict'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN CASE WHEN p_direction = 'IN' THEN 1 ELSE -1 END;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_movement_direction(character varying,character varying) IS 'The sign a movement applies to stock: +1 inbound, -1 outbound (INV-013 D1). PURCHASE, RETURN and PRODUCTION are always IN; SALE and WASTE always OUT, and a p_direction contradicting one of those raises inventory.movement.direction-conflict. ADJUSTMENT and TRANSFER carry no default and raise inventory.movement.direction-required when p_direction is NULL. An unknown movement type reaches here as an undirected one — callers validate the type first.';
