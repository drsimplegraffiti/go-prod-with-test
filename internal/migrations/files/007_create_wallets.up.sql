CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    customer_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    currency VARCHAR(3) NOT NULL DEFAULT 'NGN',

    balance NUMERIC(20, 2) NOT NULL DEFAULT 0
        CHECK (balance >= 0),

    status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'closed')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_wallet_customer_currency
        UNIQUE (customer_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_wallets_customer_id
    ON wallets(customer_id);
