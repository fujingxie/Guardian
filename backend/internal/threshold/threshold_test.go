package threshold

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/guardian/backend/internal/store"
)

func TestThresholdStateFlow(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("Skipping threshold integration test: DATABASE_URL and REDIS_URL not set")
	}

	ctx := context.Background()

	// Connect to Redis
	rOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("failed to parse redis url: %v", err)
	}
	rClient := redis.NewClient(rOpts)
	defer rClient.Close()

	// Connect to Postgres
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	// Clean up database tables we touch or ensure they exist
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS security_events (
			id TEXT PRIMARY KEY,
			server_id TEXT NOT NULL,
			type TEXT NOT NULL,
			source_ip TEXT,
			country TEXT,
			detail JSONB NOT NULL,
			plain_explanation TEXT NOT NULL,
			severity TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS notification_settings (
			id INT PRIMARY KEY,
			channels JSONB NOT NULL,
			enabled BOOLEAN NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to prepare database tables: %v", err)
	}

	// Clean up old settings
	_, _ = pool.Exec(ctx, "DELETE FROM notification_settings WHERE id = 1")
	_, _ = pool.Exec(ctx, "DELETE FROM security_events WHERE server_id = $1", "test-server-threshold")

	alertsStore := store.NewAlerts(pool)
	checker := NewChecker(rClient, alertsStore, nil)

	serverID := "test-server-threshold"
	serverName := "Test Server Threshold"

	// Ensure Redis keys are clean
	rClient.Del(ctx, "threshold:" + serverID + ":cpu")
	rClient.Del(ctx, "threshold:fired:" + serverID + ":cpu")
	rClient.Del(ctx, "threshold:" + serverID + ":disk")
	rClient.Del(ctx, "threshold:fired:" + serverID + ":disk")

	// 1. Check disk immediate alarm
	// Under threshold
	checker.Check(ctx, serverID, serverName, store.MetricPoint{
		DiskTotal: 100,
		DiskUsed:  50, // 50%
	})

	firedDisk, _ := rClient.Exists(ctx, "threshold:fired:" + serverID + ":disk").Result()
	if firedDisk > 0 {
		t.Error("expected disk alarm not to fire at 50%")
	}

	// Over threshold (triggers immediately since duration is 0)
	checker.Check(ctx, serverID, serverName, store.MetricPoint{
		DiskTotal: 100,
		DiskUsed:  96, // 96%
	})

	firedDisk, _ = rClient.Exists(ctx, "threshold:fired:" + serverID + ":disk").Result()
	if firedDisk == 0 {
		t.Error("expected disk alarm to fire at 96%")
	}

	// Verify alert is recorded in database
	events, err := alertsStore.ListAlerts(ctx, serverID, 10, 0)
	if err != nil || len(events) == 0 {
		t.Fatalf("expected alert in database, got error/empty: %v", err)
	}
	if events[0].Type != "disk_usage_high" {
		t.Errorf("expected alert type disk_usage_high, got %s", events[0].Type)
	}

	// Reset disk below threshold -> should clear fired key
	checker.Check(ctx, serverID, serverName, store.MetricPoint{
		DiskTotal: 100,
		DiskUsed:  90,
	})
	firedDisk, _ = rClient.Exists(ctx, "threshold:fired:" + serverID + ":disk").Result()
	if firedDisk > 0 {
		t.Error("expected disk alarm fired key to be cleared after disk usage returned to normal")
	}

	// 2. Check CPU duration alarm (5 minutes duration)
	// Under threshold
	checker.Check(ctx, serverID, serverName, store.MetricPoint{
		CPUPct: 40.0,
	})
	firstSeenCPU, _ := rClient.Exists(ctx, "threshold:" + serverID + ":cpu").Result()
	if firstSeenCPU > 0 {
		t.Error("expected CPU first seen key not to exist at 40%")
	}

	// Over threshold
	checker.Check(ctx, serverID, serverName, store.MetricPoint{
		CPUPct: 90.0,
	})
	firstSeenCPU, _ = rClient.Exists(ctx, "threshold:" + serverID + ":cpu").Result()
	if firstSeenCPU == 0 {
		t.Error("expected CPU first seen key to be set at 90%")
	}

	firedCPU, _ := rClient.Exists(ctx, "threshold:fired:" + serverID + ":cpu").Result()
	if firedCPU > 0 {
		t.Error("expected CPU alarm not to fire immediately (needs 5 mins duration)")
	}

	// Simulate time passing: modify CPU first seen key to be 6 minutes ago
	sixMinsAgo := time.Now().Add(-6 * time.Minute).Unix()
	rClient.Set(ctx, "threshold:" + serverID + ":cpu", strconv.FormatInt(sixMinsAgo, 10), 1*time.Hour)

	// Check again -> should fire now
	checker.Check(ctx, serverID, serverName, store.MetricPoint{
		CPUPct: 90.0,
	})
	firedCPU, _ = rClient.Exists(ctx, "threshold:fired:" + serverID + ":cpu").Result()
	if firedCPU == 0 {
		t.Error("expected CPU alarm to fire after duration has passed")
	}

	// CPU returns below threshold -> should clear keys
	checker.Check(ctx, serverID, serverName, store.MetricPoint{
		CPUPct: 50.0,
	})
	firstSeenCPU, _ = rClient.Exists(ctx, "threshold:" + serverID + ":cpu").Result()
	firedCPU, _ = rClient.Exists(ctx, "threshold:fired:" + serverID + ":cpu").Result()
	if firstSeenCPU > 0 || firedCPU > 0 {
		t.Error("expected CPU keys to be cleared after usage returned below threshold")
	}
}

func TestThresholdLoadConfigDefaults(t *testing.T) {
	// If store is nil or get settings fails, loadConfig should return defaults
	checker := NewChecker(nil, nil, nil)
	cfg := checker.loadConfig(context.Background())
	if cfg.CPUPctThreshold != 85.0 || cfg.CPUDurationMin != 5 || cfg.MemPctThreshold != 95.0 || cfg.MemDurationMin != 3 || cfg.DiskPctThreshold != 95.0 {
		t.Errorf("expected default config, got %+v", cfg)
	}
}
