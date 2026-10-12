// Package apperr 定义统一的业务错误。
//
// 概念上等价于 Python 版的 ApiError：任何失败都被归一成
// {"code": "...", "message": "...", "detail": {...}}，前端只需处理一种形状。
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error 是可以直接序列化给客户端的业务错误。
type Error struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// New 构造一个业务错误。
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message, Detail: map[string]any{}}
}

// WithDetail 附加结构化细节，便于前端定位。
func (e *Error) WithDetail(detail map[string]any) *Error {
	if e.Detail == nil {
		e.Detail = map[string]any{}
	}
	for k, v := range detail {
		e.Detail[k] = v
	}
	return e
}

// 常用错误的快捷构造器。
func BadRequest(code, message string) *Error   { return New(http.StatusBadRequest, code, message) }
func Unauthorized(code, message string) *Error { return New(http.StatusUnauthorized, code, message) }
func Forbidden(code, message string) *Error    { return New(http.StatusForbidden, code, message) }
func NotFound(code, message string) *Error     { return New(http.StatusNotFound, code, message) }
func Conflict(code, message string) *Error     { return New(http.StatusConflict, code, message) }
func BadGateway(code, message string) *Error   { return New(http.StatusBadGateway, code, message) }
func Internal() *Error {
	return New(http.StatusInternalServerError, "internal_error", "服务器内部错误")
}

// Validation 表示参数校验失败。
func Validation(message string) *Error {
	return New(http.StatusUnprocessableEntity, "validation_error", message)
}

// As 从错误链中提取业务错误。
func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
