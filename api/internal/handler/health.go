package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
func (h *Handler) Readiness(c *gin.Context) {
	checks := map[string]string{}

	if sqlDB, err := h.DB.DB(); err != nil {
		checks["database"] = describe(err)
	} else if err := sqlDB.PingContext(ctx(c)); err != nil {
		checks["database"] = describe(err)
	} else {
		checks["database"] = "ok"
	}

	if err := h.Redis.Ping(ctx(c)).Err(); err != nil {
		checks["redis"] = describe(err)
	} else {
		checks["redis"] = "ok"
	}

	if err := h.Storage.EnsureBucket(ctx(c)); err != nil {
		checks["storage"] = describe(err)
	} else {
		checks["storage"] = "ok"
	}

	status := http.StatusOK
	overall := "ok"
	for _, v := range checks {
		if v != "ok" {
			status = http.StatusServiceUnavailable
			overall = "degraded"
			break
		}
	}
	c.JSON(status, gin.H{"status": overall, "checks": checks})
}

func describe(err error) string {
	return "error: " + err.Error()
}
