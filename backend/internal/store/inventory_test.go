package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInventoryStoreUpsertAndGet(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping inventory DB store integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// 保证测试开始前有对应的表
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS inventory_snapshots (
			server_id  TEXT PRIMARY KEY,
			ts         TIMESTAMPTZ NOT NULL,
			ports      JSONB NOT NULL,
			services   JSONB NOT NULL,
			packages   JSONB NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		t.Fatalf("failed to create temp table: %v", err)
	}

	s := NewInventoryStore(pool)
	serverID := "test-server-123"
	ts := time.Now().UTC().Truncate(time.Second)

	ports := []map[string]any{
		{"port": 22, "proto": "tcp", "addr": "0.0.0.0", "pid": 12, "process": "sshd"},
	}
	services := []map[string]any{
		{"name": "nginx.service", "description": "nginx description", "active": "running"},
	}
	packages := []map[string]any{
		{"name": "nginx", "version": "1.24"},
	}

	// 1. 测试 Upsert
	err = s.Upsert(ctx, serverID, ts, ports, services, packages)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// 2. 测试 Get
	snap, err := s.Get(ctx, serverID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if snap.ServerID != serverID {
		t.Errorf("expected ServerID %s, got %s", serverID, snap.ServerID)
	}
	if !snap.TS.Equal(ts) {
		t.Errorf("expected TS %v, got %v", ts, snap.TS)
	}

	// 3. 测试覆盖更新
	ts2 := ts.Add(time.Minute)
	err = s.Upsert(ctx, serverID, ts2, ports, services, packages)
	if err != nil {
		t.Fatalf("Upsert 2 failed: %v", err)
	}

	snap2, err := s.Get(ctx, serverID)
	if err != nil {
		t.Fatalf("Get 2 failed: %v", err)
	}
	if !snap2.TS.Equal(ts2) {
		t.Errorf("expected TS to be updated to %v, got %v", ts2, snap2.TS)
	}
}
