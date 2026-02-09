-- Add email_verified field to users_v2
-- Migration: 003_add_email_verified

ALTER TABLE users_v2 
ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT false;

-- Set existing users as verified
UPDATE users_v2 SET email_verified = true WHERE email_verified IS NULL;

-- Add comment
COMMENT ON COLUMN users_v2.email_verified IS 'Whether user email has been verified';

