package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/store"
)

type MetricsHandler struct {
	Store *store.Metrics
}

// GET /api/servers/:id/metrics?range=24h
func (h *MetricsHandler) List(c *gin.Context) {
	id := c.Param("id")
	rng := c.DefaultQuery("range", "24h")
	d, err := time.ParseDuration(rng)
	if err != nil || d <= 0 || d > 7*24*time.Hour {
		d = 24 * time.Hour
	}
	points, err := h.Store.ListRange(c.Request.Context(), id, time.Now().UTC().Add(-d))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db"})
		return
	}
	out := make([]gin.H, 0, len(points))
	for _, p := range points {
		out = append(out, metricPointAPI(p))
	}
	c.JSON(http.StatusOK, gin.H{"points": out})
}

// metricPointAPI 转成前端 MetricPoint 形状：cpu/mem/disk 为百分比，netUp/netDown 为 MB/s。
func metricPointAPI(p store.MetricPoint) gin.H {
	memPct := 0.0
	if p.MemTotal > 0 {
		memPct = float64(p.MemUsed) / float64(p.MemTotal) * 100
	}
	diskPct := 0.0
	if p.DiskTotal > 0 {
		diskPct = float64(p.DiskUsed) / float64(p.DiskTotal) * 100
	}
	return gin.H{
		"ts":      p.TS.UTC().Format(time.RFC3339),
		"cpu":     roundPct(p.CPUPct),
		"mem":     roundPct(memPct),
		"disk":    roundPct(diskPct),
		"netUp":   roundMBs(p.NetTx),
		"netDown": roundMBs(p.NetRx),
	}
}

func roundPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return float64(int(v*10+0.5)) / 10
}

func roundMBs(bytes int64) float64 {
	mb := float64(bytes) / 1024.0 / 1024.0
	return float64(int(mb*100+0.5)) / 100
}
