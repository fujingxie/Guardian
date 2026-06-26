package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InventorySnapshot struct {
	ServerID  string    `json:"serverId"`
	TS        time.Time `json:"ts"`
	Ports     any       `json:"ports"`
	Services  any       `json:"services"`
	Packages  any       `json:"packages"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type InventoryStore struct {
	pool *pgxpool.Pool
}

func NewInventoryStore(pool *pgxpool.Pool) *InventoryStore {
	return &InventoryStore{pool: pool}
}

// Upsert 覆盖式写入该服务器当前的系统画像
func (s *InventoryStore) Upsert(ctx context.Context, serverID string, ts time.Time, ports, services, packages any) error {
	portsJSON, err := json.Marshal(ports)
	if err != nil {
		return err
	}
	servicesJSON, err := json.Marshal(services)
	if err != nil {
		return err
	}
	packagesJSON, err := json.Marshal(packages)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO inventory_snapshots (server_id, ts, ports, services, packages, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (server_id) DO UPDATE
		SET ts = EXCLUDED.ts,
		    ports = EXCLUDED.ports,
		    services = EXCLUDED.services,
		    packages = EXCLUDED.packages,
		    updated_at = NOW()
	`, serverID, ts, portsJSON, servicesJSON, packagesJSON)
	return err
}

// Get 查询指定服务器的最新画像
func (s *InventoryStore) Get(ctx context.Context, serverID string) (*InventorySnapshot, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT server_id, ts, ports, services, packages, updated_at
		FROM inventory_snapshots
		WHERE server_id = $1
	`, serverID)

	var snap InventorySnapshot
	var portsJSON, servicesJSON, packagesJSON []byte
	err := row.Scan(&snap.ServerID, &snap.TS, &portsJSON, &servicesJSON, &packagesJSON, &snap.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(portsJSON, &snap.Ports); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(servicesJSON, &snap.Services); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(packagesJSON, &snap.Packages); err != nil {
		return nil, err
	}

	return &snap, nil
}
