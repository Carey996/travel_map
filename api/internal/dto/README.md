# `api/internal/dto` · 接口契约

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`dto`**

请求校验与响应序列化。**这一层不碰数据库**：它决定「接口长什么样」，
而 `model` 决定「数据怎么存」。

| 文件 | 请求模型 | 响应模型 |
|---|---|---|
| `auth.go` | `RegisterRequest`, `LoginRequest` | `UserOut` |
| `place.go` | `PlaceCreate`, `PlaceUpdate`, `AssignPhotosRequest`, `MergePlaceRequest` | `PlaceOut`, `PlaceDetailOut` |
| `photo.go` | `UploadURLRequest`, `UploadVariant`, `PhotoCreate`, `PhotoUpdate` | `SignedUpload`, `UploadURLResponse`, `PhotoOut` |
| `note.go` | `NoteCreate`, `NoteUpdate` | `NoteOut` |
| `date.go` | —— | `DateOnly` |

## 校验写在结构体标签上

`F`go
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=320"`
	Password string `json:"password" binding:"required,min=8,max=200"`
}
`F`

由 Gin 内置的 validator 执行，失败时 `handler.bind()` 统一转成 422。
**不要在处理器里手写 if 校验**——那样错误码与消息格式会各写各的。

## 更新类请求一律用指针

`PlaceUpdate` / `PhotoUpdate` / `NoteUpdate` 的字段都是 `*T`，
用来区分「没传这个字段」与「传了零值」。用值类型的话，
把 `rating` 改成 0 和「不改 rating」在 JSON 上无法区分。

> 已知取舍：`date_from: null` 与「未传」目前都表现为 nil，无法显式清空日期。
> 前端当前不发这两个字段，等有需求时再加 `clear_date` 之类的显式标志。

## `PhotoOut` 必须经 `NewPhotoOut()` 生成

这是**唯一**把对象键换成预签名直链的地方。直接返回 `model.Photo` 会泄露存储键、
且前端拿不到可用的图片地址。签名是纯本地计算（不发网络请求），所以每个请求现签即可，
既不必在库里存易过期的 URL，也不需要任何缓存。

它通过接受一个 `URLSigner` 接口而不是具体的存储实现，避免 `dto` 反向依赖 `storage`。

## DateOnly 为什么存在

两端日期口径不同：浏览器的 `el-date-picker` 提交 `2025-04-01`，
而 Go 的 `time.Time` 默认只认 RFC3339。`DateOnly` 两种都接受，
输出统一为 `2006-01-02` 或 `null`。

## 相关

- 上级：[`internal/`](../README.md)
- 使用方：[`handler/`](../handler/README.md) · 数据来源：[`model/`](../model/README.md)
