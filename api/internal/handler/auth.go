package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Carey996/travel_map/api/internal/apperr"
	"github.com/Carey996/travel_map/api/internal/dto"
	"github.com/Carey996/travel_map/api/internal/model"
)

// 用于「邮箱不存在时也做一次哈希比对」的假哈希，避免通过响应时间枚举账号。
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Register 注册并自动登录。
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if !h.bind(c, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	var existing model.User
	err := h.DB.WithContext(ctx(c)).First(&existing, "email = ?", email).Error
	if err == nil {
		h.fail(c, apperr.Conflict("email_taken", "该邮箱已被注册"))
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		h.fail(c, err)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.fail(c, err)
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		if at := strings.IndexByte(email, '@'); at > 0 {
			displayName = email[:at]
		} else {
			displayName = email
		}
	}

	user := model.User{Email: email, DisplayName: displayName, PasswordHash: string(hashed), IsActive: true}
	if err := h.DB.WithContext(ctx(c)).Create(&user).Error; err != nil {
		h.fail(c, err)
		return
	}

	if err := h.issueSession(c, user.ID); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewUserOut(&user))
}

// Login 校验凭据并建立会话。
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !h.bind(c, &req) {
		return
	}
	email := strings.ToLower(req.Email)

	var user model.User
	err := h.DB.WithContext(ctx(c)).First(&user, "email = ?", email).Error

	// 无论邮箱是否存在都做一次哈希比对，避免通过响应时间枚举账号
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(req.Password))
		h.fail(c, apperr.Unauthorized("invalid_credentials", "邮箱或密码不正确"))
		return
	}
	if err != nil {
		h.fail(c, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		h.fail(c, apperr.Unauthorized("invalid_credentials", "邮箱或密码不正确"))
		return
	}
	if !user.IsActive {
		h.fail(c, apperr.Forbidden("account_inactive", "账号已被停用"))
		return
	}

	if err := h.issueSession(c, user.ID); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewUserOut(&user))
}

// Logout 登出当前设备。
func (h *Handler) Logout(c *gin.Context) {
	if sid, err := c.Cookie(h.Cfg.SessionCookieName); err == nil && sid != "" {
		if err := h.Sessions.Revoke(ctx(c), sid); err != nil {
			h.fail(c, err)
			return
		}
	}
	h.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// LogoutAll 登出所有设备。
func (h *Handler) LogoutAll(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	if err := h.Sessions.RevokeAllForUser(ctx(c), user.ID); err != nil {
		h.fail(c, err)
		return
	}
	h.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// Me 返回当前登录用户。
func (h *Handler) Me(c *gin.Context) {
	user := h.currentUser(c)
	if user == nil {
		return
	}
	c.JSON(http.StatusOK, dto.NewUserOut(user))
}

func (h *Handler) issueSession(c *gin.Context, userID uuid.UUID) error {
	sid, err := h.Sessions.Create(ctx(c), userID, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		return err
	}
	h.setSessionCookie(c, sid)
	return nil
}
