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
	Country          *string
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

func (a *Alerts) CreateAlert(ctx context.Context, serverID, eventType, sourceIP, country string, detail map[string]any, plainExplanation, severity string) (*SecurityEvent, error) {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	var ev SecurityEvent
	var ip *string
	if sourceIP != "" {
		ip = &sourceIP
	}
	var countryVal *string
	if country != "" {
		countryVal = &country
	}
	err = a.pool.QueryRow(ctx, `
		INSERT INTO security_events (server_id, type, source_ip, country, detail, plain_explanation, severity, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'new')
		RETURNING id, server_id, type, source_ip, country, detail, plain_explanation, severity, status, created_at
	`, serverID, eventType, ip, countryVal, detailJSON, plainExplanation, severity).Scan(
		&ev.ID, &ev.ServerID, &ev.Type, &ev.SourceIP, &ev.Country, &ev.Detail, &ev.PlainExplanation, &ev.Severity, &ev.Status, &ev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (a *Alerts) ListAlerts(ctx context.Context, serverID string, limit, offset int) ([]SecurityEvent, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT id, server_id, type, source_ip, country, detail, plain_explanation, severity, status, created_at
		FROM security_events
		WHERE server_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, serverID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SecurityEvent
	for rows.Next() {
		var ev SecurityEvent
		if err := rows.Scan(
			&ev.ID, &ev.ServerID, &ev.Type, &ev.SourceIP, &ev.Country, &ev.Detail, &ev.PlainExplanation, &ev.Severity, &ev.Status, &ev.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, ev)
	}
	return list, rows.Err()
}

type TimelinePoint struct {
	Date   string `json:"date"`
	High   int    `json:"high"`
	Medium int    `json:"medium"`
	Info   int    `json:"info"`
}

func (a *Alerts) GetAlertsTimeline(ctx context.Context, serverID string) ([]TimelinePoint, error) {
	// 查询过去 7 天每天的告警严重程度分布 (按 UTC 时区划分日期)
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	rows, err := a.pool.Query(ctx, `
		SELECT (created_at AT TIME ZONE 'UTC')::date AS date, severity, COUNT(*) AS count
		FROM security_events
		WHERE server_id = $1 AND created_at >= $2
		GROUP BY date, severity
		ORDER BY date ASC
	`, serverID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 内存中做按天和严重程度聚合
	counts := make(map[string]map[string]int)
	for rows.Next() {
		var dt time.Time
		var sev string
		var count int
		if err := rows.Scan(&dt, &sev, &count); err != nil {
			return nil, err
		}
		dateStr := dt.Format("2006-01-02")
		if _, ok := counts[dateStr]; !ok {
			counts[dateStr] = make(map[string]int)
		}
		counts[dateStr][sev] = count
	}

	// 补全过去 7 天的数据（包含今天）
	var timeline []TimelinePoint
	for i := 6; i >= 0; i-- {
		t := time.Now().UTC().AddDate(0, 0, -i)
		dateStr := t.Format("2006-01-02")
		
		high := 0
		medium := 0
		info := 0
		if dayMap, ok := counts[dateStr]; ok {
			high = dayMap["high"]
			medium = dayMap["medium"]
			info = dayMap["info"]
		}

		timeline = append(timeline, TimelinePoint{
			Date:   dateStr,
			High:   high,
			Medium: medium,
			Info:   info,
		})
	}

	return timeline, nil
}

type AttackerIP struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	Count   int    `json:"count"`
}

type CountryStats struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}

type AlertStats struct {
	TopIPs    []AttackerIP   `json:"topIPs"`
	Countries []CountryStats `json:"countries"`
}

func (a *Alerts) GetAlertsStats(ctx context.Context, serverID string) (*AlertStats, error) {
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)

	// 1. Top 10 IP
	ipRows, err := a.pool.Query(ctx, `
		SELECT source_ip, COALESCE(country, '未知') AS country, COUNT(*) AS count
		FROM security_events
		WHERE server_id = $1 AND source_ip IS NOT NULL AND source_ip <> '' AND created_at >= $2
		GROUP BY source_ip, country
		ORDER BY count DESC
		LIMIT 10
	`, serverID, cutoff)
	if err != nil {
		return nil, err
	}
	defer ipRows.Close()

	var topIPs []AttackerIP
	for ipRows.Next() {
		var item AttackerIP
		if err := ipRows.Scan(&item.IP, &item.Country, &item.Count); err != nil {
			return nil, err
		}
		topIPs = append(topIPs, item)
	}

	// 2. 国家占比分布
	cRows, err := a.pool.Query(ctx, `
		SELECT COALESCE(country, '未知') AS country, COUNT(*) AS count
		FROM security_events
		WHERE server_id = $1 AND source_ip IS NOT NULL AND source_ip <> '' AND created_at >= $2
		GROUP BY country
		ORDER BY count DESC
	`, serverID, cutoff)
	if err != nil {
		return nil, err
	}
	defer cRows.Close()

	var countries []CountryStats
	for cRows.Next() {
		var item CountryStats
		if err := cRows.Scan(&item.Country, &item.Count); err != nil {
			return nil, err
		}
		countries = append(countries, item)
	}

	return &AlertStats{
		TopIPs:    topIPs,
		Countries: countries,
	}, nil
}

// GetTodayBlockedCounts 获取所有服务器今天（UTC）拦截的安全事件数量
func (a *Alerts) GetTodayBlockedCounts(ctx context.Context) (map[string]int, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	rows, err := a.pool.Query(ctx, `
		SELECT server_id, COUNT(*) 
		FROM security_events 
		WHERE created_at >= $1
		GROUP BY server_id
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var sID string
		var count int
		if err := rows.Scan(&sID, &count); err != nil {
			return nil, err
		}
		counts[sID] = count
	}
	return counts, rows.Err()
}

// GetTodayBlockedCountForServer 获取特定服务器今天（UTC）拦截的安全事件数量
func (a *Alerts) GetTodayBlockedCountForServer(ctx context.Context, serverID string) (int, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var count int
	err := a.pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM security_events 
		WHERE server_id = $1 AND created_at >= $2
	`, serverID, today).Scan(&count)
	return count, err
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
