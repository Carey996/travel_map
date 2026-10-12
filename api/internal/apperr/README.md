# `api/internal/apperr` · 统一错误

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`apperr`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `apperr.go` | 可序列化给客户端的业务错误 | `Error`, `New()`, `As()` 及一组快捷构造器 |

任何失败都归一成同一形状，前端只需处理一种结构：

`F`json
{ "code": "photo_not_found", "message": "照片不存在", "detail": {} }
`F`

| 构造器 | HTTP 状态 |
|---|---|
| `BadRequest(code, msg)` | 400 |
| `Unauthorized(code, msg)` | 401 |
| `Forbidden(code, msg)` | 403 |
| `NotFound(code, msg)` | 404 |
| `Conflict(code, msg)` | 409 |
| `Validation(msg)` | 422（固定 code 为 `validation_error`） |
| `BadGateway(code, msg)` | 502 |
| `Internal()` | 500（固定 code 为 `internal_error`） |

## 约定

**handler 层统一用 `(*Handler).fail(c, err)` 输出错误。** 它会先用 `As()` 尝试
提取业务错误，提取不到就记日志并按 500 返回——这样既不会把内部错误细节泄露给客户端，
也不会让意料之外的错误静默消失。

**错误码用 snake_case 英文**，是给程序判断用的；`message` 才是给人看的中文。

**`WithDetail()` 用于附加结构化上下文**（例如 `invalid_bbox` 会带上原始值），
注意不要往里塞敏感信息。

## 相关

- 上级：[`internal/`](../README.md)
- 使用方：[`handler/`](../handler/README.md)
