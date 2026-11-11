package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Carey996/travel_map/api/internal/model"
)

// RegisterRequest 注册请求。
//
// 注意：密码的 max=200 是按「字符数」校验的，管不住 bcrypt 的 72 字节硬上限
// （24 个汉字就已 72 字节）。真正的字节级校验在 handler.passwordTooLong()，
// 这里的 binding 只作为浅层防线，不要因为改了它而放松下面的检查。
type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email,max=320"`
	Password    string `json:"password" binding:"required,min=8,max=200"`
	DisplayName string `json:"display_name" binding:"max=80"`
}

// LoginRequest 登录请求。
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=320"`
	Password string `json:"password" binding:"required,min=1,max=200"`
}

// UserOut 是用户对外的表示；刻意不含 password_hash。
type UserOut struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewUserOut 由模型构造响应。
func NewUserOut(u *model.User) UserOut {
	return UserOut{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, CreatedAt: u.CreatedAt}
}
