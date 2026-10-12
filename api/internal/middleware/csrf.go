package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var safeMethods = map[string]struct{}{
	http.MethodGet: {}, http.MethodHead: {}, http.MethodOptions: {}, http.MethodTrace: {},
}

// CSRF 要求变更类请求携带约定的自定义头。
//
// 选用 Cookie 承载会话（比 localStorage 抗 XSS）就必然引入 CSRF 面，
// 因此做三层防护：Cookie 的 SameSite=Lax、CORS 严格白名单，以及这里的自定义头校验——
// HTML 表单无法设置自定义头，跨站 fetch 带自定义头又会触发预检并被白名单拒绝。
func CSRF(headerName, headerValue string, exemptPaths map[string]struct{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, safe := safeMethods[c.Request.Method]; safe {
			c.Next()
			return
		}
		if _, exempt := exemptPaths[c.Request.URL.Path]; exempt {
			c.Next()
			return
		}
		if !strings.EqualFold(c.GetHeader(headerName), headerValue) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "csrf_header_missing",
				"message": "变更类请求必须携带头部 " + headerName + ": " + headerValue,
				"detail":  gin.H{"path": c.Request.URL.Path, "method": c.Request.Method},
			})
			return
		}
		c.Next()
	}
}
