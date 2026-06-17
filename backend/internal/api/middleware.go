package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AccessTokenAuth 校验请求头 X-Access-Token 是否等于配置中的 AccessToken。
// 用 subtle.ConstantTimeCompare 防止时序攻击。
// 注意：agent 路由（/api/agent/*）不应套用此中间件，agent 走自己的 token。
func AccessTokenAuth(expected string) gin.HandlerFunc {
	expectedBytes := []byte(expected)
	return func(c *gin.Context) {
		got := c.GetHeader("X-Access-Token")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), expectedBytes) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "访问口令缺失或不正确",
			})
			return
		}
		c.Next()
	}
}
