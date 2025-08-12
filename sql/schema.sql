CREATE TABLE payments (
    id SERIAL PRIMARY KEY,
    correlation_id UUID NOT NULL UNIQUE,
    amount NUMERIC(10, 2) NOT NULL,
    processor VARCHAR(10) NOT NULL, 
    requested_at TIMESTAMPTZ NOT NULL
);