// Package agenthub 维护所有 agent 的 WSS 连接登记、在线状态（Redis）、命令路由。
//
// 设计：
//   - 在线状态：Redis 键 agent:<id>:online，TTL 30s，每次心跳/消息刷新。
//   - 进程内 conns 表用于命令路由（CommandTo 把消息写给目标 agent）。
//   - 后台 sweeper 周期检查：DB 里 status=online 但 Redis 键失效的，置 offline。
package agenthub

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/guardian/backend/internal/store"
	"github.com/redis/go-redis/v9"
)

const (
	OnlineTTL      = 30 * time.Second
	SweeperPeriod  = 10 * time.Second
	OnlineKeyPref  = "agent:"
	OnlineKeySuf   = ":online"
)

func onlineKey(id string) string { return OnlineKeyPref + id + OnlineKeySuf }

// Conn 是单条 agent 连接的抽象 —— 用接口避免循环引用 websocket 包。
type Conn interface {
	WriteJSON(v any) error
	Close() error
}

type entry struct {
	conn Conn
	id   string
}

type Hub struct {
	rds     *redis.Client
	servers *store.Servers

	mu    sync.RWMutex
	conns map[string]*entry // serverID -> entry
}

func New(rds *redis.Client, servers *store.Servers) *Hub {
	return &Hub{
		rds:     rds,
		servers: servers,
		conns:   make(map[string]*entry),
	}
}

// Register 当 agent 建立 WSS 连接时调用：登记 conn、置 online。
// 如果同一台服务器已有旧连接，旧连接会被关闭（避免假在线）。
func (h *Hub) Register(ctx context.Context, serverID string, c Conn) error {
	h.mu.Lock()
	if old, ok := h.conns[serverID]; ok {
		_ = old.conn.Close()
	}
	h.conns[serverID] = &entry{conn: c, id: serverID}
	h.mu.Unlock()

	if err := h.rds.Set(ctx, onlineKey(serverID), "1", OnlineTTL).Err(); err != nil {
		return err
	}
	return h.servers.SetStatus(ctx, serverID, "online", time.Now().UTC())
}

// Unregister 当连接断开时调用：摘掉 conn、置 offline。
func (h *Hub) Unregister(ctx context.Context, serverID string) {
	h.mu.Lock()
	delete(h.conns, serverID)
	h.mu.Unlock()
	_ = h.rds.Del(ctx, onlineKey(serverID)).Err()
	if err := h.servers.SetStatus(ctx, serverID, "offline", time.Now().UTC()); err != nil {
		log.Printf("[hub] mark offline %s: %v", serverID, err)
	}
}

// Touch 刷新在线 TTL（心跳/任意 agent 消息触发）。
func (h *Hub) Touch(ctx context.Context, serverID string) {
	_ = h.rds.Expire(ctx, onlineKey(serverID), OnlineTTL).Err()
}

// CommandTo 给目标 agent 推一条命令；不在线则报错。
var ErrNotConnected = errors.New("agent not connected")

func (h *Hub) CommandTo(serverID string, msg any) error {
	h.mu.RLock()
	e, ok := h.conns[serverID]
	h.mu.RUnlock()
	if !ok {
		return ErrNotConnected
	}
	return e.conn.WriteJSON(msg)
}

// StartSweeper 后台循环：把 DB 里 online 但 Redis 键失效的服务器置 offline。
func (h *Hub) StartSweeper(ctx context.Context) {
	t := time.NewTicker(SweeperPeriod)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.sweepOnce(ctx)
		}
	}
}

func (h *Hub) sweepOnce(ctx context.Context) {
	ids, err := h.servers.ListOnlineIDs(ctx)
	if err != nil || len(ids) == 0 {
		return
	}
	stale := make([]string, 0, len(ids))
	for _, id := range ids {
		n, err := h.rds.Exists(ctx, onlineKey(id)).Result()
		if err != nil {
			continue
		}
		if n == 0 {
			stale = append(stale, id)
		}
	}
	if len(stale) == 0 {
		return
	}
	if err := h.servers.SetOfflineByIDs(ctx, stale, time.Now().UTC()); err != nil {
		log.Printf("[hub] sweeper: %v", err)
		return
	}
	log.Printf("[hub] sweeper marked offline: %v", stale)

	// 关掉残留 conn（应当已不存在，保险起见）
	h.mu.Lock()
	for _, id := range stale {
		if e, ok := h.conns[id]; ok {
			_ = e.conn.Close()
			delete(h.conns, id)
		}
	}
	h.mu.Unlock()
}
