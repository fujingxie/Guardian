package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGetInventoryAPI(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping inventory API test: DATABASE_URL not set")
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
		t.Fatalf("failed to create table: %v", err)
	}

	s := store.NewInventoryStore(pool)
	serverID := "api-test-server"
	ts := time.Now().UTC().Truncate(time.Second)

	// 先写入一条
	ports := []map[string]any{
		{"port": 22, "proto": "tcp", "addr": "0.0.0.0", "pid": 12, "process": "sshd"},
	}
	services := []map[string]any{
		{"name": "nginx.service", "description": "nginx description", "active": "running"},
	}
	packages := []map[string]any{
		{"name": "nginx", "version": "1.24"},
	}

	err = s.Upsert(ctx, serverID, ts, ports, services, packages)
	if err != nil {
		t.Fatalf("prepare mock data failed: %v", err)
	}

	// 初始化 Gin 路由
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &InventoryHandler{Store: s}
	r.GET("/api/servers/:id/inventory", handler.GetInventory)

	// 1. 发起真实数据请求
	req, _ := http.NewRequest(http.MethodGet, "/api/servers/api-test-server/inventory", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response body error: %v", err)
	}

	if _, ok := body["ts"]; !ok {
		t.Errorf("expected 'ts' in response body")
	}

	// 2. 发起无数据请求
	reqNotFound, _ := http.NewRequest(http.MethodGet, "/api/servers/non-existent-server/inventory", nil)
	respNotFound := httptest.NewRecorder()
	r.ServeHTTP(respNotFound, reqNotFound)

	if respNotFound.Code != http.StatusOK {
		t.Errorf("expected status 200 for not found server, got %d", respNotFound.Code)
	}

	var bodyNotFound map[string]any
	_ = json.Unmarshal(respNotFound.Body.Bytes(), &bodyNotFound)
	if portsList, ok := bodyNotFound["ports"].([]any); !ok || len(portsList) != 0 {
		t.Errorf("expected empty ports array, got %v", bodyNotFound["ports"])
	}
}
