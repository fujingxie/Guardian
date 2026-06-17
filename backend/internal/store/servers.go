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
