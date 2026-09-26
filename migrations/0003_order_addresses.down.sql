ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_drop_address_len,
    DROP CONSTRAINT IF EXISTS orders_pickup_address_len,
    DROP COLUMN IF EXISTS drop_address,
    DROP COLUMN IF EXISTS pickup_address;
