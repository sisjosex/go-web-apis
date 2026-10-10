CREATE OR REPLACE FUNCTION public.fn_within_limit(p_tenant_id uuid, p_used bigint, p_limit integer, p_feature text)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF p_limit IS NOT NULL AND p_limit >= 0 AND p_used >= p_limit THEN
        RAISE EXCEPTION 'billing.limit-reached'
            USING ERRCODE = 'P0001',
                  DETAIL = json_build_object('feature', p_feature, 'limit', p_limit, 'used', p_used)::TEXT;
    END IF;
    RETURN p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION fn_within_limit(uuid,bigint,integer,text) IS 'Answers p_tenant_id while p_used is under p_limit (NULL or -1: unlimited); else raises billing.limit-reached with {feature, limit, used} in DETAIL (BILLING-001 D2)';
