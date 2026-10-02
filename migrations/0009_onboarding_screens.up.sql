-- Customer preferences + rider multi-step onboarding fields for HTML screens.

ALTER TABLE users
    ADD COLUMN preferred_order_types TEXT[];

CREATE UNIQUE INDEX users_email_unique
    ON users (lower(email))
    WHERE email IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE riders
    ADD COLUMN vehicle_type TEXT,
    ADD COLUMN license_number VARCHAR(50),
    ADD COLUMN cnic_front_object_key TEXT,
    ADD COLUMN cnic_back_object_key TEXT,
    ADD COLUMN orientation_status TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN orientation_preferred_slot TEXT,
    ADD COLUMN application_received_at TIMESTAMPTZ,
    ADD COLUMN onboarding_step TEXT NOT NULL DEFAULT 'applied',
    ADD CONSTRAINT riders_vehicle_type_check
        CHECK (vehicle_type IS NULL OR vehicle_type IN ('motorcycle', 'bicycle')),
    ADD CONSTRAINT riders_orientation_status_check
        CHECK (orientation_status IN ('none', 'booked', 'completed')),
    ADD CONSTRAINT riders_onboarding_step_check
        CHECK (onboarding_step IN ('applied', 'details', 'documents', 'submitted')),
    ADD CONSTRAINT riders_license_number_len
        CHECK (license_number IS NULL OR char_length(license_number) <= 50),
    ADD CONSTRAINT riders_orientation_slot_len
        CHECK (orientation_preferred_slot IS NULL OR char_length(orientation_preferred_slot) <= 120);

UPDATE riders
SET cnic_front_object_key = cnic_object_key
WHERE cnic_object_key IS NOT NULL AND cnic_front_object_key IS NULL;

UPDATE riders
SET onboarding_step = 'submitted',
    application_received_at = COALESCE(application_received_at, created_at)
WHERE cnic_object_key IS NOT NULL
   OR selfie_object_key IS NOT NULL
   OR license_object_key IS NOT NULL;
