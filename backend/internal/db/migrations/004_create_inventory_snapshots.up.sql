CREATE TABLE inventory_snapshots (
    server_id  TEXT PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    ts         TIMESTAMPTZ NOT NULL,
    ports      JSONB NOT NULL,
    services   JSONB NOT NULL,
    packages   JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
