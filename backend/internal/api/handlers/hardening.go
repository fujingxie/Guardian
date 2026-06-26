package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/agenthub"
	"github.com/guardian/backend/internal/store"
)

type HardeningHandler struct {
	Store   *store.Hardening
	Servers *store.Servers
	Hub     *agenthub.Hub
}

// GET /api/servers/:id/hardening
func (h *HardeningHandler) GetHardening(c *gin.Context) {
	id := c.Param("id")

	// 确保服务器存在
	sv, err := h.Servers.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	items, err := h.Store.ListItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	jobs, err := h.Store.GetLatestJobs(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		job := jobs[it.Key]
		out = append(out, mapHardeningItem(it, job, sv))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// POST /api/servers/:id/hardening/:key/apply
func (h *HardeningHandler) ApplyHardening(c *gin.Context) {
	id := c.Param("id")
	key := c.Param("key")

	sv, err := h.Servers.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	if sv.Status != "online" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server_offline"})
		return
	}

	// 1. 任务幂等：检查最近 60 秒内是否已经存在 pending 或 trial 的任务
	if latestJob, err := h.Store.GetLatestJobByKey(c.Request.Context(), id, key); err == nil && latestJob != nil {
		if (latestJob.Status == "pending" || latestJob.Status == "trial") && time.Since(latestJob.CreatedAt) < 60*time.Second {
			c.JSON(http.StatusOK, gin.H{"ok": true, "jobId": latestJob.ID})
			return
		}
	}

	// 2. 开启新任务
	jobID, err := h.Store.CreateJob(c.Request.Context(), id, key, "pending")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	adminIP := c.ClientIP()
	if adminIP != "" {
		// 记录当前访问 IP，方便 Agent 写入白名单防止误拦截
		if err := h.Servers.UpdateAdminIP(c.Request.Context(), id, adminIP); err != nil {
			log.Printf("[handlers] UpdateAdminIP %s error: %v", id, err)
		}
	}

	// 动态从数据库中单点读取判定风险等级
	isHighRisk := false
	if item, err := h.Store.GetItem(c.Request.Context(), key); err == nil && item != nil {
		isHighRisk = item.RiskLevel == "high"
	}

	// WSS 命令投递
	cmdMsg := map[string]any{
		"type": "command",
		"payload": map[string]any{
			"cmd":      "run_hardening",
			"jobId":    jobID,
			"key":      key,
			"adminIp":  adminIP,
			"highRisk": isHighRisk,
		},
	}
	if err := h.Hub.CommandTo(id, cmdMsg); err != nil {
		// 回滚数据库状态为 failed
		failedErr := "agent not connected"
		if err := h.Store.UpdateJobStatus(c.Request.Context(), jobID, "failed", &failedErr); err != nil {
			log.Printf("[handlers] UpdateJobStatus failed rollback error: %v", err)
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_disconnected"})
		return
	}

	log.Printf("[AUDIT] [ServerID:%s] [Action:ApplyHardening] [Key:%s] [JobID:%s]", id, key, jobID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "jobId": jobID})
}

// POST /api/servers/:id/hardening/:key/confirm
func (h *HardeningHandler) ConfirmHardening(c *gin.Context) {
	id := c.Param("id")
	key := c.Param("key")

	job, err := h.Store.GetLatestJobByKey(c.Request.Context(), id, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job_not_found"})
		return
	}

	if job.Status != "trial" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_not_in_trial"})
		return
	}

	// 1. 投递 confirm 指令给 Agent 清除本地死人开关；必须投递成功才置为 applied
	cmdMsg := map[string]any{
		"type": "command",
		"payload": map[string]any{
			"cmd":   "confirm_hardening",
			"jobId": job.ID,
			"key":   key,
		},
	}
	if err := h.Hub.CommandTo(id, cmdMsg); err != nil {
		log.Printf("[handlers] failed to deliver confirm cmd to Agent %s: %v", id, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_disconnected"})
		return
	}

	// 2. 更新为 applied 状态
	if err := h.Store.UpdateJobStatus(c.Request.Context(), job.ID, "applied", nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	log.Printf("[AUDIT] [ServerID:%s] [Action:ConfirmHardening] [Key:%s] [JobID:%s]", id, key, job.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/servers/:id/hardening/:key/rollback
func (h *HardeningHandler) RollbackHardening(c *gin.Context) {
	id := c.Param("id")
	key := c.Param("key")

	job, err := h.Store.GetLatestJobByKey(c.Request.Context(), id, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job_not_found"})
		return
	}

	if job.Status != "trial" && job.Status != "applied" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_cannot_rollback"})
		return
	}

	// 标记为 rolledback
	if err := h.Store.UpdateJobStatus(c.Request.Context(), job.ID, "rolledback", nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}

	// 投递 rollback 消息给 Agent
	var files map[string]any
	if len(job.SnapshotFiles) > 0 {
		_ = json.Unmarshal(job.SnapshotFiles, &files)
	}

	cmdMsg := map[string]any{
		"type": "command",
		"payload": map[string]any{
			"cmd":   "rollback",
			"jobId": job.ID,
			"key":   key,
			"files": files,
		},
	}
	if err := h.Hub.CommandTo(id, cmdMsg); err != nil {
		log.Printf("[handlers] rollback CommandTo %s error: %v", id, err)
	}

	log.Printf("[AUDIT] [ServerID:%s] [Action:RollbackHardening] [Key:%s] [JobID:%s]", id, key, job.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func mapHardeningItem(it store.HardeningItem, job *store.HardeningJob, sv *store.Server) gin.H {
	enabled := false
	status := "idle"
	var trialObj any = nil

	if job != nil {
		switch job.Status {
		case "applied":
			enabled = true
			status = "idle"
		case "trial":
			enabled = true
			status = "trial"
			if job.ConfirmDeadline != nil {
				trialObj = gin.H{
					"rollbackAt": job.ConfirmDeadline.UTC().Format(time.RFC3339),
				}
			}
		case "pending":
			enabled = false
			status = "applying"
		case "failed":
			enabled = false
			status = "failed"
		default:
			enabled = false
			status = "idle"
		}
	}

	val := ""
	// 放行端口时展示值
	if it.Key == "ufw_ports" {
		val = "22, 80, 443" // 默认端口
	} else if it.Key == "ssh_port" && sv.CurrentAdminIP != nil {
		// 可以附加值展示
	}

	return gin.H{
		"key":              it.Key,
		"group":            it.Category,
		"title":            it.Title,
		"plainExplanation": it.PlainExplanation,
		"enabled":          enabled,
		"highRisk":         it.RiskLevel == "high",
		"value":            val,
		"status":           status,
		"trial":            trialObj,
	}
}
