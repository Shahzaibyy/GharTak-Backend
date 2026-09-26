-- GharTak MVP schema. PostgreSQL 16.
-- Money is NUMERIC(14,2) PKR. Phone and CNIC are ciphertext plus a lowercase HMAC hex lookup.
-- ledger_entries is the money source of truth. wallet_balance and cash_owed are caches.
-- Apply forward only. Later changes are new numbered migrations.

CREATE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE FUNCTION forbid_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'append-only table';
END;
$$;

CREATE TABLE zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    city_name VARCHAR(50) NOT NULL,
    slug VARCHAR(50) NOT NULL,
    base_delivery_fee NUMERIC(14,2) NOT NULL,
    per_km_rate NUMERIC(14,2) NOT NULL,
    surge_multiplier NUMERIC(3,2) NOT NULL DEFAULT 1.00,
    service_radius_km NUMERIC(6,2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT zones_city_name_unique UNIQUE (city_name),
    CONSTRAINT zones_slug_unique UNIQUE (slug),
    CONSTRAINT zones_base_fee_check CHECK (base_delivery_fee >= 0 AND base_delivery_fee <= 10000),
    CONSTRAINT zones_per_km_check CHECK (per_km_rate >= 0 AND per_km_rate <= 1000),
    CONSTRAINT zones_surge_check CHECK (surge_multiplier >= 1.00 AND surge_multiplier <= 3.00),
    CONSTRAINT zones_radius_check CHECK (service_radius_km > 0 AND service_radius_km <= 50)
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_ciphertext TEXT NOT NULL,
    phone_lookup TEXT NOT NULL,
    name VARCHAR(100),
    status TEXT NOT NULL DEFAULT 'active',
    wallet_balance NUMERIC(14,2) NOT NULL DEFAULT 0,
    device_fingerprint VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_phone_lookup_unique UNIQUE (phone_lookup),
    CONSTRAINT users_phone_lookup_format CHECK (phone_lookup ~ '^[0-9a-f]{64}$'),
    CONSTRAINT users_status_check CHECK (status IN ('active', 'suspended')),
    CONSTRAINT users_wallet_non_negative CHECK (wallet_balance >= 0)
);

CREATE TABLE addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label VARCHAR(50) NOT NULL,
    lat DOUBLE PRECISION NOT NULL,
    lng DOUBLE PRECISION NOT NULL,
    address_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT addresses_text_len CHECK (char_length(address_text) <= 500)
);

CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_ciphertext TEXT NOT NULL,
    phone_lookup TEXT NOT NULL,
    name VARCHAR(100) NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT admins_phone_lookup_unique UNIQUE (phone_lookup),
    CONSTRAINT admins_phone_lookup_format CHECK (phone_lookup ~ '^[0-9a-f]{64}$'),
    CONSTRAINT admins_status_check CHECK (status IN ('active', 'suspended'))
);

CREATE TABLE riders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_ciphertext TEXT NOT NULL,
    phone_lookup TEXT NOT NULL,
    name VARCHAR(100),
    cnic_ciphertext TEXT,
    cnic_lookup TEXT,
    cnic_object_key TEXT,
    selfie_object_key TEXT,
    license_object_key TEXT,
    vehicle_reg VARCHAR(30),
    verification_status TEXT NOT NULL DEFAULT 'pending',
    zone_id UUID REFERENCES zones (id) ON DELETE RESTRICT,
    rating NUMERIC(3,2) NOT NULL DEFAULT 5.00,
    rating_count INT NOT NULL DEFAULT 0,
    is_online BOOLEAN NOT NULL DEFAULT false,
    cash_owed NUMERIC(14,2) NOT NULL DEFAULT 0,
    device_fingerprint VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT riders_phone_lookup_unique UNIQUE (phone_lookup),
    CONSTRAINT riders_phone_lookup_format CHECK (phone_lookup ~ '^[0-9a-f]{64}$'),
    CONSTRAINT riders_cnic_lookup_unique UNIQUE (cnic_lookup),
    CONSTRAINT riders_cnic_lookup_format CHECK (cnic_lookup IS NULL OR cnic_lookup ~ '^[0-9a-f]{64}$'),
    CONSTRAINT riders_verification_status_check CHECK (verification_status IN ('pending', 'approved', 'rejected', 'suspended')),
    CONSTRAINT riders_rating_check CHECK (rating >= 0 AND rating <= 5),
    CONSTRAINT riders_rating_count_check CHECK (rating_count >= 0),
    CONSTRAINT riders_cash_owed_check CHECK (cash_owed >= 0),
    CONSTRAINT riders_online_ready CHECK (
        NOT is_online OR (zone_id IS NOT NULL AND verification_status = 'approved')
    )
);

CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_ciphertext TEXT NOT NULL,
    phone_lookup TEXT NOT NULL,
    owner_name VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    category TEXT NOT NULL,
    zone_id UUID NOT NULL REFERENCES zones (id) ON DELETE RESTRICT,
    lat DOUBLE PRECISION NOT NULL,
    lng DOUBLE PRECISION NOT NULL,
    address_text TEXT NOT NULL,
    photo_key TEXT,
    commission_rate NUMERIC(5,2) NOT NULL,
    verification_status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT merchants_phone_lookup_unique UNIQUE (phone_lookup),
    CONSTRAINT merchants_phone_lookup_format CHECK (phone_lookup ~ '^[0-9a-f]{64}$'),
    CONSTRAINT merchants_category_check CHECK (category IN ('restaurant', 'mart', 'pharmacy')),
    CONSTRAINT merchants_commission_rate_check CHECK (commission_rate >= 0 AND commission_rate <= 100),
    CONSTRAINT merchants_address_len CHECK (char_length(address_text) <= 500),
    CONSTRAINT merchants_verification_status_check CHECK (verification_status IN ('pending', 'approved', 'rejected', 'suspended'))
);

CREATE TABLE catalog_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants (id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price NUMERIC(14,2) NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT true,
    photo_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT catalog_items_price_check CHECK (price >= 0),
    CONSTRAINT catalog_items_description_len CHECK (description IS NULL OR char_length(description) <= 1000)
);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type TEXT NOT NULL,
    customer_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    merchant_id UUID REFERENCES merchants (id) ON DELETE RESTRICT,
    rider_id UUID REFERENCES riders (id) ON DELETE RESTRICT,
    zone_id UUID NOT NULL REFERENCES zones (id) ON DELETE RESTRICT,
    pickup_lat DOUBLE PRECISION NOT NULL,
    pickup_lng DOUBLE PRECISION NOT NULL,
    drop_lat DOUBLE PRECISION NOT NULL,
    drop_lng DOUBLE PRECISION NOT NULL,
    status TEXT NOT NULL DEFAULT 'placed',
    description TEXT,
    effort_tier TEXT,
    distance_km NUMERIC(8,2),
    item_total NUMERIC(14,2) NOT NULL DEFAULT 0,
    delivery_fee NUMERIC(14,2) NOT NULL DEFAULT 0,
    commission_amount NUMERIC(14,2) NOT NULL DEFAULT 0,
    rider_earning NUMERIC(14,2) NOT NULL DEFAULT 0,
    surge_multiplier NUMERIC(3,2) NOT NULL DEFAULT 1.00,
    payment_method TEXT NOT NULL,
    client_request_id VARCHAR(64),
    delivery_otp_hash TEXT,
    proof_photo_key TEXT,
    cancel_reason VARCHAR(200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    CONSTRAINT orders_type_check CHECK (type IN ('food', 'mart', 'courier', 'errand')),
    CONSTRAINT orders_status_check CHECK (status IN ('placed', 'merchant_accepted', 'preparing', 'ready_for_pickup', 'rider_offered', 'accepted', 'picked_up', 'on_the_way', 'delivered', 'cancelled', 'rejected')),
    CONSTRAINT orders_effort_tier_check CHECK (effort_tier IN ('low', 'medium', 'high')),
    CONSTRAINT orders_payment_method_check CHECK (payment_method IN ('jazzcash', 'easypaisa', 'cod', 'wallet')),
    CONSTRAINT orders_merchant_by_type CHECK (
        (type IN ('food', 'mart') AND merchant_id IS NOT NULL)
        OR (type IN ('courier', 'errand') AND merchant_id IS NULL)
    ),
    CONSTRAINT orders_description_len CHECK (description IS NULL OR char_length(description) <= 2000),
    CONSTRAINT orders_distance_check CHECK (distance_km IS NULL OR distance_km >= 0),
    CONSTRAINT orders_item_total_check CHECK (item_total >= 0),
    CONSTRAINT orders_delivery_fee_check CHECK (delivery_fee >= 0),
    CONSTRAINT orders_commission_check CHECK (commission_amount >= 0),
    CONSTRAINT orders_rider_earning_check CHECK (rider_earning >= 0),
    CONSTRAINT orders_surge_check CHECK (surge_multiplier >= 1.00 AND surge_multiplier <= 3.00)
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    catalog_item_id UUID REFERENCES catalog_items (id) ON DELETE SET NULL,
    item_name VARCHAR(100) NOT NULL,
    quantity INT NOT NULL,
    price_at_order NUMERIC(14,2) NOT NULL,
    CONSTRAINT order_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT order_items_price_check CHECK (price_at_order >= 0)
);

CREATE TABLE order_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    from_status TEXT,
    to_status TEXT NOT NULL,
    actor_role TEXT NOT NULL,
    actor_id UUID,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_events_actor_role_check CHECK (actor_role IN ('customer', 'rider', 'merchant', 'admin', 'system')),
    CONSTRAINT order_events_note_len CHECK (note IS NULL OR char_length(note) <= 500)
);

CREATE TABLE order_offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    rider_id UUID NOT NULL REFERENCES riders (id) ON DELETE RESTRICT,
    offered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    response TEXT,
    responded_at TIMESTAMPTZ,
    CONSTRAINT order_offers_pair_unique UNIQUE (order_id, rider_id),
    CONSTRAINT order_offers_response_check CHECK (response IN ('accepted', 'rejected', 'expired'))
);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    amount NUMERIC(14,2) NOT NULL,
    method TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'held',
    gateway_ref VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payments_amount_check CHECK (amount > 0),
    CONSTRAINT payments_method_check CHECK (method IN ('jazzcash', 'easypaisa', 'cod', 'wallet')),
    CONSTRAINT payments_status_check CHECK (status IN ('held', 'released', 'refunded', 'failed'))
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_id UUID NOT NULL,
    order_id UUID REFERENCES orders (id) ON DELETE RESTRICT,
    payment_id UUID REFERENCES payments (id) ON DELETE RESTRICT,
    account_kind TEXT NOT NULL,
    account_id UUID NOT NULL,
    direction TEXT NOT NULL,
    amount NUMERIC(14,2) NOT NULL,
    entry_type TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ledger_entries_idempotency_unique UNIQUE (idempotency_key),
    CONSTRAINT ledger_account_kind_check CHECK (account_kind IN ('customer_wallet', 'rider_earnings', 'rider_cash_owed', 'merchant_payable', 'platform_revenue', 'escrow', 'gateway_clearing', 'settlement_cash', 'adjustment')),
    CONSTRAINT ledger_direction_check CHECK (direction IN ('debit', 'credit')),
    CONSTRAINT ledger_entry_type_check CHECK (entry_type IN ('escrow_hold', 'escrow_release', 'refund', 'cod_recognized', 'cod_settled', 'wallet_adjustment')),
    CONSTRAINT ledger_amount_check CHECK (amount > 0)
);

CREATE TABLE ratings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    rater_role TEXT NOT NULL,
    rater_id UUID NOT NULL,
    ratee_role TEXT NOT NULL,
    ratee_id UUID NOT NULL,
    score SMALLINT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ratings_pair_unique UNIQUE (order_id, rater_id, ratee_id),
    CONSTRAINT ratings_rater_role_check CHECK (rater_role IN ('customer', 'rider', 'merchant')),
    CONSTRAINT ratings_ratee_role_check CHECK (ratee_role IN ('customer', 'rider', 'merchant')),
    CONSTRAINT ratings_score_check CHECK (score BETWEEN 1 AND 5),
    CONSTRAINT ratings_comment_len CHECK (comment IS NULL OR char_length(comment) <= 1000),
    CONSTRAINT ratings_not_self CHECK (NOT (rater_role = ratee_role AND rater_id = ratee_id))
);

CREATE TABLE fraud_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    rider_id UUID NOT NULL REFERENCES riders (id) ON DELETE RESTRICT,
    reason TEXT NOT NULL,
    pair_count INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'open',
    reviewed_by UUID REFERENCES admins (id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fraud_flags_status_check CHECK (status IN ('open', 'reviewed', 'dismissed')),
    CONSTRAINT fraud_flags_pair_count_check CHECK (pair_count >= 0),
    CONSTRAINT fraud_flags_reason_len CHECK (char_length(reason) <= 500)
);

CREATE TABLE blacklist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cnic_lookup TEXT,
    phone_lookup TEXT,
    device_fingerprint VARCHAR(128),
    reason TEXT NOT NULL,
    created_by UUID REFERENCES admins (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT blacklist_cnic_lookup_format CHECK (cnic_lookup IS NULL OR cnic_lookup ~ '^[0-9a-f]{64}$'),
    CONSTRAINT blacklist_phone_lookup_format CHECK (phone_lookup IS NULL OR phone_lookup ~ '^[0-9a-f]{64}$'),
    CONSTRAINT blacklist_has_identifier CHECK (
        cnic_lookup IS NOT NULL OR device_fingerprint IS NOT NULL OR phone_lookup IS NOT NULL
    ),
    CONSTRAINT blacklist_reason_len CHECK (char_length(reason) <= 500)
);

-- Zone list orders by city_name and uses zones_city_name_unique (index scan, 6 rows).
CREATE INDEX idx_addresses_user ON addresses (user_id);

CREATE INDEX idx_riders_zone ON riders (zone_id);
CREATE INDEX idx_riders_dispatch ON riders (zone_id)
    WHERE is_online = true AND verification_status = 'approved';

CREATE INDEX idx_merchants_zone ON merchants (zone_id);
CREATE INDEX idx_merchants_browse ON merchants (zone_id, category)
    WHERE verification_status = 'approved';

CREATE INDEX idx_catalog_merchant ON catalog_items (merchant_id);
CREATE INDEX idx_catalog_available ON catalog_items (merchant_id)
    WHERE is_available = true;

CREATE INDEX idx_orders_customer_created ON orders (customer_id, created_at DESC);
CREATE INDEX idx_orders_zone_created ON orders (zone_id, created_at DESC);
CREATE INDEX idx_orders_rider ON orders (rider_id) WHERE rider_id IS NOT NULL;
CREATE INDEX idx_orders_merchant_status ON orders (merchant_id, status) WHERE merchant_id IS NOT NULL;
CREATE INDEX idx_orders_open ON orders (zone_id, status)
    WHERE status NOT IN ('delivered', 'cancelled', 'rejected');
CREATE INDEX idx_orders_active_rider ON orders (rider_id)
    WHERE rider_id IS NOT NULL AND status IN ('accepted', 'picked_up', 'on_the_way');
CREATE UNIQUE INDEX idx_orders_client_request ON orders (customer_id, client_request_id)
    WHERE client_request_id IS NOT NULL;

CREATE INDEX idx_order_items_order ON order_items (order_id);
CREATE INDEX idx_order_items_catalog ON order_items (catalog_item_id);

CREATE INDEX idx_order_events_order ON order_events (order_id, created_at);

CREATE INDEX idx_order_offers_rider ON order_offers (rider_id, offered_at DESC);
CREATE UNIQUE INDEX idx_order_offers_one_accept ON order_offers (order_id)
    WHERE response = 'accepted';

CREATE UNIQUE INDEX idx_payments_order ON payments (order_id);
CREATE UNIQUE INDEX idx_payments_gateway_ref ON payments (gateway_ref)
    WHERE gateway_ref IS NOT NULL;
CREATE INDEX idx_payments_held ON payments (created_at) WHERE status = 'held';

CREATE INDEX idx_ledger_account ON ledger_entries (account_kind, account_id, created_at);
CREATE INDEX idx_ledger_journal ON ledger_entries (journal_id);
CREATE INDEX idx_ledger_order ON ledger_entries (order_id);
CREATE INDEX idx_ledger_payment ON ledger_entries (payment_id);

CREATE INDEX idx_ratings_order ON ratings (order_id);
CREATE INDEX idx_ratings_ratee ON ratings (ratee_role, ratee_id);

CREATE INDEX idx_fraud_flags_customer ON fraud_flags (customer_id);
CREATE INDEX idx_fraud_flags_rider ON fraud_flags (rider_id);
CREATE INDEX idx_fraud_flags_open ON fraud_flags (created_at) WHERE status = 'open';
CREATE UNIQUE INDEX idx_fraud_flags_open_pair ON fraud_flags (customer_id, rider_id)
    WHERE status = 'open';
CREATE INDEX idx_fraud_flags_reviewer ON fraud_flags (reviewed_by);

CREATE INDEX idx_blacklist_cnic ON blacklist (cnic_lookup) WHERE cnic_lookup IS NOT NULL;
CREATE INDEX idx_blacklist_phone ON blacklist (phone_lookup) WHERE phone_lookup IS NOT NULL;
CREATE INDEX idx_blacklist_device ON blacklist (device_fingerprint) WHERE device_fingerprint IS NOT NULL;
CREATE INDEX idx_blacklist_admin ON blacklist (created_by);

CREATE TRIGGER trg_zones_updated_at
    BEFORE UPDATE ON zones
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_addresses_updated_at
    BEFORE UPDATE ON addresses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_admins_updated_at
    BEFORE UPDATE ON admins
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_riders_updated_at
    BEFORE UPDATE ON riders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_merchants_updated_at
    BEFORE UPDATE ON merchants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_catalog_items_updated_at
    BEFORE UPDATE ON catalog_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_fraud_flags_updated_at
    BEFORE UPDATE ON fraud_flags
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_order_events_append_only
    BEFORE UPDATE OR DELETE ON order_events
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TRIGGER trg_ledger_entries_append_only
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TRIGGER trg_ratings_append_only
    BEFORE UPDATE OR DELETE ON ratings
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TRIGGER trg_blacklist_append_only
    BEFORE UPDATE OR DELETE ON blacklist
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
