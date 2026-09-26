-- Google sign-in leaves phone empty. Existing OTP users stay verified.
-- Placeholders cannot sit in phone_lookup: the format check requires 64 hex characters.
-- Soft delete clears phone, email, and firebase_uid instead of writing a fake lookup.

ALTER TABLE users
    ALTER COLUMN phone_ciphertext DROP NOT NULL,
    ALTER COLUMN phone_lookup DROP NOT NULL,
    ADD COLUMN email TEXT,
    ADD COLUMN firebase_uid TEXT,
    ADD COLUMN phone_verified BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN deleted_at TIMESTAMPTZ,
    ADD CONSTRAINT users_email_len CHECK (email IS NULL OR char_length(email) <= 320),
    ADD CONSTRAINT users_firebase_uid_len CHECK (firebase_uid IS NULL OR char_length(firebase_uid) <= 128);

ALTER TABLE users DROP CONSTRAINT users_phone_lookup_unique;

CREATE UNIQUE INDEX users_phone_lookup_unique ON users (phone_lookup) WHERE phone_lookup IS NOT NULL;
CREATE UNIQUE INDEX users_firebase_uid_unique ON users (firebase_uid) WHERE firebase_uid IS NOT NULL;

UPDATE users SET phone_verified = true WHERE phone_lookup IS NOT NULL;
