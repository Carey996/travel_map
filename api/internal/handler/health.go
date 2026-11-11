package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 检查结果的对外取值。探针不需要、也不应该回显底层错误原文——
// 它们面向未认证请求，错误串里常带主机名、端口、bucket 名等内部信息。
const (
	checkOK    = "ok"
	checkError = "error"
)

// Liveness 存活探针：不接触依赖，用于判断进程是否需要重启。
func (h *Handler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"app":         h.Cfg.AppName,
		"version":     h.Cfg.AppVersion,
		"environment": h.Cfg.Environment,
	})
}

// Readiness 就绪探针：检查数据库、Redis 与对象存储的连通性。
//
// 每项只返回 "ok" / "error"，真实错误用 slog.Warn 记录在服务端日志里。
// HTTP 状态与整体 status 语义保持不变：全 ok -> 200 + "ok"，
// 任一失败 -> 503 + "degraded"。
func (h *Handler) Readiness(c *gin.Context) {
	checks := map[string]string{}

	if sqlDB, err := h.DB.DB(); err != nil {
		checks["database"] = checkError
		slog.Warn("就绪探针：取数据库连接失败", "err", err)
	} else if err := sqlDB.PingContext(ctx(c)); err != nil {
		checks["database"] = checkError
		slog.Warn("就绪探针：数据库 ping 失败", "err", err)
	} else {
		checks["database"] = checkOK
	}

	if err := h.Redis.Ping(ctx(c)).Err(); err != nil {
		checks["redis"] = checkError
		slog.Warn("就绪探针：Redis ping 失败", "err", err)
	} else {
		checks["redis"] = checkOK
	}

	if err := h.Storage.EnsureBucket(ctx(c)); err != nil {
		checks["storage"] = checkError
		slog.Warn("就绪探针：对象存储不可用", "err", err)
	} else {
		checks["storage"] = checkOK
	}

	status := http.StatusOK
	overall := checkOK
	for _, v := range checks {
		if v != checkOK {
			status = http.StatusServiceUnavailable
			overall = "degraded"
			break
		}
	}
	c.JSON(status, gin.H{"status": overall, "checks": checks})
}
