package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// LimitBody 给请求体加一个硬上限，超出时读取会返回 *http.MaxBytesError。
//
// 只挂在 /api/v1 分组上（元数据与预签名请求都很小，1 MiB 绰绰有余），
// 探针等无请求体的路径不必包一层。真正把错误翻译成 413 的是 handler.bind：
// 中间件只负责限制可读字节数，不直接写响应，这样响应形状仍由 HTTP 层统一决定。
func LimitBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
