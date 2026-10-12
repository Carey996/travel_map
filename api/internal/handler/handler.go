// Package handler 是 HTTP 层：解析请求 -> 调用服务 -> 返回契约模型。
//
// 业务规则不写在这里——它们在 internal/service 与各服务包里。
package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/Carey996/travel_map/api/internal/apperr"
	"github.com/Carey996/travel_map/api/internal/config"
	"github.com/Carey996/travel_map/api/internal/geocode"
	"github.com/Carey996/travel_map/api/internal/model"
	"github.com/Carey996/travel_map/api/internal/session"
	"github.com/Carey996/travel_map/api/internal/storage"
)

// Handler 持有全部依赖。它们是进程级基础设施（连接池、客户端），不是业务状态。
type Handler struct {
	Cfg      *config.Config
	DB       *gorm.DB
	Redis    *redis.Client
	Sessions session.Store
	Storage  *storage.Storage
	Geocoder *geocode.Geocoder
}

// New 构造 Handler。
func New(cfg *config.Config, gdb *gorm.DB, rdb *redis.Client, store session.Store, st *storage.Storage, gc *geocode.Geocoder) *Handler {
	return &Handler{Cfg: cfg, DB: gdb, Redis: rdb, Sessions: store, Storage: st, Geocoder: gc}
}

// ctx 返回请求上下文，便于在辅助函数里保持简洁。
func ctx(c *gin.Context) context.Context { return c.Request.Context() }

// ---------------------------------------------------------------------------
// 错误处理
// ---------------------------------------------------------------------------

// fail 把错误归一成统一结构后写出响应。
//
// 与 Python 版一致：前端只需要处理 {"code", "message", "detail"} 这一种形状。
func (h *Handler) fail(c *gin.Context, err error) {
	if apiErr, ok := apperr.As(err); ok {
		c.JSON(apiErr.Status, apiErr)
		return
	}
	slog.Error("未处理的错误", "path", c.Request.URL.Path, "err", err)
	c.JSON(http.StatusInternalServerError, apperr.Internal())
}

// bind 解析并校验请求体；失败时直接写出 422 并返回 false。
func (h *Handler) bind(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
			"code":    "validation_error",
			"message": "请求参数校验失败",
			"detail":  gin.H{"errors": err.Error()},
		})
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// 鉴权
// ---------------------------------------------------------------------------

// currentUser 从 Cookie 解析会话并载入用户；未登录时写出 401 并返回 nil。
func (h *Handler) currentUser(c *gin.Context) *model.User {
	sid, err := c.Cookie(h.Cfg.SessionCookieName)
	if err != nil || sid == "" {
		h.fail(c, apperr.Unauthorized("not_authenticated", "请先登录"))
		return nil
	}

	userID, ok, err := h.Sessions.Resolve(ctx(c), sid)
	if err != nil {
		h.fail(c, err)
		return nil
	}
	if !ok {
		h.fail(c, apperr.Unauthorized("session_expired", "登录已失效，请重新登录"))
		return nil
	}

	var user model.User
	if err := h.DB.WithContext(ctx(c)).First(&user, "id = ?", userID).Error; err != nil {
		h.fail(c, apperr.Unauthorized("session_expired", "登录已失效，请重新登录"))
		return nil
	}
	if !user.IsActive {
		h.fail(c, apperr.Unauthorized("account_inactive", "账号不可用"))
		return nil
	}
	return &user
}

// userID 解析路径参数中的 UUID。
func (h *Handler) pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		h.fail(c, apperr.BadRequest("invalid_id", "路径参数 "+name+" 不是合法的 UUID"))
		return uuid.Nil, false
	}
	return id, true
}

// setSessionCookie 写入会话 Cookie。
func (h *Handler) setSessionCookie(c *gin.Context, sid string) {
	secure := h.Cfg.CookieSecure
	sameSite := parseSameSite(h.Cfg.CookieSameSite)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.Cfg.SessionCookieName,
		Value:    sid,
		Path:     "/",
		MaxAge:   int(h.Cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// clearSessionCookie 删除会话 Cookie。
func (h *Handler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.Cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.Cfg.CookieSecure,
		SameSite: parseSameSite(h.Cfg.CookieSameSite),
	})
}

func parseSameSite(value string) http.SameSite {
	switch value {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
