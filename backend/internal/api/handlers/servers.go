package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/agentauth"
	"github.com/guardian/backend/internal/agenthub"
	"github.com/guardian/backend/internal/store"
)

type ServersHandler struct {
	Store          *store.Servers
	Metrics        *store.Metrics
	ConsoleBaseURL string // 例如 https://guardian.example.com —— 用于拼安装命令
	Hub            *agenthub.Hub
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

	log.Printf("[AUDIT] [Action:CreateServer] [ServerID:%s] [Name:%s]", id, name)

	cmd := fmt.Sprintf(
		"curl -fsSL %s/install.sh | sudo bash -s -- --token %s --console %s",
		h.ConsoleBaseURL, enrollment, h.ConsoleBaseURL,
	)

	c.JSON(http.StatusOK, gin.H{
		"server": apiServer(&store.Server{
			ID:     id,
			Name:   name,
			Status: "offline",
		}, store.MetricPoint{}),
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
	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	latest := map[string]store.MetricPoint{}
	if h.Metrics != nil {
		if m, err := h.Metrics.LastFor(c.Request.Context(), ids); err == nil {
			latest = m
		}
	}

	servers := make([]gin.H, 0, len(rows))
	online, offline, protected := 0, 0, 0
	for i := range rows {
		sv := apiServer(&rows[i], latest[rows[i].ID])
		servers = append(servers, sv)
		switch rows[i].Status {
		case "online":
			online++
			protected++ // T15 接入后用真实加固状态替换
		default:
			offline++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
		"summary": gin.H{
			"total":          len(rows),
			"online":         online,
			"offline":        offline,
			"protected":      protected,
			"pending":        0, // T11+T15 接通后回填
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
	var last store.MetricPoint
	if h.Metrics != nil {
		if m, err := h.Metrics.LastFor(c.Request.Context(), []string{id}); err == nil {
			last = m[id]
		}
	}
	c.JSON(http.StatusOK, gin.H{"server": apiServer(sv, last)})
}

// apiServer 把 store.Server + 最新一条指标映射成前端 SPEC §5 的 Server 形状。
func apiServer(sv *store.Server, last store.MetricPoint) gin.H {
	metrics := gin.H{
		"cpu":     0.0,
		"mem":     0.0,
		"disk":    0.0,
		"netUp":   0.0,
		"netDown": 0.0,
	}
	if !last.TS.IsZero() {
		metrics["cpu"] = roundPct(last.CPUPct)
		if last.MemTotal > 0 {
			metrics["mem"] = roundPct(float64(last.MemUsed) / float64(last.MemTotal) * 100)
		}
		if last.DiskTotal > 0 {
			metrics["disk"] = roundPct(float64(last.DiskUsed) / float64(last.DiskTotal) * 100)
		}
		metrics["netUp"] = roundMBs(last.NetTx)
		metrics["netDown"] = roundMBs(last.NetRx)
	}

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
		"metrics":             metrics,
		"attacksBlockedToday": 0,
	}
	if sv.LastSeenAt != nil {
		out["lastSeen"] = sv.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if sv.Distro != nil || sv.AgentVersion != nil || last.UptimeSec > 0 {
		out["system"] = gin.H{
			"distro": strPtrOr(sv.Distro, "—"),
			"kernel": "—",
			"uptime": formatUptime(last.UptimeSec),
			"agent":  strPtrOr(sv.AgentVersion, "—"),
		}
	}
	return out
}

func formatUptime(sec int64) string {
	if sec <= 0 {
		return "—"
	}
	days := sec / 86400
	hours := (sec % 86400) / 3600
	mins := (sec % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%d 天 %d 小时", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d 小时 %d 分", hours, mins)
	}
	return fmt.Sprintf("%d 分钟", mins)
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

type updateServerReq struct {
	Name string `json:"name" binding:"required"`
}

// PUT /api/servers/:id
func (h *ServersHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request", "message": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request", "message": "name cannot be empty"})
		return
	}

	if err := h.Store.UpdateName(c.Request.Context(), id, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db", "message": err.Error()})
		return
	}

	log.Printf("[AUDIT] [ServerID:%s] [Action:RenameServer] [NewName:%s]", id, name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/servers/:id
func (h *ServersHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if h.Hub != nil {
		h.Hub.Disconnect(id)
	}
	if err := h.Store.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db", "message": err.Error()})
		return
	}
	log.Printf("[AUDIT] [ServerID:%s] [Action:DeleteServer]", id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /install.sh
func (h *ServersHandler) DownloadInstallScript(c *gin.Context) {
	filePath := "/app/install.sh"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		filePath = "../install.sh"
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "install script not found"})
			return
		}
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.File(filePath)
}

// GET /api/agent/download?arch=amd64
func (h *ServersHandler) DownloadAgent(c *gin.Context) {
	arch := c.Query("arch")
	if arch != "amd64" && arch != "arm64" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request", "message": "unsupported architecture"})
		return
	}

	filePath := fmt.Sprintf("/app/agents/agent-linux-%s", arch)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		filePath = fmt.Sprintf("../agent/agent-linux-%s", arch)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			filePath = "../agent/agent_bin"
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "agent binary not found"})
				return
			}
		}
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=guardian-agent-%s", arch))
	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
}
