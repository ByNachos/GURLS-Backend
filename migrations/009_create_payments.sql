-- 009_create_payments.sql
-- Create payments table

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_id VARCHAR(255) UNIQUE NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL,
    subscription_type_id SMALLINT REFERENCES subscription_types(id),
    yookassa_payment_id VARCHAR(255),
    yookassa_payment_data TEXT,
    failure_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for payments table
CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_payment_id ON payments(payment_id);
CREATE INDEX idx_payments_yookassa_payment_id ON payments(yookassa_payment_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);

-- Create subscription_changes table
CREATE TABLE IF NOT EXISTS subscription_changes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    old_subscription_id SMALLINT REFERENCES subscription_types(id),
    new_subscription_id SMALLINT NOT NULL REFERENCES subscription_types(id),
    payment_id BIGINT REFERENCES payments(id),
    change_type VARCHAR(50) NOT NULL,
    effective_date TIMESTAMP WITH TIME ZONE NOT NULL,
    expiration_date TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for subscription_changes table
CREATE INDEX idx_subscription_changes_user_id ON subscription_changes(user_id);
CREATE INDEX idx_subscription_changes_effective_date ON subscription_changes(effective_date);
CREATE INDEX idx_subscription_changes_expiration_date ON subscription_changes(expiration_date);
CREATE INDEX idx_subscription_changes_is_active ON subscription_changes(is_active);
CREATE INDEX idx_subscription_changes_change_type ON subscription_changes(change_type);