DROP INDEX IF EXISTS users_firebase_uid_unique;
DROP INDEX IF EXISTS users_phone_lookup_unique;

DELETE FROM users WHERE phone_lookup IS NULL OR phone_ciphertext IS NULL;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_email_len,
    DROP CONSTRAINT IF EXISTS users_firebase_uid_len,
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS firebase_uid,
    DROP COLUMN IF EXISTS phone_verified,
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE users
    ALTER COLUMN phone_ciphertext SET NOT NULL,
    ALTER COLUMN phone_lookup SET NOT NULL;

ALTER TABLE users ADD CONSTRAINT users_phone_lookup_unique UNIQUE (phone_lookup);
