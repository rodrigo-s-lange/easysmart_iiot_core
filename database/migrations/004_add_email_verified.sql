-- Add email_verified field to users
-- Migration: 004_add_email_verified

ALTER TABLE users 
ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT false;

-- Set existing users as verified
UPDATE users SET email_verified = true WHERE email_verified IS NULL;

-- Add comment
COMMENT ON COLUMN users.email_verified IS 'Whether user email has been verified';
