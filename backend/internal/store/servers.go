package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	ID              string
	Name            string
	Hostname        *string
	OS              *string
	Distro          *string
	Arch            *string
	AgentTokenHash  *string
	Status          string // online | offline
	LastSeenAt      *time.Time
	CurrentAdminIP  *string
	AgentVersion    *string
	CreatedAt       time.Time
}

var ErrNotFound = errors.New("not found")

type Servers struct{ pool *pgxpool.Pool }

func NewServers(pool *pgxpool.Pool) *Servers { return &Servers{pool: pool} }

// Insert 创建一台服务器；id/name 必填，其余字段后续 enroll 时回填。
func (s *Servers) Insert(ctx context.Context, id, name, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO servers (id, name, agent_token_hash, status)
		VALUES ($1, $2, $3, 'offline')
	`, id, name, tokenHash)
	return err
}

func (s *Servers) Get(ctx context.Context, id string) (*Server, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, hostname, os, distro, arch, agent_token_hash,
		       status, last_seen_at, current_admin_ip, agent_version, created_at
		FROM servers WHERE id = $1
	`, id)
	var sv Server
	err := row.Scan(
		&sv.ID, &sv.Name, &sv.Hostname, &sv.OS, &sv.Distro, &sv.Arch,
		&sv.AgentTokenHash, &sv.Status, &sv.LastSeenAt, &sv.CurrentAdminIP,
		&sv.AgentVersion, &sv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sv, nil
}

// LookupByTokenHash 用 token 哈希反查一台服务器。enroll / WSS 鉴权都走它。
func (s *Servers) LookupByTokenHash(ctx context.Context, hash string) (*Server, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, hostname, os, distro, arch, agent_token_hash,
		       status, last_seen_at, current_admin_ip, agent_version, created_at
		FROM servers WHERE agent_token_hash = $1
	`, hash)
	var sv Server
	err := row.Scan(
		&sv.ID, &sv.Name, &sv.Hostname, &sv.OS, &sv.Distro, &sv.Arch,
		&sv.AgentTokenHash, &sv.Status, &sv.LastSeenAt, &sv.CurrentAdminIP,
		&sv.AgentVersion, &sv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sv, nil
}

// RotateToken 用新 agent_token 哈希替换旧 enrollment_token 哈希，同步回填 enroll 信息。
func (s *Servers) RotateToken(ctx context.Context, id, newHash, hostname, osName, arch, agentVersion string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE servers
		SET agent_token_hash = $2,
		    hostname = $3,
		    os = $4,
		    arch = $5,
		    agent_version = $6
		WHERE id = $1
	`, id, newHash, hostname, osName, arch, agentVersion)
	return err
}

// UpdateDistro 用 agent 报上来的 distro 字符串覆盖（如果非空且变化）。
func (s *Servers) UpdateDistro(ctx context.Context, id, distro string) error {
	if distro == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `UPDATE servers SET distro = $2 WHERE id = $1 AND distro IS DISTINCT FROM $2`, id, distro)
	return err
}

// UpdateAdminIP 更新管理端的连接 IP。
func (s *Servers) UpdateAdminIP(ctx context.Context, id, ip string) error {
	if ip == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `UPDATE servers SET current_admin_ip = $2 WHERE id = $1 AND current_admin_ip IS DISTINCT FROM $2`, id, ip)
	return err
}

// SetStatus 写入在/离线 + last_seen_at。
func (s *Servers) SetStatus(ctx context.Context, id, status string, lastSeen time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE servers SET status = $2, last_seen_at = $3 WHERE id = $1
	`, id, status, lastSeen)
	return err
}

// SetOfflineIfMissing 对每个 online 但 Redis 里没有键的服务器置 offline。
func (s *Servers) SetOfflineByIDs(ctx context.Context, ids []string, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE servers SET status = 'offline', last_seen_at = $2
		WHERE id = ANY($1) AND status = 'online'
	`, ids, now)
	return err
}

func (s *Servers) CountOnline(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM servers WHERE status = 'online'`).Scan(&count)
	return count, err
}

func (s *Servers) ListOnlineIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM servers WHERE status = 'online'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Servers) List(ctx context.Context) ([]Server, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, hostname, os, distro, arch, agent_token_hash,
		       status, last_seen_at, current_admin_ip, agent_version, created_at
		FROM servers
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		var sv Server
		if err := rows.Scan(
			&sv.ID, &sv.Name, &sv.Hostname, &sv.OS, &sv.Distro, &sv.Arch,
			&sv.AgentTokenHash, &sv.Status, &sv.LastSeenAt, &sv.CurrentAdminIP,
			&sv.AgentVersion, &sv.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

// UpdateName 修改服务器名称。
func (s *Servers) UpdateName(ctx context.Context, id, name string) error {
	_, err := s.pool.Exec(ctx, `UPDATE servers SET name = $2 WHERE id = $1`, id, name)
	return err
}

// Delete 删除指定服务器。外键级联删除 (ON DELETE CASCADE) 会自动清理相关联的表记录。
func (s *Servers) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM servers WHERE id = $1`, id)
	return err
}
