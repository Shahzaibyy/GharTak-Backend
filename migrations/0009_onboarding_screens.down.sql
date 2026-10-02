ALTER TABLE riders
    DROP CONSTRAINT IF EXISTS riders_orientation_slot_len,
    DROP CONSTRAINT IF EXISTS riders_license_number_len,
    DROP CONSTRAINT IF EXISTS riders_onboarding_step_check,
    DROP CONSTRAINT IF EXISTS riders_orientation_status_check,
    DROP CONSTRAINT IF EXISTS riders_vehicle_type_check,
    DROP COLUMN IF EXISTS onboarding_step,
    DROP COLUMN IF EXISTS application_received_at,
    DROP COLUMN IF EXISTS orientation_preferred_slot,
    DROP COLUMN IF EXISTS orientation_status,
    DROP COLUMN IF EXISTS cnic_back_object_key,
    DROP COLUMN IF EXISTS cnic_front_object_key,
    DROP COLUMN IF EXISTS license_number,
    DROP COLUMN IF EXISTS vehicle_type;

DROP INDEX IF EXISTS users_email_unique;

ALTER TABLE users
    DROP COLUMN IF EXISTS preferred_order_types;
