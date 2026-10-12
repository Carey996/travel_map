package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Carey996/travel_map/api/internal/model"
)

// RegisterRequest 注册请求。
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
