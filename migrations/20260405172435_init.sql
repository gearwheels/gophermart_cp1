-- +goose Up
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    login VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    current_balance DOUBLE PRECISION NOT NULL DEFAULT 0,
    withdrawn DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_login ON users(login);

CREATE TABLE orders (
    number TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL,
    accrual DOUBLE PRECISION,
    accrual_applied BOOLEAN NOT NULL DEFAULT FALSE,
    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_uploaded_at ON orders(uploaded_at);

CREATE TABLE withdrawals (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_number TEXT NOT NULL,
    sum DOUBLE PRECISION NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdrawals_user_id ON withdrawals(user_id);
CREATE INDEX idx_withdrawals_processed_at ON withdrawals(processed_at);

-- +goose Down
DROP INDEX IF EXISTS idx_withdrawals_processed_at;
DROP INDEX IF EXISTS idx_withdrawals_user_id;
DROP TABLE IF EXISTS withdrawals;

DROP INDEX IF EXISTS idx_orders_uploaded_at;
DROP INDEX IF EXISTS idx_orders_user_id;
DROP TABLE IF EXISTS orders;

DROP INDEX IF EXISTS idx_users_login;
DROP TABLE IF EXISTS users;




