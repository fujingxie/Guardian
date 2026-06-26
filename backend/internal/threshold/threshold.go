package threshold

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/guardian/backend/internal/notify"
	"github.com/guardian/backend/internal/store"
)

type Checker struct {
	rds         *redis.Client
	alertsStore *store.Alerts
	notify      *notify.Service
}

func NewChecker(rds *redis.Client, alertsStore *store.Alerts, notify *notify.Service) *Checker {
	return &Checker{
		rds:         rds,
		alertsStore: alertsStore,
		notify:      notify,
	}
}

type Config struct {
	CPUPctThreshold  float64 `json:"cpuPctThreshold"`
	CPUDurationMin   int     `json:"cpuDurationMin"`
	MemPctThreshold  float64 `json:"memPctThreshold"`
	MemDurationMin   int     `json:"memDurationMin"`
	DiskPctThreshold float64 `json:"diskPctThreshold"`
}

func (c *Checker) loadConfig(ctx context.Context) Config {
	cfg := Config{
		CPUPctThreshold:  85.0,
		CPUDurationMin:   5,
		MemPctThreshold:  95.0,
		MemDurationMin:   3,
		DiskPctThreshold: 95.0,
	}

	var ns *store.NotifySettings
	var err error
	if c.notify != nil {
		ns, err = c.notify.GetSettings(ctx)
	} else if c.alertsStore != nil {
		ns, err = c.alertsStore.GetNotifySettings(ctx)
	}
	if err != nil || ns == nil {
		return cfg
	}

	var m map[string]any
	if len(ns.Channels) > 0 {
		_ = json.Unmarshal(ns.Channels, &m)
	}
	if m == nil {
		return cfg
	}

	if val, ok := m["cpuPctThreshold"]; ok {
		if f, ok := val.(float64); ok {
			cfg.CPUPctThreshold = f
		} else if i, ok := val.(int); ok {
			cfg.CPUPctThreshold = float64(i)
		} else if i, ok := val.(int64); ok {
			cfg.CPUPctThreshold = float64(i)
		}
	}
	if val, ok := m["cpuDurationMin"]; ok {
		if f, ok := val.(float64); ok {
			cfg.CPUDurationMin = int(f)
		} else if i, ok := val.(int); ok {
			cfg.CPUDurationMin = i
		} else if i, ok := val.(int64); ok {
			cfg.CPUDurationMin = int(i)
		}
	}
	if val, ok := m["memPctThreshold"]; ok {
		if f, ok := val.(float64); ok {
			cfg.MemPctThreshold = f
		} else if i, ok := val.(int); ok {
			cfg.MemPctThreshold = float64(i)
		} else if i, ok := val.(int64); ok {
			cfg.MemPctThreshold = float64(i)
		}
	}
	if val, ok := m["memDurationMin"]; ok {
		if f, ok := val.(float64); ok {
			cfg.MemDurationMin = int(f)
		} else if i, ok := val.(int); ok {
			cfg.MemDurationMin = i
		} else if i, ok := val.(int64); ok {
			cfg.MemDurationMin = int(i)
		}
	}
	if val, ok := m["diskPctThreshold"]; ok {
		if f, ok := val.(float64); ok {
			cfg.DiskPctThreshold = f
		} else if i, ok := val.(int); ok {
			cfg.DiskPctThreshold = float64(i)
		} else if i, ok := val.(int64); ok {
			cfg.DiskPctThreshold = float64(i)
		}
	}

	return cfg
}

func (c *Checker) Check(ctx context.Context, serverID string, serverName string, point store.MetricPoint) {
	if c.rds == nil {
		return
	}

	cfg := c.loadConfig(ctx)

	// 1. CPU check
	c.checkMetric(ctx, serverID, serverName, "cpu", point.CPUPct, cfg.CPUPctThreshold, cfg.CPUDurationMin,
		"cpu_usage_high", "服务器 CPU 使用率持续超过 %v%%，当前值为 %.1f%%。")

	// 2. Memory check
	var memPct float64
	if point.MemTotal > 0 {
		memPct = float64(point.MemUsed) / float64(point.MemTotal) * 100
		c.checkMetric(ctx, serverID, serverName, "mem", memPct, cfg.MemPctThreshold, cfg.MemDurationMin,
			"mem_usage_high", "服务器 内存 使用率持续超过 %v%%，当前值为 %.1f%%。")
	}

	// 3. Disk check
	var diskPct float64
	if point.DiskTotal > 0 {
		diskPct = float64(point.DiskUsed) / float64(point.DiskTotal) * 100
		c.checkMetric(ctx, serverID, serverName, "disk", diskPct, cfg.DiskPctThreshold, 0,
			"disk_usage_high", "服务器 磁盘 使用率超过 %v%%，当前值为 %.1f%%。")
	}
}

func (c *Checker) checkMetric(ctx context.Context, serverID, serverName string, metric string, val, threshold float64, durationMin int, eventType, msgTpl string) {
	key := fmt.Sprintf("threshold:%s:%s", serverID, metric)
	firedKey := fmt.Sprintf("threshold:fired:%s:%s", serverID, metric)

	if val > threshold {
		fired, err := c.rds.Exists(ctx, firedKey).Result()
		if err == nil && fired > 0 {
			return
		}

		if durationMin <= 0 {
			c.fireAlert(ctx, serverID, serverName, eventType, threshold, val, msgTpl)
			_ = c.rds.Set(ctx, firedKey, "1", 1*time.Hour).Err()
			return
		}

		firstSeenStr, err := c.rds.Get(ctx, key).Result()
		if err == redis.Nil {
			nowStr := strconv.FormatInt(time.Now().Unix(), 10)
			_ = c.rds.Set(ctx, key, nowStr, 1*time.Hour).Err()
			return
		} else if err != nil {
			log.Printf("[threshold] redis error: %v", err)
			return
		}

		firstSeen, err := strconv.ParseInt(firstSeenStr, 10, 64)
		if err != nil {
			return
		}

		elapsed := time.Since(time.Unix(firstSeen, 0))
		if elapsed >= time.Duration(durationMin)*time.Minute {
			c.fireAlert(ctx, serverID, serverName, eventType, threshold, val, msgTpl)
			_ = c.rds.Set(ctx, firedKey, "1", 1*time.Hour).Err()
		}
	} else {
		_ = c.rds.Del(ctx, key, firedKey).Err()
	}
}

func (c *Checker) fireAlert(ctx context.Context, serverID, serverName, eventType string, threshold, currentVal float64, msgTpl string) {
	plainMsg := fmt.Sprintf(msgTpl, threshold, currentVal)

	detail := map[string]any{
		"value":     fmt.Sprintf("%.1f%%", currentVal),
		"threshold": fmt.Sprintf("%.1f%%", threshold),
	}

	ev, err := c.alertsStore.CreateAlert(ctx, serverID, eventType, "", "", detail, plainMsg, "high")
	if err != nil {
		log.Printf("[threshold] create alert error: %v", err)
		return
	}

	log.Printf("[threshold] %s threshold alert saved: id=%s, type=%s, msg=%s", serverID, ev.ID, eventType, plainMsg)

	if c.notify != nil {
		title := fmt.Sprintf("[%s] 系统告警: %s", serverName, eventType)
		c.notify.Send(ctx, title, plainMsg)
	}
}
