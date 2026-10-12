package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// logWarn 记录不影响本次响应的次要错误（例如对象清理失败）。
func (h *Handler) logWarn(message string, err error) {
	slog.Warn(message, "err", err)
}

// RequestLogger 用 slog 记录访问日志，格式与项目其余日志保持一致。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
			slog.Error("请求处理出错", attrs...)
			return
		}
		slog.Info("请求", attrs...)
	}
}
