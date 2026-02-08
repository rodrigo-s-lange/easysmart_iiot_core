-- Reset database keeping only the device with this token
-- Update token if needed
DO $$
DECLARE
    v_device_id UUID;
    v_user_id UUID;
    v_token TEXT := '8f8835f1-70c3-4cbd-b4c0-9acb6826c641';
BEGIN
    SELECT id, user_id INTO v_device_id, v_user_id
    FROM devices
    WHERE token::text = v_token
    LIMIT 1;

    IF v_device_id IS NULL THEN
        RAISE EXCEPTION 'Device token not found: %', v_token;
    END IF;

    -- Clear volatile tables
    TRUNCATE telemetry CASCADE;
    TRUNCATE commands, device_slots, device_rate_limits;

    -- Keep only the selected device and its owner
    DELETE FROM devices WHERE id <> v_device_id;
    DELETE FROM users WHERE id <> v_user_id;
END $$;
