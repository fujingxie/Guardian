package api

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/guardian/backend/internal/agenthub"
	"github.com/guardian/backend/internal/api/agentapi"
	"github.com/guardian/backend/internal/api/handlers"
	"github.com/guardian/backend/internal/store"
)

type Deps struct {
	AccessToken    string
	ServersStore   *store.Servers
	ConsoleBaseURL string
	Hub            *agenthub.Hub
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.New()
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
			ConsoleBaseURL: deps.ConsoleBaseURL,
		}
		protected.POST("/servers", sh.Create)
		protected.GET("/servers", sh.List)
		protected.GET("/servers/:id", sh.Get)
	}

	// Agent 协议路由（自带 agent token 校验，不进 access token 中间件）：
	if deps.Hub != nil && deps.ServersStore != nil {
		ah := &agentapi.Handler{Servers: deps.ServersStore, Hub: deps.Hub}
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
