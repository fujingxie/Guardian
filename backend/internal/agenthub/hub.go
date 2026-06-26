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
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/guardian/backend/internal/explain"
	"github.com/guardian/backend/internal/notify"
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
	name string
}

type Hub struct {
	rds     *redis.Client
	servers *store.Servers
	alerts  *store.Alerts
	explain *explain.Service
	notify  *notify.Service

	mu    sync.RWMutex
	conns map[string]*entry // serverID -> entry
}

func New(rds *redis.Client, servers *store.Servers, alerts *store.Alerts, exp *explain.Service, notify *notify.Service) *Hub {
	return &Hub{
		rds:     rds,
		servers: servers,
		alerts:  alerts,
		explain: exp,
		notify:  notify,
		conns:   make(map[string]*entry),
	}
}

// Register 当 agent 建立 WSS 连接时调用：登记 conn、置 online。
// 如果同一台服务器已有旧连接，旧连接会被关闭（避免假在线）。
func (h *Hub) Register(ctx context.Context, serverID string, c Conn) (*entry, error) {
	name := serverID
	if h.servers != nil {
		if sv, err := h.servers.Get(ctx, serverID); err == nil && sv != nil {
			name = sv.Name
		}
	}

	h.mu.Lock()
	if old, ok := h.conns[serverID]; ok {
		_ = old.conn.Close()
	}
	e := &entry{conn: c, id: serverID, name: name}
	h.conns[serverID] = e
	h.mu.Unlock()

	if h.rds != nil {
		if err := h.rds.Set(ctx, onlineKey(serverID), "1", OnlineTTL).Err(); err != nil {
			h.mu.Lock()
			if h.conns[serverID] == e {
				delete(h.conns, serverID)
			}
			h.mu.Unlock()
			return nil, err
		}
	}
	if h.servers != nil {
		if err := h.servers.SetStatus(ctx, serverID, "online", time.Now().UTC()); err != nil {
			h.mu.Lock()
			if h.conns[serverID] == e {
				delete(h.conns, serverID)
			}
			h.mu.Unlock()
			return nil, err
		}
	}
	return e, nil
}

// Unregister 当连接断开时调用：摘掉 conn、置 offline。
func (h *Hub) Unregister(ctx context.Context, serverID string, e *entry) {
	h.mu.Lock()
	curr, exists := h.conns[serverID]
	if exists && curr == e {
		delete(h.conns, serverID)
	} else {
		exists = false
	}
	h.mu.Unlock()

	if exists {
		if h.rds != nil {
			if err := h.rds.Del(ctx, onlineKey(serverID)).Err(); err != nil {
				log.Printf("[hub] delete online key from redis %s error: %v", serverID, err)
			}
		}
		if h.servers != nil {
			if err := h.servers.SetStatus(ctx, serverID, "offline", time.Now().UTC()); err != nil {
				log.Printf("[hub] mark offline %s: %v", serverID, err)
			}
		}
		go h.triggerOfflineAlert(context.Background(), serverID)
	}
}

// Touch 刷新在线 TTL（心跳/任意 agent 消息触发）。
func (h *Hub) Touch(ctx context.Context, serverID string) {
	_ = h.rds.Expire(ctx, onlineKey(serverID), OnlineTTL).Err()
}

// Disconnect 主动断开指定服务器的 WebSocket 连接（比如用户删除服务器时），以防止其误报离线。
func (h *Hub) Disconnect(serverID string) {
	h.mu.Lock()
	e, ok := h.conns[serverID]
	if ok {
		delete(h.conns, serverID)
		_ = e.conn.Close()
	}
	h.mu.Unlock()

	// 从 Redis 删除在线标记
	_ = h.rds.Del(context.Background(), onlineKey(serverID)).Err()
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

	// 触发离线告警和通知
	for _, id := range stale {
		go h.triggerOfflineAlert(context.Background(), id)
	}
}

func (h *Hub) triggerOfflineAlert(ctx context.Context, serverID string) {
	if h.alerts == nil || h.explain == nil {
		return
	}

	serverName := serverID
	if sv, err := h.servers.Get(ctx, serverID); err == nil && sv != nil {
		serverName = sv.Name
	}

	plainMsg, err := h.explain.Translate(ctx, "offline", serverName, nil)
	if err != nil {
		plainMsg = fmt.Sprintf("服务器离线！Guardian 失去与服务器 %s 的长连接心跳，请检查服务器网络或 Agent 状态。", serverName)
	}

	ev, err := h.alerts.CreateAlert(ctx, serverID, "offline", "", "", nil, plainMsg, "high")
	if err != nil {
		log.Printf("[hub] save offline alert error: %v", err)
		return
	}
	log.Printf("[hub] server %s offline alert saved: id=%s", serverID, ev.ID)

	if h.notify != nil {
		title := fmt.Sprintf("[%s] 紧急告警: 服务器离线", serverName)
		h.notify.Send(ctx, title, plainMsg)
	}
}

// GetServerName returns the cached server name for an active connection, 
// or falls back to DB query, or serverID as a last resort.
func (h *Hub) GetServerName(ctx context.Context, serverID string) string {
	h.mu.RLock()
	e, ok := h.conns[serverID]
	if ok && e.name != "" {
		name := e.name
		h.mu.RUnlock()
		return name
	}
	h.mu.RUnlock()

	if h.servers != nil {
		if sv, err := h.servers.Get(ctx, serverID); err == nil && sv != nil {
			h.mu.Lock()
			if e, ok := h.conns[serverID]; ok {
				e.name = sv.Name
			}
			h.mu.Unlock()
			return sv.Name
		}
	}
	return serverID
}
