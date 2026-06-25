package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HardeningItem struct {
	Key              string
	Category         string
	Title            string
	PlainExplanation string
	RiskLevel        string
	DefaultEnabled   bool
}

type HardeningJob struct {
	ID              string
	ServerID        string
	ItemKey         string
	Status          string
	SnapshotID      *string
	ConfirmDeadline *time.Time
	ConfirmedAt     *time.Time
	Error           *string
	CreatedAt       time.Time

	// Preloaded
	SnapshotFiles []byte // JSONB data of files map
}

type Hardening struct {
	pool *pgxpool.Pool
}

func NewHardening(pool *pgxpool.Pool) *Hardening {
	return &Hardening{pool: pool}
}

// ListItems 获取所有全局加固项定义
func (h *Hardening) ListItems(ctx context.Context) ([]HardeningItem, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT key, category, title, plain_explanation, risk_level, default_enabled
		FROM hardening_items
		ORDER BY category, key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []HardeningItem
	for rows.Next() {
		var item HardeningItem
		if err := rows.Scan(
			&item.Key, &item.Category, &item.Title, &item.PlainExplanation,
			&item.RiskLevel, &item.DefaultEnabled,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// GetLatestJobs 获取某台服务器上每个加固项最新的一条任务记录
func (h *Hardening) GetLatestJobs(ctx context.Context, serverID string) (map[string]*HardeningJob, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT DISTINCT ON (item_key)
			id, server_id, item_key, status, snapshot_id, confirm_deadline, confirmed_at, error, created_at
		FROM hardening_jobs
		WHERE server_id = $1
		ORDER BY item_key, created_at DESC
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make(map[string]*HardeningJob)
	for rows.Next() {
		var j HardeningJob
		if err := rows.Scan(
			&j.ID, &j.ServerID, &j.ItemKey, &j.Status, &j.SnapshotID,
			&j.ConfirmDeadline, &j.ConfirmedAt, &j.Error, &j.CreatedAt,
		); err != nil {
			return nil, err
		}
		jobs[j.ItemKey] = &j
	}
	return jobs, rows.Err()
}

// GetLatestJobByKey 获取某台服务器上特定加固项最新的一条任务记录（包含快照内容）
func (h *Hardening) GetLatestJobByKey(ctx context.Context, serverID, itemKey string) (*HardeningJob, error) {
	row := h.pool.QueryRow(ctx, `
		SELECT j.id, j.server_id, j.item_key, j.status, j.snapshot_id, j.confirm_deadline, j.confirmed_at, j.error, j.created_at, s.files
		FROM hardening_jobs j
		LEFT JOIN config_snapshots s ON j.snapshot_id = s.id
		WHERE j.server_id = $1 AND j.item_key = $2
		ORDER BY j.created_at DESC
		LIMIT 1
	`, serverID, itemKey)

	var j HardeningJob
	err := row.Scan(
		&j.ID, &j.ServerID, &j.ItemKey, &j.Status, &j.SnapshotID,
		&j.ConfirmDeadline, &j.ConfirmedAt, &j.Error, &j.CreatedAt,
		&j.SnapshotFiles,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &j, err
}

// CreateJob 创建一个新的加固任务
func (h *Hardening) CreateJob(ctx context.Context, serverID, itemKey, status string) (string, error) {
	var id string
	err := h.pool.QueryRow(ctx, `
		INSERT INTO hardening_jobs (server_id, item_key, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`, serverID, itemKey, status).Scan(&id)
	return id, err
}

// GetJob 查询单个加固任务（包含关联快照内容）
func (h *Hardening) GetJob(ctx context.Context, jobID string) (*HardeningJob, error) {
	row := h.pool.QueryRow(ctx, `
		SELECT j.id, j.server_id, j.item_key, j.status, j.snapshot_id, j.confirm_deadline, j.confirmed_at, j.error, j.created_at, s.files
		FROM hardening_jobs j
		LEFT JOIN config_snapshots s ON j.snapshot_id = s.id
		WHERE j.id = $1
	`, jobID)

	var j HardeningJob
	err := row.Scan(
		&j.ID, &j.ServerID, &j.ItemKey, &j.Status, &j.SnapshotID,
		&j.ConfirmDeadline, &j.ConfirmedAt, &j.Error, &j.CreatedAt,
		&j.SnapshotFiles,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &j, err
}

// UpdateJobToTrial 更新任务状态为 trial 并填入倒计时与快照关联
func (h *Hardening) UpdateJobToTrial(ctx context.Context, jobID string, deadline time.Time, snapshotID string) error {
	_, err := h.pool.Exec(ctx, `
		UPDATE hardening_jobs
		SET status = 'trial', confirm_deadline = $2, snapshot_id = $3
		WHERE id = $1
	`, jobID, deadline, snapshotID)
	return err
}

// UpdateJobStatus 更新任务状态
func (h *Hardening) UpdateJobStatus(ctx context.Context, jobID, status string, errStr *string) error {
	if status == "applied" {
		_, err := h.pool.Exec(ctx, `
			UPDATE hardening_jobs
			SET status = $2, confirmed_at = NOW()
			WHERE id = $1
		`, jobID, status)
		return err
	}
	if status == "failed" && errStr != nil {
		_, err := h.pool.Exec(ctx, `
			UPDATE hardening_jobs
			SET status = $2, error = $3
			WHERE id = $1
		`, jobID, status, *errStr)
		return err
	}
	_, err := h.pool.Exec(ctx, `
		UPDATE hardening_jobs
		SET status = $2
		WHERE id = $1
	`, jobID, status)
	return err
}

// SaveSnapshot 写入备份快照记录并返回其 UUID 字符串
func (h *Hardening) SaveSnapshot(ctx context.Context, serverID, jobID string, files []byte) (string, error) {
	var id string
	err := h.pool.QueryRow(ctx, `
		INSERT INTO config_snapshots (server_id, job_id, files)
		VALUES ($1, $2, $3)
		RETURNING id
	`, serverID, jobID, files).Scan(&id)
	return id, err
}

// GetTimeoutTrialJobs 扫描获得已经超时的 trial jobs（伴随 snapshot 加载）
func (h *Hardening) GetTimeoutTrialJobs(ctx context.Context) ([]*HardeningJob, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT j.id, j.server_id, j.item_key, j.status, j.snapshot_id, j.confirm_deadline, j.confirmed_at, j.error, j.created_at, s.files
		FROM hardening_jobs j
		LEFT JOIN config_snapshots s ON j.snapshot_id = s.id
		WHERE j.status = 'trial' AND j.confirm_deadline <= NOW()
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*HardeningJob
	for rows.Next() {
		var j HardeningJob
		if err := rows.Scan(
			&j.ID, &j.ServerID, &j.ItemKey, &j.Status, &j.SnapshotID,
			&j.ConfirmDeadline, &j.ConfirmedAt, &j.Error, &j.CreatedAt,
			&j.SnapshotFiles,
		); err != nil {
			return nil, err
		}
		list = append(list, &j)
	}
	return list, rows.Err()
}
