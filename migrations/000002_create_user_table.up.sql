CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS user_ (
    id_ uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    email_ CITEXT UNIQUE NOT NULL,
    password_hash_ BYTEA NOT NULL,
    created_at_ TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
