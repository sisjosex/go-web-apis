-- Unassign Rider
CREATE OR REPLACE FUNCTION tracking.sp_unassign_rider(
    p_assignment_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_exists BOOLEAN;
BEGIN
    -- Check if assignment exists
    SELECT EXISTS(SELECT 1 FROM tracking.rider_assignments WHERE id = p_assignment_id)
    INTO v_exists;

    IF NOT v_exists THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Update status to inactive instead of deleting
    UPDATE tracking.rider_assignments
    SET 
        status = 'inactive',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_assignment_id;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
