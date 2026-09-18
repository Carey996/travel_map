# `api/app/schemas` · 接口契约

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`app`](../README.md) › **`schemas`**

Pydantic v2 模型，定义 HTTP 层的输入输出形状。**这一层不碰数据库**——
它决定「接口长什么样」，而 ORM 决定「数据怎么存」。

## 文件

| 文件 | 请求模型 | 响应模型 |
|---|---|---|
| `auth.py` | `RegisterRequest`, `LoginRequest` | `UserOut` |
| `place.py` | `PlaceCreate`, `PlaceUpdate`, `AssignPhotosRequest`, `MergePlaceRequest` | `PlaceOut`, `PlaceDetailOut` |
| `photo.py` | `UploadUrlRequest`, `UploadVariant`, `PhotoCreate`, `PhotoUpdate` | `SignedUpload`, `UploadUrlResponse`, `PhotoOut` |
| `note.py` | `NoteCreate`, `NoteUpdate` | `NoteOut` |
| `geo.py` | —— | `ReverseResult`, `SearchResult` |

## 约定

**`*Out` 只暴露该给前端的字段。** 例如 `UserOut` 没有 `password_hash`，
`PhotoOut` 不返回 `storage_key` 而是返回签名后的直链。

**照片响应必须经 `photo_to_out()` 生成。** 这是唯一把所有对象键换成预签名 URL 的地方；
直接返回 ORM 对象会泄露存储键、且前端拿不到可用的图片地址。

**校验规则写在模型上**（如 `password` 的 `min_length=8`、经纬度的范围约束），
不要在路由里手写 if。校验失败由 `core/errors.py` 统一转成 422。

**日期字段统一用 `date` / `datetime`**，序列化为 ISO 8601 字符串。

## 相关

- 上级：[`app/`](../README.md)
- 同级：[`core/`](../core/README.md) · [`db/`](../db/README.md) · [`services/`](../services/README.md) · [`api/`](../api/README.md)
