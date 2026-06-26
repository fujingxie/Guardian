package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/notify"
	"github.com/guardian/backend/internal/store"
)

type AlertsHandler struct {
	Store         *store.Alerts
	Servers       *store.Servers
	NotifyService *notify.Service
}

type AlertResp struct {
	ID       string  `json:"id"`
	TS       string  `json:"ts"`
	Kind     string  `json:"kind"`
	SourceIP *string `json:"sourceIp,omitempty"`
	Severity string  `json:"severity"`
	Read     bool    `json:"read"`
	Resolved bool    `json:"resolved"`
	Title    string  `json:"title"`
	Message  string  `json:"message"`
}

// GET /api/servers/:id/alerts
func (h *AlertsHandler) GetAlerts(c *gin.Context) {
	serverID := c.Param("id")

	events, err := h.Store.ListAlerts(c.Request.Context(), serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db", "message": err.Error()})
		return
	}

	alerts := make([]AlertResp, 0, len(events))
	for _, ev := range events {
		alerts = append(alerts, formatAlert(ev))
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

// GET /api/settings/notifications
func (h *AlertsHandler) GetSettings(c *gin.Context) {
	ns, err := h.Store.GetNotifySettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db", "message": err.Error()})
		return
	}

	var channels map[string]any
	if len(ns.Channels) > 0 {
		_ = json.Unmarshal(ns.Channels, &channels)
	}

	if channels == nil {
		channels = make(map[string]any)
	}

	// 补全默认字段，避免前端解构报错
	for _, key := range []string{"email", "telegram", "serverChan"} {
		if _, ok := channels[key]; !ok {
			channels[key] = ""
		}
	}

	var enabled map[string]any
	if val, ok := channels["enabled"]; ok {
		if m, ok := val.(map[string]any); ok {
			enabled = m
		}
	}
	if enabled == nil {
		enabled = make(map[string]any)
		channels["enabled"] = enabled
	}
	for _, key := range []string{"email", "telegram", "serverChan"} {
		if _, ok := enabled[key]; !ok {
			enabled[key] = false
		}
	}

	count, err := h.Servers.CountOnline(c.Request.Context())
	if err != nil {
		count = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"notify": channels,
		"version": gin.H{
			"console":      "0.1.0",
			"agent":        "0.1.0",
			"agentsOnline": fmt.Sprintf("%d", count),
		},
	})
}

type updateSettingsReq struct {
	Notify map[string]any `json:"notify"`
}

// PUT /api/settings/notifications
func (h *AlertsHandler) UpdateSettings(c *gin.Context) {
	var req updateSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request", "message": err.Error()})
		return
	}

	channelsJSON, err := json.Marshal(req.Notify)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal", "message": err.Error()})
		return
	}

	if err := h.Store.UpdateNotifySettings(c.Request.Context(), channelsJSON, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db", "message": err.Error()})
		return
	}

	count, err := h.Servers.CountOnline(c.Request.Context())
	if err != nil {
		count = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"notify": req.Notify,
		"version": gin.H{
			"console":      "0.1.0",
			"agent":        "0.1.0",
			"agentsOnline": fmt.Sprintf("%d", count),
		},
	})
}

type testNotificationReq struct {
	Channel string `json:"channel"`
}

// POST /api/settings/notifications/test
func (h *AlertsHandler) TestNotification(c *gin.Context) {
	var req testNotificationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request", "message": err.Error()})
		return
	}

	if err := h.NotifyService.TestChannel(c.Request.Context(), req.Channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "测试通知已成功发送，请查收对应的通知渠道是否正常。",
	})
}

func formatAlert(ev store.SecurityEvent) AlertResp {
	kind := ev.Type
	// 前端只接收 'bruteforce' | 'port_scan' | 'new_login' | 'unknown'
	if kind != "bruteforce" && kind != "port_scan" && kind != "new_login" && kind != "unknown" {
		if kind == "bruteforce_blocked" {
			kind = "bruteforce"
		} else {
			kind = "unknown"
		}
	}

	title := "系统告警"
	switch ev.Type {
	case "bruteforce", "bruteforce_blocked":
		title = "恶意登录拦截"
	case "new_login":
		title = "新登录提醒"
	case "port_scan":
		title = "端口扫描告警"
	case "offline":
		title = "服务器离线告警"
	}

	return AlertResp{
		ID:       ev.ID,
		TS:       ev.CreatedAt.Format(time.RFC3339),
		Kind:     kind,
		SourceIP: ev.SourceIP,
		Severity: ev.Severity,
		Read:     ev.Status == "seen" || ev.Status == "resolved",
		Resolved: ev.Status == "resolved",
		Title:    title,
		Message:  ev.PlainExplanation,
	}
}
