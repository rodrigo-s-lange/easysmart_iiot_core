-- Add claim code + secret delivery tracking for device provisioning (Option A)

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS claim_code_hash VARCHAR(255),
    ADD COLUMN IF NOT EXISTS secret_delivered_at TIMESTAMPTZ;

-- Backfill claim_code_hash for existing devices (temporary: claim_code = device_label)
-- NOTE: For production, rotate claim codes and do NOT reuse device_label.
UPDATE devices
SET claim_code_hash = crypt(device_label, gen_salt('bf', 12))
WHERE claim_code_hash IS NULL
  AND device_label IS NOT NULL;
