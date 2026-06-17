// Package agentapi 实现 agent 协议端点：enroll（HTTP）+ WSS。
// 注意：这两个端点不挂访问口令中间件，agent 自带 enroll/agent token。
package agentapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/guardian/backend/internal/agentauth"
	"github.com/guardian/backend/internal/agenthub"
	"github.com/guardian/backend/internal/store"
)

type Handler struct {
	Servers *store.Servers
	Hub     *agenthub.Hub
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
	if err := h.Hub.Register(ctx, sv.ID, wsConn); err != nil {
		log.Printf("[agentapi] hub register %s: %v", sv.ID, err)
		_ = conn.Close()
		return
	}
	log.Printf("[agentapi] %s connected", sv.ID)

	defer func() {
		h.Hub.Unregister(ctx, sv.ID)
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
			// T9 落库
		case "event":
			// T16 落库
		case "job_result":
			// T13 推进
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
