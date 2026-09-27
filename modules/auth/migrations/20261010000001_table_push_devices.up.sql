-- TRACK-012 step 2 (D3): where a phone receives pushes. One row per account and device, in the main
-- database: a guardian with riders in two operators registers the phone once, and the worker's push
-- task reads it from its own pool whatever tenant the notice came from.
--
-- token is FCM's registration token for that install. It rotates, so the phone re-sends it on every
-- start and the upsert refreshes it; a token FCM answers UNREGISTERED is deleted by the push task.
-- idx_push_devices_token backs that delete, and keeps one install's token from sitting under two
-- accounts: registering it for a new account takes it from the old one.

CREATE TABLE auth.push_devices (
    user_id      UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    device_id    VARCHAR(100) NOT NULL,
    platform     VARCHAR(10) NOT NULL,
    token        TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, device_id),
    CONSTRAINT chk_push_devices_platform CHECK (platform IN ('android', 'ios'))
);

CREATE UNIQUE INDEX idx_push_devices_token ON auth.push_devices (token);

COMMENT ON TABLE auth.push_devices IS
'One FCM registration token per account and device; refreshed by POST /mobile/devices, deleted on logout or when FCM answers UNREGISTERED (TRACK-012 D3)';

-- Registers or refreshes a device. The token leaves any other account or device it sat under first:
-- one install is one recipient.
CREATE FUNCTION auth.sp_upsert_push_device(
    p_user_id UUID,
    p_device_id VARCHAR,
    p_platform VARCHAR,
    p_token TEXT
)
RETURNS VOID AS $$
BEGIN
    DELETE FROM auth.push_devices d
    WHERE d.token = p_token AND (d.user_id, d.device_id) <> (p_user_id, p_device_id);

    INSERT INTO auth.push_devices (user_id, device_id, platform, token, last_seen_at)
    VALUES (p_user_id, p_device_id, p_platform, p_token, now())
    ON CONFLICT (user_id, device_id) DO UPDATE
    SET platform = EXCLUDED.platform,
        token = EXCLUDED.token,
        last_seen_at = now();
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION auth.sp_upsert_push_device(UUID, VARCHAR, VARCHAR, TEXT) IS
'Registers or refreshes an account''s device token; the same token under another account or device is moved, not duplicated (TRACK-012)';

-- Forgets one of the account's devices; an unknown device is not an error — logout is idempotent.
CREATE FUNCTION auth.sp_delete_push_device(
    p_user_id UUID,
    p_device_id VARCHAR
)
RETURNS VOID AS $$
    DELETE FROM auth.push_devices d WHERE d.user_id = p_user_id AND d.device_id = p_device_id;
$$ LANGUAGE sql;

COMMENT ON FUNCTION auth.sp_delete_push_device(UUID, VARCHAR) IS
'Forgets one of an account''s devices; idempotent (TRACK-012)';

-- The push task's two statements: an account's tokens, and forgetting one FCM refused for good.
CREATE FUNCTION auth.sp_push_tokens(
    p_user_id UUID
)
RETURNS TABLE(device_id VARCHAR, platform VARCHAR, token TEXT) AS $$
    SELECT d.device_id, d.platform, d.token FROM auth.push_devices d WHERE d.user_id = p_user_id;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION auth.sp_push_tokens(UUID) IS
'Every push token of an account (TRACK-012)';

CREATE FUNCTION auth.sp_forget_push_token(
    p_token TEXT
)
RETURNS VOID AS $$
    DELETE FROM auth.push_devices d WHERE d.token = p_token;
$$ LANGUAGE sql;

COMMENT ON FUNCTION auth.sp_forget_push_token(TEXT) IS
'Deletes a token FCM answered UNREGISTERED (TRACK-012)';
