CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    data BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);

-- The scs package will automatically delete expired sessions
CREATE INDEX sessions_expiry_idx ON sessions (expiry);
