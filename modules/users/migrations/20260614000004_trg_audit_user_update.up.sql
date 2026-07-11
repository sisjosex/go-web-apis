-- Trigger function: build field-level diff for auth.users updates and insert
-- one audit row into users.user_audit_log. tenant_id and performed_by are
-- passed via session-local variables set by the Go repository within the same
-- transaction (is_local = true).

CREATE OR REPLACE FUNCTION users.trg_fn_audit_user_update()
RETURNS TRIGGER AS $$
DECLARE
    l_tenant_id     UUID;
    l_performed_by  UUID;
    l_tenant_str    TEXT;
    l_performer_str TEXT;
    l_changed       JSONB := '[]';
    l_before        JSONB := '{}';
    l_after         JSONB := '{}';
    l_action        VARCHAR(30);
BEGIN
    -- Read session-local variables injected by the Go repository.
    l_tenant_str    := NULLIF(current_setting('app.tenant_id', TRUE), '');
    l_performer_str := NULLIF(current_setting('app.performed_by', TRUE), '');

    -- Skip when called from migrations or direct DB scripts (no tenant context).
    IF l_tenant_str IS NULL THEN
        RETURN NEW;
    END IF;

    l_tenant_id    := l_tenant_str::UUID;
    l_performed_by := l_performer_str::UUID;

    -- Skip soft-delete updates; the controller records "user.deleted" separately.
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    -- Build diff arrays/objects only for tracked fields.

    IF OLD.first_name IS DISTINCT FROM NEW.first_name THEN
        l_changed  := l_changed  || to_jsonb('first_name'::TEXT);
        l_before   := l_before   || jsonb_build_object('first_name', OLD.first_name);
        l_after    := l_after    || jsonb_build_object('first_name', NEW.first_name);
    END IF;

    IF OLD.last_name IS DISTINCT FROM NEW.last_name THEN
        l_changed  := l_changed  || to_jsonb('last_name'::TEXT);
        l_before   := l_before   || jsonb_build_object('last_name', OLD.last_name);
        l_after    := l_after    || jsonb_build_object('last_name', NEW.last_name);
    END IF;

    IF OLD.email IS DISTINCT FROM NEW.email THEN
        l_changed  := l_changed  || to_jsonb('email'::TEXT);
        l_before   := l_before   || jsonb_build_object('email', OLD.email);
        l_after    := l_after    || jsonb_build_object('email', NEW.email);
    END IF;

    IF OLD.phone IS DISTINCT FROM NEW.phone THEN
        l_changed  := l_changed  || to_jsonb('phone'::TEXT);
        l_before   := l_before   || jsonb_build_object('phone', OLD.phone);
        l_after    := l_after    || jsonb_build_object('phone', NEW.phone);
    END IF;

    IF OLD.birthday IS DISTINCT FROM NEW.birthday THEN
        l_changed  := l_changed  || to_jsonb('birthday'::TEXT);
        l_before   := l_before   || jsonb_build_object('birthday', OLD.birthday);
        l_after    := l_after    || jsonb_build_object('birthday', NEW.birthday);
    END IF;

    IF OLD.is_active IS DISTINCT FROM NEW.is_active THEN
        l_changed  := l_changed  || to_jsonb('is_active'::TEXT);
        l_before   := l_before   || jsonb_build_object('is_active', OLD.is_active);
        l_after    := l_after    || jsonb_build_object('is_active', NEW.is_active);
    END IF;

    IF OLD.expiration_date IS DISTINCT FROM NEW.expiration_date THEN
        l_changed  := l_changed  || to_jsonb('expiration_date'::TEXT);
        l_before   := l_before   || jsonb_build_object('expiration_date', OLD.expiration_date);
        l_after    := l_after    || jsonb_build_object('expiration_date', NEW.expiration_date);
    END IF;

    IF OLD.password IS DISTINCT FROM NEW.password THEN
        l_changed  := l_changed  || to_jsonb('password'::TEXT);
        l_before   := l_before   || jsonb_build_object('password', '[redacted]'::TEXT);
        l_after    := l_after    || jsonb_build_object('password', '[redacted]'::TEXT);
    END IF;

    -- Nothing auditable changed — skip insert.
    IF jsonb_array_length(l_changed) = 0 THEN
        RETURN NEW;
    END IF;

    -- Derive action from is_active change; fall back to generic update.
    IF OLD.is_active IS DISTINCT FROM NEW.is_active THEN
        IF NEW.is_active THEN
            l_action := 'user.activated';
        ELSE
            l_action := 'user.deactivated';
        END IF;
    ELSE
        l_action := 'user.updated';
    END IF;

    INSERT INTO users.user_audit_log (
        tenant_id,
        action,
        target_user_id,
        performed_by,
        metadata,
        source
    ) VALUES (
        l_tenant_id,
        l_action,
        NEW.id,
        l_performed_by,
        jsonb_build_object(
            'changed', l_changed,
            'before',  l_before,
            'after',   l_after
        ),
        'user'
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_user_update
    AFTER UPDATE ON auth.users
    FOR EACH ROW
    EXECUTE FUNCTION users.trg_fn_audit_user_update();

COMMENT ON FUNCTION users.trg_fn_audit_user_update IS 'Records field-level diffs for auth.users updates into users.user_audit_log';
