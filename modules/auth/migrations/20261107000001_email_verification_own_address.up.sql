-- INFRA-014: asking to verify the account's own address sends no email, so the token carries no
-- new_email — sp_generate_email_verification_token inserts NULL and sp_verify_email keeps the current
-- address on NULL. The column's NOT NULL refused exactly that call, the one the app makes.
ALTER TABLE auth.email_verification_tokens ALTER COLUMN new_email DROP NOT NULL;
