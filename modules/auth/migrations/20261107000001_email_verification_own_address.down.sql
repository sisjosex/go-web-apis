DELETE FROM auth.email_verification_tokens WHERE new_email IS NULL;
ALTER TABLE auth.email_verification_tokens ALTER COLUMN new_email SET NOT NULL;
