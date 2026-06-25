package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricPoint struct {
	ServerID  string
	TS        time.Time
	CPUPct    float64
	MemUsed   int64
	MemTotal  int64
	DiskUsed  int64
	DiskTotal int64
	NetRx     int64 // bytes/s
	NetTx     int64 // bytes/s
	Load1     float64
	UptimeSec int64
}

type Metrics struct{ pool *pgxpool.Pool }

func NewMetrics(pool *pgxpool.Pool) *Metrics { return &Metrics{pool: pool} }

func (m *Metrics) Insert(ctx context.Context, p MetricPoint) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO metrics
			(server_id, ts, cpu_pct, mem_used, mem_total, disk_used, disk_total,
			 net_rx, net_tx, load1, uptime_sec)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, p.ServerID, p.TS, p.CPUPct, p.MemUsed, p.MemTotal, p.DiskUsed,
		p.DiskTotal, p.NetRx, p.NetTx, p.Load1, p.UptimeSec)
	return err
}

// LastFor 返回每台机器的最新一条指标。给 GET /api/servers 用，避免 N+1。
func (m *Metrics) LastFor(ctx context.Context, ids []string) (map[string]MetricPoint, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT ON (server_id)
			server_id, ts, cpu_pct, mem_used, mem_total, disk_used, disk_total,
			net_rx, net_tx, load1, uptime_sec
		FROM metrics
		WHERE server_id = ANY($1)
		ORDER BY server_id, ts DESC
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]MetricPoint, len(ids))
	for rows.Next() {
		var p MetricPoint
		if err := rows.Scan(&p.ServerID, &p.TS, &p.CPUPct, &p.MemUsed, &p.MemTotal,
			&p.DiskUsed, &p.DiskTotal, &p.NetRx, &p.NetTx, &p.Load1, &p.UptimeSec); err != nil {
			return nil, err
		}
		out[p.ServerID] = p
	}
	return out, rows.Err()
}

func (m *Metrics) ListRange(ctx context.Context, serverID string, from time.Time) ([]MetricPoint, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT server_id, ts, cpu_pct, mem_used, mem_total, disk_used, disk_total,
		       net_rx, net_tx, load1, uptime_sec
		FROM metrics
		WHERE server_id = $1 AND ts >= $2
		ORDER BY ts ASC
	`, serverID, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MetricPoint
	for rows.Next() {
		var p MetricPoint
		if err := rows.Scan(&p.ServerID, &p.TS, &p.CPUPct, &p.MemUsed, &p.MemTotal,
			&p.DiskUsed, &p.DiskTotal, &p.NetRx, &p.NetTx, &p.Load1, &p.UptimeSec); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeleteOlderThan 删除 cutoff 之前的指标点；返回删除行数。
func (m *Metrics) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := m.pool.Exec(ctx, `DELETE FROM metrics WHERE ts < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
