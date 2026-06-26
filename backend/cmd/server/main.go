package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/guardian/backend/internal/agenthub"
	"github.com/guardian/backend/internal/api"
	"github.com/guardian/backend/internal/config"
	"github.com/guardian/backend/internal/db"
	"github.com/guardian/backend/internal/explain"
	"github.com/guardian/backend/internal/notify"
	"github.com/guardian/backend/internal/redisx"
	"github.com/guardian/backend/internal/store"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[guardian] postgres connect: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(pool, cfg.DatabaseURL); err != nil {
		log.Fatalf("[guardian] migrate: %v", err)
	}
	log.Printf("[guardian] migrations ok")

	rds, err := redisx.New(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("[guardian] redis connect: %v", err)
	}
	defer rds.Close()

	servers := store.NewServers(pool)
	metrics := store.NewMetrics(pool)
	hardening := store.NewHardening(pool)
	alerts := store.NewAlerts(pool)

	explainService := explain.New(rds, cfg.ClaudeAPIKey, cfg.DeepSeekAPIKey)
	notifyService := notify.New(alerts, notify.SMTPConfig{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.SMTPFrom,
	})

	hub := agenthub.New(rds, servers, alerts, explainService, notifyService)
	go hub.StartSweeper(ctx)
	go runMetricsCleanup(ctx, metrics)
	go runDeadmanSwitchSweeper(ctx, hardening, hub)

	router := api.NewRouter(api.Deps{
		AccessToken:    cfg.AccessToken,
		ServersStore:   servers,
		MetricsStore:   metrics,
		HardeningStore: hardening,
		AlertsStore:    alerts,
		ExplainService: explainService,
		NotifyService:  notifyService,
		ConsoleBaseURL: cfg.ConsoleBaseURL,
		Hub:            hub,
	})
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("[guardian] listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[guardian] http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("[guardian] shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// runMetricsCleanup 每小时清理 24h 前的指标点；启动时先跑一次。
func runMetricsCleanup(ctx context.Context, m *store.Metrics) {
	clean := func() {
		cutoff := time.Now().UTC().Add(-24 * time.Hour)
		n, err := m.DeleteOlderThan(ctx, cutoff)
		if err != nil {
			log.Printf("[guardian] metrics cleanup: %v", err)
			return
		}
		if n > 0 {
			log.Printf("[guardian] metrics cleanup: removed %d rows older than %s", n, cutoff.Format(time.RFC3339))
		}
	}
	clean()
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			clean()
		}
	}
}

// runDeadmanSwitchSweeper 每 5 秒扫描一次超时 trial 的加固任务并自动下发回滚命令。
func runDeadmanSwitchSweeper(ctx context.Context, s *store.Hardening, hub *agenthub.Hub) {
	sweep := func() {
		// 1. 定期清理已经超时 (超过5分钟) 仍为 pending 的任务状态，防止前端 spinner 永远加载
		if n, err := s.CleanTimeoutPendingJobs(ctx); err == nil && n > 0 {
			log.Printf("[deadman] cleaned up %d timeout pending hardening jobs", n)
		} else if err != nil {
			log.Printf("[deadman] CleanTimeoutPendingJobs error: %v", err)
		}

		// 2. 扫描处理超时的 trial 任务并触发自主回滚
		jobs, err := s.GetTimeoutTrialJobs(ctx)
		if err != nil {
			log.Printf("[deadman] scan timeout: %v", err)
			return
		}
		for _, j := range jobs {
			log.Printf("[deadman] automatically rolling back job %s (server: %s, key: %s) due to confirm timeout", j.ID, j.ServerID, j.ItemKey)
			
			// 数据库置为 rolledback
			if err := s.UpdateJobStatus(ctx, j.ID, "rolledback", nil); err != nil {
				log.Printf("[deadman] UpdateJobStatus %s to rolledback error: %v", j.ID, err)
			}

			// 尝试下发 WSS 命令给 Agent
			var files map[string]any
			if len(j.SnapshotFiles) > 0 {
				_ = json.Unmarshal(j.SnapshotFiles, &files)
			}

			cmdMsg := map[string]any{
				"type": "command",
				"payload": map[string]any{
					"cmd":   "rollback",
					"jobId": j.ID,
					"key":   j.ItemKey,
					"files": files,
				},
			}
			if err := hub.CommandTo(j.ServerID, cmdMsg); err != nil {
				log.Printf("[deadman] automatically rollback CommandTo %s error: %v", j.ServerID, err)
			}
		}
	}

	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweep()
		}
	}
}
