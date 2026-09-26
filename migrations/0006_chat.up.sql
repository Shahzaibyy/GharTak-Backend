CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    sender_role TEXT NOT NULL,
    sender_id UUID NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chat_messages_role_check CHECK (sender_role IN ('customer', 'rider', 'merchant')),
    CONSTRAINT chat_messages_body_len CHECK (char_length(body) BETWEEN 1 AND 1000)
);

CREATE INDEX idx_chat_messages_order ON chat_messages (order_id, created_at DESC);
