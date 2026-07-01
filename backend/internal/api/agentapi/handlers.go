// Package agentapi 实现 agent 协议端点：enroll（HTTP）+ WSS。
// 注意：这两个端点不挂访问口令中间件，agent 自带 enroll/agent token。
package agentapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/guardian/backend/internal/agentauth"
	"github.com/guardian/backend/internal/agenthub"
	"github.com/guardian/backend/internal/explain"
	"github.com/guardian/backend/internal/notify"
	"github.com/guardian/backend/internal/store"
	"github.com/guardian/backend/internal/threshold"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	Servers          *store.Servers
	Metrics          *store.Metrics
	Hardening        *store.Hardening
	Hub              *agenthub.Hub
	Alerts           *store.Alerts
	Explain          *explain.Service
	Notify           *notify.Service
	Inventory        *store.InventoryStore
	Redis            *redis.Client
	ThresholdChecker *threshold.Checker
}

type metricsPayload struct {
	TS        time.Time `json:"ts"`
	CPUPct    float64   `json:"cpuPct"`
	MemUsed   int64     `json:"memUsed"`
	MemTotal  int64     `json:"memTotal"`
	DiskUsed  int64     `json:"diskUsed"`
	DiskTotal int64     `json:"diskTotal"`
	NetRx     int64     `json:"netRx"`
	NetTx     int64     `json:"netTx"`
	Load1     float64   `json:"load1"`
	UptimeSec int64     `json:"uptimeSec"`
	Distro    string    `json:"distro"`
	Kernel    string    `json:"kernel"`
}

type enrollReq struct {
	EnrollmentToken string `json:"enrollmentToken"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
}

type enrollResp struct {
	ServerID   string `json:"serverId"`
	AgentToken string `json:"agentToken"`
}

// POST /api/agent/enroll —— 用一次性 token 换长期 agent token。
func (h *Handler) Enroll(c *gin.Context) {
	var req enrollReq
	if err := c.ShouldBindJSON(&req); err != nil || req.EnrollmentToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request"})
		return
	}

	hash := agentauth.Hash(req.EnrollmentToken)
	sv, err := h.Servers.LookupByTokenHash(c.Request.Context(), hash)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	// 校验：必须仍是 enroll_ 前缀（即未换过）；否则视为重放。
	// 真实落库的是哈希，看不到前缀，所以只能允许一次性使用：换 token 后哈希变了，原 enrollment 再来就 lookup not found。
	agentToken := agentauth.NewAgentToken()
	newHash := agentauth.Hash(agentToken)

	if err := h.Servers.RotateToken(
		c.Request.Context(),
		sv.ID, newHash,
		req.Hostname, req.OS, req.Arch, "0.1.0",
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	log.Printf("[agentapi] enrolled %s (%s/%s)", sv.ID, req.OS, req.Arch)
	c.JSON(http.StatusOK, enrollResp{ServerID: sv.ID, AgentToken: agentToken})
}

// 升级 WSS 时用：允许任何 Origin（鉴权走 Bearer token）。
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// GET /api/agent/ws —— agent 升级为 WSS。
func (h *Handler) WebSocket(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" || token == auth {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing_token"})
		return
	}

	sv, err := h.Servers.LookupByTokenHash(c.Request.Context(), agentauth.Hash(token))
	if errors.Is(err, store.ErrNotFound) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[agentapi] upgrade %s: %v", sv.ID, err)
		return
	}
	wsConn := &wsConnAdapter{Conn: conn}

	ctx := context.Background()
	e, err := h.Hub.Register(ctx, sv.ID, wsConn)
	if err != nil {
		log.Printf("[agentapi] hub register %s: %v", sv.ID, err)
		_ = conn.Close()
		return
	}
	log.Printf("[agentapi] %s connected", sv.ID)

	defer func() {
		h.Hub.Unregister(ctx, sv.ID, e)
		_ = conn.Close()
		log.Printf("[agentapi] %s disconnected", sv.ID)
	}()

	// 读循环 —— 任何消息都刷新 TTL；按 type 分发后续处理（T8/T13 接入）。
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			log.Printf("[agentapi] %s bad json: %v", sv.ID, err)
			continue
		}
		h.Hub.Touch(ctx, sv.ID)

		switch env.Type {
		case "heartbeat", "hello":
			// 仅刷新在线 TTL（上面已做）
		case "metrics":
			h.handleMetrics(ctx, sv.ID, env.Payload)
		case "event":
			h.handleEvent(ctx, sv.ID, env.Payload)
		case "job_result":
			h.handleJobResult(ctx, sv.ID, env.Payload)
		case "inventory":
			h.handleInventory(ctx, sv.ID, env.Payload)
		default:
			log.Printf("[agentapi] %s recv %s (ignored)", sv.ID, env.Type)
		}
	}
}

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// wsConnAdapter 把 *websocket.Conn 适配成 agenthub.Conn 接口。
type wsConnAdapter struct{ *websocket.Conn }

func (w *wsConnAdapter) WriteJSON(v any) error { return w.Conn.WriteJSON(v) }
func (w *wsConnAdapter) Close() error          { return w.Conn.Close() }

func (h *Handler) handleMetrics(ctx context.Context, serverID string, raw json.RawMessage) {
	if h.Metrics == nil {
		return
	}
	var p metricsPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("[agentapi] %s bad metrics: %v", serverID, err)
		return
	}
	if p.TS.IsZero() {
		p.TS = time.Now().UTC()
	}
	point := store.MetricPoint{
		ServerID:  serverID,
		TS:        p.TS,
		CPUPct:    p.CPUPct,
		MemUsed:   p.MemUsed,
		MemTotal:  p.MemTotal,
		DiskUsed:  p.DiskUsed,
		DiskTotal: p.DiskTotal,
		NetRx:     p.NetRx,
		NetTx:     p.NetTx,
		Load1:     p.Load1,
		UptimeSec: p.UptimeSec,
	}
	if err := h.Metrics.Insert(ctx, point); err != nil {
		log.Printf("[agentapi] %s insert metrics: %v", serverID, err)
	}
	if p.Distro != "" {
		if err := h.Servers.UpdateDistro(ctx, serverID, p.Distro); err != nil {
			log.Printf("[agentapi] %s update distro error: %v", serverID, err)
		}
	}

	if h.ThresholdChecker != nil {
		serverName := serverID
		if h.Hub != nil {
			serverName = h.Hub.GetServerName(ctx, serverID)
		}
		h.ThresholdChecker.Check(ctx, serverID, serverName, point)
	}
}

type jobResultPayload struct {
	JobID    string            `json:"jobId"`
	Key      string            `json:"key"`
	Status   string            `json:"status"` // success | failed
	Snapshot map[string]string `json:"snapshot"`
	Error    string            `json:"error"`
}

func (h *Handler) handleJobResult(ctx context.Context, serverID string, raw json.RawMessage) {
	if h.Hardening == nil {
		return
	}
	var p jobResultPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("[agentapi] %s bad job_result: %v", serverID, err)
		return
	}

	_, err := h.Hardening.GetJob(ctx, p.JobID)
	if err != nil {
		log.Printf("[agentapi] %s get job %s: %v", serverID, p.JobID, err)
		return
	}

	if p.Status == "failed" {
		log.Printf("[agentapi] %s job %s failed: %s", serverID, p.JobID, p.Error)
		if err := h.Hardening.UpdateJobStatus(ctx, p.JobID, "failed", &p.Error); err != nil {
			log.Printf("[agentapi] %s UpdateJobStatus failed error: %v", serverID, err)
		}
		return
	}

	if p.Status == "rolledback" {
		log.Printf("[agentapi] %s job %s reported rolledback: %s", serverID, p.JobID, p.Error)
		if err := h.Hardening.UpdateJobStatus(ctx, p.JobID, "rolledback", &p.Error); err != nil {
			log.Printf("[agentapi] %s UpdateJobStatus rolledback error: %v", serverID, err)
		}
		return
	}

	// 成功逻辑
	// 动态从数据库中单点读取判定风险等级
	isHighRisk := false
	if item, err := h.Hardening.GetItem(ctx, p.Key); err == nil && item != nil {
		isHighRisk = item.RiskLevel == "high"
	}

	if !isHighRisk {
		// 普通加固项直接 applied
		if err := h.Hardening.UpdateJobStatus(ctx, p.JobID, "applied", nil); err != nil {
			log.Printf("[agentapi] %s update job %s: %v", serverID, p.JobID, err)
		}
		return
	}

	// 高风险项进入 5 分钟试运行
	snapshotJSON, err := json.Marshal(p.Snapshot)
	if err != nil {
		log.Printf("[agentapi] %s marshal snapshot: %v", serverID, err)
		return
	}

	snapshotID, err := h.Hardening.SaveSnapshot(ctx, serverID, p.JobID, snapshotJSON)
	if err != nil {
		log.Printf("[agentapi] %s save snapshot: %v", serverID, err)
		return
	}

	// 默认试运行 5 分钟
	deadline := time.Now().Add(5 * time.Minute)
	if err := h.Hardening.UpdateJobToTrial(ctx, p.JobID, deadline, snapshotID); err != nil {
		log.Printf("[agentapi] %s update job to trial %s: %v", serverID, p.JobID, err)
	} else {
		log.Printf("[agentapi] %s job %s entered trial, deadline: %s", serverID, p.JobID, deadline.Format(time.RFC3339))
	}
}

type eventPayload struct {
	TS        time.Time      `json:"ts"`
	EventType string         `json:"eventType"`
	SourceIP  string         `json:"sourceIp"`
	Detail    map[string]any `json:"detail"`
}

func (h *Handler) handleEvent(ctx context.Context, serverID string, raw json.RawMessage) {
	if h.Alerts == nil || h.Explain == nil {
		return
	}

	var p eventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("[agentapi] %s bad event json: %v", serverID, err)
		return
	}

	if p.TS.IsZero() {
		p.TS = time.Now().UTC()
	}

	// 1. 调用 Explain 翻译
	detailBytes, _ := json.Marshal(p.Detail)
	plainMsg, err := h.Explain.Translate(ctx, p.EventType, p.SourceIP, detailBytes)
	if err != nil {
		log.Printf("[agentapi] %s explain event error: %v", serverID, err)
		plainMsg = fmt.Sprintf("系统检测到安全事件（类型：%s），来源：%s。", p.EventType, p.SourceIP)
	}

	country := ""
	if p.SourceIP != "" && h.Explain != nil {
		c, _, _ := h.Explain.LookupIPDetail(ctx, p.SourceIP)
		if c != "" && c != "未知" {
			country = c
		}
	}

	// 严重程度 severity (根据 eventType，通常封禁可以是 medium，其它 info)
	severity := "info"
	if p.EventType == "bruteforce_blocked" {
		severity = "medium"
	} else if p.EventType == "port_scan" {
		severity = "medium"
	} else if p.EventType == "critical_attack" {
		severity = "high"
	}

	// 2. 落库 security_events
	ev, err := h.Alerts.CreateAlert(ctx, serverID, p.EventType, p.SourceIP, country, p.Detail, plainMsg, severity)
	if err != nil {
		log.Printf("[agentapi] %s save security event to db error: %v", serverID, err)
		return
	}

	log.Printf("[agentapi] %s security event saved: id=%s, type=%s, msg=%s", serverID, ev.ID, p.EventType, plainMsg)

	// 3. 多通道通知推送
	if h.Notify != nil {
		serverName := serverID
		if h.Hub != nil {
			serverName = h.Hub.GetServerName(ctx, serverID)
		}
		title := fmt.Sprintf("[%s] 安全告警: %s", serverName, formatEventTitle(p.EventType))
		h.Notify.SendAlert(ctx, p.EventType, title, plainMsg)
	}
}

func formatEventTitle(evt string) string {
	switch evt {
	case "bruteforce_blocked":
		return "拦截恶意登录"
	case "new_login":
		return "新登录提醒"
	case "port_scan":
		return "拦截端口扫描"
	default:
		return "安全警告"
	}
}

type inventoryPayload struct {
	TS       time.Time `json:"ts"`
	Ports    any       `json:"ports"`
	Services any       `json:"services"`
	Packages any       `json:"packages"`
}

func (h *Handler) handleInventory(ctx context.Context, serverID string, raw json.RawMessage) {
	if h.Inventory == nil {
		return
	}
	var p inventoryPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("[agentapi] %s bad inventory payload: %v", serverID, err)
		return
	}
	if p.TS.IsZero() {
		p.TS = time.Now().UTC()
	}

	if err := h.Inventory.Upsert(ctx, serverID, p.TS, p.Ports, p.Services, p.Packages); err != nil {
		log.Printf("[agentapi] %s upsert inventory: %v", serverID, err)
	}
}
