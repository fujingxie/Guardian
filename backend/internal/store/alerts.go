package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SecurityEvent struct {
	ID               string
	ServerID         string
	Type             string // 对应底层存储的 type
	SourceIP         *string
	Detail           []byte
	PlainExplanation string
	Severity         string
	Status           string
	CreatedAt        time.Time
}

type NotifySettings struct {
	Channels []byte
	Enabled  bool
}

type Alerts struct {
	pool *pgxpool.Pool
}

func NewAlerts(pool *pgxpool.Pool) *Alerts {
	return &Alerts{pool: pool}
}

func (a *Alerts) CreateAlert(ctx context.Context, serverID, eventType, sourceIP string, detail map[string]any, plainExplanation, severity string) (*SecurityEvent, error) {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	var ev SecurityEvent
	var ip *string
	if sourceIP != "" {
		ip = &sourceIP
	}
	err = a.pool.QueryRow(ctx, `
		INSERT INTO security_events (server_id, type, source_ip, detail, plain_explanation, severity, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'new')
		RETURNING id, server_id, type, source_ip, detail, plain_explanation, severity, status, created_at
	`, serverID, eventType, ip, detailJSON, plainExplanation, severity).Scan(
		&ev.ID, &ev.ServerID, &ev.Type, &ev.SourceIP, &ev.Detail, &ev.PlainExplanation, &ev.Severity, &ev.Status, &ev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (a *Alerts) ListAlerts(ctx context.Context, serverID string) ([]SecurityEvent, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT id, server_id, type, source_ip, detail, plain_explanation, severity, status, created_at
		FROM security_events
		WHERE server_id = $1
		ORDER BY created_at DESC
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SecurityEvent
	for rows.Next() {
		var ev SecurityEvent
		if err := rows.Scan(
			&ev.ID, &ev.ServerID, &ev.Type, &ev.SourceIP, &ev.Detail, &ev.PlainExplanation, &ev.Severity, &ev.Status, &ev.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, ev)
	}
	return list, rows.Err()
}

func (a *Alerts) GetNotifySettings(ctx context.Context) (*NotifySettings, error) {
	var ns NotifySettings
	err := a.pool.QueryRow(ctx, `
		SELECT channels, enabled FROM notification_settings WHERE id = 1
	`).Scan(&ns.Channels, &ns.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return &NotifySettings{
			Channels: []byte("{}"),
			Enabled:  true,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &ns, nil
}

func (a *Alerts) UpdateNotifySettings(ctx context.Context, channels []byte, enabled bool) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO notification_settings (id, channels, enabled)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO UPDATE
		SET channels = EXCLUDED.channels, enabled = EXCLUDED.enabled
	`, channels, enabled)
	return err
}
