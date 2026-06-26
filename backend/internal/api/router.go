package api

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/agenthub"
	"github.com/guardian/backend/internal/api/agentapi"
	"github.com/guardian/backend/internal/api/handlers"
	"github.com/guardian/backend/internal/explain"
	"github.com/guardian/backend/internal/notify"
	"github.com/guardian/backend/internal/store"
	"github.com/guardian/backend/internal/threshold"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	AccessToken    string
	ServersStore   *store.Servers
	MetricsStore   *store.Metrics
	HardeningStore *store.Hardening
	AlertsStore    *store.Alerts
	ExplainService *explain.Service
	NotifyService  *notify.Service
	ConsoleBaseURL string
	Hub            *agenthub.Hub
	InventoryStore *store.InventoryStore
	Redis          *redis.Client
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies([]string{
		"127.0.0.1",
		"::1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// 开发期允许 5173（Vite dev）跨域；生产由 Caddy 同域服务，CORS 实际不会触发。
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "X-Access-Token"},
		MaxAge:       3600,
	}))

	apiGroup := r.Group("/api")
	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 解锁不走中间件：自身就是发口令的端点。
	apiGroup.POST("/unlock", postUnlock(deps.AccessToken))

	// 需要访问口令的端点都进 protected 组。
	protected := apiGroup.Group("/")
	protected.Use(AccessTokenAuth(deps.AccessToken))

	if deps.ServersStore != nil {
		sh := &handlers.ServersHandler{
			Store:          deps.ServersStore,
			Metrics:        deps.MetricsStore,
			Hardening:      deps.HardeningStore,
			Alerts:         deps.AlertsStore,
			ConsoleBaseURL: deps.ConsoleBaseURL,
			Hub:            deps.Hub,
		}
		protected.POST("/servers", sh.Create)
		protected.GET("/servers", sh.List)
		protected.GET("/servers/:id", sh.Get)
		protected.PUT("/servers/:id", sh.Update)
		protected.DELETE("/servers/:id", sh.Delete)

		// 公开的下载服务（不需要鉴权）
		r.GET("/install.sh", sh.DownloadInstallScript)
		apiGroup.GET("/agent/download", sh.DownloadAgent)
		apiGroup.GET("/agent/download/sha256", sh.DownloadAgentSHA256)
	}
	if deps.MetricsStore != nil {
		mh := &handlers.MetricsHandler{Store: deps.MetricsStore}
		protected.GET("/servers/:id/metrics", mh.List)
	}
	if deps.HardeningStore != nil {
		hh := &handlers.HardeningHandler{
			Store:   deps.HardeningStore,
			Servers: deps.ServersStore,
			Hub:     deps.Hub,
		}
		protected.GET("/servers/:id/hardening", hh.GetHardening)
		protected.POST("/servers/:id/hardening/:key/apply", hh.ApplyHardening)
		protected.POST("/servers/:id/hardening/:key/confirm", hh.ConfirmHardening)
		protected.POST("/servers/:id/hardening/:key/rollback", hh.RollbackHardening)
	}

	if deps.AlertsStore != nil && deps.NotifyService != nil {
		al := &handlers.AlertsHandler{
			Store:         deps.AlertsStore,
			Servers:       deps.ServersStore,
			NotifyService: deps.NotifyService,
		}
		protected.GET("/servers/:id/alerts", al.GetAlerts)
		protected.GET("/servers/:id/alerts/timeline", al.GetAlertsTimeline)
		protected.GET("/servers/:id/alerts/stats", al.GetAlertsStats)
		protected.GET("/settings/notifications", al.GetSettings)
		protected.PUT("/settings/notifications", al.UpdateSettings)
		protected.POST("/settings/notifications/test", al.TestNotification)
	}

	if deps.InventoryStore != nil {
		ih := &handlers.InventoryHandler{Store: deps.InventoryStore}
		protected.GET("/servers/:id/inventory", ih.GetInventory)
	}

	// Agent 协议路由（自带 agent token 校验，不进 access token 中间件）：
	if deps.Hub != nil && deps.ServersStore != nil {
		var checker *threshold.Checker
		if deps.Redis != nil && deps.AlertsStore != nil {
			checker = threshold.NewChecker(deps.Redis, deps.AlertsStore, deps.NotifyService)
		}

		ah := &agentapi.Handler{
			Servers:          deps.ServersStore,
			Metrics:          deps.MetricsStore,
			Hardening:        deps.HardeningStore,
			Hub:              deps.Hub,
			Alerts:           deps.AlertsStore,
			Explain:          deps.ExplainService,
			Notify:           deps.NotifyService,
			Inventory:        deps.InventoryStore,
			Redis:            deps.Redis,
			ThresholdChecker: checker,
		}
		apiGroup.POST("/agent/enroll", ah.Enroll)
		apiGroup.GET("/agent/ws", ah.WebSocket)
	}

	return r
}

type unlockReq struct {
	AccessToken string `json:"accessToken"`
}

func postUnlock(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req unlockReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request"})
			return
		}
		if req.AccessToken != expected {
			c.JSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "访问口令不正确",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "token": expected})
	}
}
