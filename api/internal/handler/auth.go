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

// maxPasswordBytes 是 bcrypt 的硬上限：超过 72 字节时
// GenerateFromPassword / CompareHashAndPassword 会直接返回错误而不是比对，
// 因此必须在进入 bcrypt 之前拦掉，否则注册路径会变成 500。
const maxPasswordBytes = 72

// passwordTooLong 报告密码是否超过 bcrypt 的 72 字节上限。
//
// 按「字节」而非「字符」计算：一个汉字占 3 字节，72 字节约等于 24 个汉字。
// 抽成纯函数便于单元测试，也避免各处各写一套判断。
func passwordTooLong(p string) bool {
	return len([]byte(p)) > maxPasswordBytes
}

// Register 注册并自动登录。
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if !h.bind(c, &req) {
		return
	}
	// dto 里 max=200 只按字符数校验，拦不住 73+ 字节的 UTF-8 密码；
	// bcrypt 会拒绝这种输入，若继续往下走就会变成 500 而不是 400。
	if passwordTooLong(req.Password) {
		h.fail(c, apperr.BadRequest("validation_error",
			"密码过长（最多 72 字节，约 24 个汉字）"))
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
	// 超长密码与任何已存哈希都不可能匹配（注册路径同样会拒绝）。
	// 这里仍然付一次 bcrypt 比对的成本（用截断到上限的密码喂给假哈希），
	// 否则「密码超长」会比「密码错误」快一个数量级，形成可观测的时序差异。
	if passwordTooLong(req.Password) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(req.Password)[:maxPasswordBytes])
		h.fail(c, apperr.Unauthorized("invalid_credentials", "邮箱或密码不正确"))
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
