// Package middleware 提供跨域与 CSRF 防护。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 实现带凭据的跨域白名单。
//
// 与 FastAPI 的 CORSMiddleware 行为对齐：白名单之外的来源拿不到
// Access-Control-Allow-Origin，浏览器会自行拦截，服务端不必额外报错。
func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		_, ok := allowed[origin]

		if origin != "" && ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			if ok {
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				if requested := c.GetHeader("Access-Control-Request-Headers"); requested != "" {
					c.Header("Access-Control-Allow-Headers", requested)
				}
				c.Header("Access-Control-Max-Age", "600")
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
