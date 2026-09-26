-- Ratings already exist. Tickets and device tokens are the support-step tables.
CREATE TABLE support_tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT support_tickets_status_check CHECK (status IN ('open', 'closed'))
);

CREATE TABLE ticket_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES support_tickets (id) ON DELETE CASCADE,
    sender_role TEXT NOT NULL,
    sender_id UUID NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ticket_messages_role_check CHECK (sender_role IN ('customer', 'rider', 'merchant', 'admin')),
    CONSTRAINT ticket_messages_body_len CHECK (char_length(body) BETWEEN 1 AND 2000)
);

CREATE TABLE device_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    role TEXT NOT NULL,
    token VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT device_tokens_role_check CHECK (role IN ('customer', 'rider', 'merchant', 'admin')),
    CONSTRAINT device_tokens_unique UNIQUE (account_id, role, token)
);

CREATE INDEX idx_support_tickets_customer ON support_tickets (customer_id, created_at DESC);
CREATE INDEX idx_support_tickets_order ON support_tickets (order_id);
CREATE INDEX idx_ticket_messages_ticket ON ticket_messages (ticket_id, created_at);
CREATE INDEX idx_device_tokens_account ON device_tokens (account_id);
