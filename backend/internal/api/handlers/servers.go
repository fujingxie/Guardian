package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/agentauth"
	"github.com/guardian/backend/internal/store"
)

type ServersHandler struct {
	Store          *store.Servers
	ConsoleBaseURL string // 例如 https://guardian.example.com —— 用于拼安装命令
}

type addServerReq struct {
	Name string `json:"name"`
}

// POST /api/servers
func (h *ServersHandler) Create(c *gin.Context) {
	var req addServerReq
	_ = c.ShouldBindJSON(&req) // 名称可选，绑定失败也无所谓

	id := generateServerID(req.Name)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = id
	}

	enrollment := agentauth.NewEnrollmentToken()
	hash := agentauth.Hash(enrollment)

	if err := h.Store.Insert(c.Request.Context(), id, name, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db", "message": err.Error()})
		return
	}

	cmd := fmt.Sprintf(
		"curl -fsSL %s/install.sh | sudo bash -s -- --token %s --console %s",
		h.ConsoleBaseURL, enrollment, h.ConsoleBaseURL,
	)

	c.JSON(http.StatusOK, gin.H{
		"server": apiServer(&store.Server{
			ID:     id,
			Name:   name,
			Status: "offline",
		}),
		"enrollmentToken": enrollment,
		"installCommand":  cmd,
	})
}

// GET /api/servers
func (h *ServersHandler) List(c *gin.Context) {
	rows, err := h.Store.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}
	servers := make([]gin.H, 0, len(rows))
	online, offline := 0, 0
	for i := range rows {
		sv := apiServer(&rows[i])
		servers = append(servers, sv)
		if rows[i].Status == "online" {
			online++
		} else {
			offline++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
		"summary": gin.H{
			"total":          len(rows),
			"online":         online,
			"offline":        offline,
			"protected":      0, // T11+T15 接通后回填
			"pending":        0,
			"pendingServers": 0,
			"todayBlocked":   0,
			"yesterdayDelta": 0,
		},
	})
}

// GET /api/servers/:id
func (h *ServersHandler) Get(c *gin.Context) {
	id := c.Param("id")
	sv, err := h.Store.Get(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"server": apiServer(sv)})
}

// apiServer 把 store.Server 映射成前端 SPEC §5 中的 Server 形状。
// 字段没有数据的（首次 enroll 前）走零值/空串，前端 SecurityBadge + Meter 已处理 offline 显示。
func apiServer(sv *store.Server) gin.H {
	out := gin.H{
		"id":     sv.ID,
		"name":   sv.Name,
		"ip":     strPtrOr(sv.CurrentAdminIP, ""),
		"status": sv.Status,
		"security": func() string {
			if sv.Status == "online" {
				return "protected"
			}
			return "danger"
		}(),
		"metrics": gin.H{
			"cpu": 0, "mem": 0, "disk": 0, "netUp": 0, "netDown": 0,
		},
		"attacksBlockedToday": 0,
	}
	if sv.LastSeenAt != nil {
		out["lastSeen"] = sv.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if sv.Distro != nil || sv.AgentVersion != nil {
		out["system"] = gin.H{
			"distro": strPtrOr(sv.Distro, "—"),
			"kernel": "—",
			"uptime": "—",
			"agent":  strPtrOr(sv.AgentVersion, "—"),
		}
	}
	return out
}

func strPtrOr(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

// generateServerID：优先使用 name 派生友好 ID，方便日志识别
func generateServerID(name string) string {
	name = sanitizeSlug(name)
	suffix := randSuffix(6)
	if name == "" {
		return "srv-" + suffix
	}
	return name + "-" + suffix
}

func sanitizeSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == ' ':
			b.WriteByte('-')
		}
		if b.Len() >= 28 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func randSuffix(n int) string {
	buf := make([]byte, (n+1)/2)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)[:n]
}
