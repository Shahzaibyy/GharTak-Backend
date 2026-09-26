ALTER TABLE orders
    ADD COLUMN pickup_address TEXT,
    ADD COLUMN drop_address TEXT,
    ADD CONSTRAINT orders_pickup_address_len CHECK (pickup_address IS NULL OR char_length(pickup_address) <= 500),
    ADD CONSTRAINT orders_drop_address_len CHECK (drop_address IS NULL OR char_length(drop_address) <= 500);
