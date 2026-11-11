# `api/internal/handler` · HTTP 层

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`handler`**

把 HTTP 请求翻译成服务层调用所需的一切。**这里不写业务规则**，
规则在 [`../service/`](../service/README.md) 与各服务包。

| 文件 | 职责 |
|---|---|
| `handler.go` | 依赖容器 `Handler`、统一错误输出 `fail()`、请求校验 `bind()`（含 413 映射）、鉴权 `currentUser()`、Cookie 读写 |
| `router.go` | 路由与中间件装配（含 `/api/v1` 的 1 MiB 请求体上限） |
| `query.go` | 查询参数解析（`parseBBox` / `queryInt` / `queryBool` / `queryDate`） |
| `health.go` | 存活与就绪探针 |
| `auth.go` | 注册 / 登录 / 登出 / 登出所有设备 / 当前用户；密码 72 字节边界 |
| `place.go` | 地点 CRUD、照片归属、地点合并；`trip_id` 归属校验 |
| `photo.go` | 预签名签发、元数据登记、查询、更新、删除；对象键归属校验 |
| `note.go` | 笔记 CRUD |
| `trip.go` | 行程 CRUD、地点批量加入 / 移出、行程维度的地点与照片计数 |
| `geo.go` | 地名代理接口 |
| `logging.go` | slog 访问日志 |
| `auth_test.go` | 边界单测：bcrypt 72 字节、请求体上限 413、slug 唯一性 |
| `photo_test.go` | 纯函数单测：`validStorageKey()` 前缀 / 路径穿越 / 反斜杠，`validContentHash()` 十六进制边界 |

## Handler 结构体

`Handler` 持有全部依赖（配置、GORM、Redis、会话存储、对象存储、地名服务），
在 `cmd/server` 里一次性装配。它们是进程级基础设施，不是业务状态。

## 三条必须遵守的约定

**1. 每个数据库查询都要带 `user_id` 过滤。** 这是多租户隔离的唯一防线。
`ownedPlace()` / `ownedPhoto()` / `ownedNote()` 这几个小助手就是为了避免漏写。

请求体里的外键同样不可信任：`place_id` / `trip_id` 来自客户端，凡是「把记录挂到某个
地点 / 行程下」的写路径，都必须先证明目标属于当前用户（见下节）。

**2. 错误一律走 `h.fail(c, err)`。** 它先尝试提取 `apperr.Error`，
提取不到就记日志并返回 500——既不泄露内部细节，也不让意外错误静默消失。

**3. 照片归属回写必须无条件执行。** 见下方说明。

## 客户端提交的归属必须校验

「带 `user_id` 的查询」只解决了「读别人的数据」，还有一类漏洞是「把数据写到别人名下」——
请求体里的 UUID 与对象键全部由客户端提供，必须逐项确认归属。五个落点：

| 入口 | 不可信字段 | 校验 | 失败响应 |
|---|---|---|---|
| `POST /photos` | `place_id` | `requireOwnedPlace()`：`First(&place, "id = ? AND user_id = ?")` | 404 `place_not_found` |
| `POST /photos` | `storage_key` / `preview_key` / `thumb_key` | `validStorageKey(user.ID, key)`：必须以 `u/<当前用户 UUID>/` 开头、不含 `..` / 反斜杠 / NUL | 400 `invalid_storage_key`（`detail.field` 指出是哪个键） |
| `PATCH /photos/:photo_id` | `place_id` | 同上（`clear_place` 走置空，不需要校验） | 404 `place_not_found` |
| `PATCH /places/:id` | `trip_id` | `First(&trip, "id = ? AND user_id = ?")`（`clear_trip` 走置空） | 404 `trip_not_found` |
| `POST /photos/upload-url` | `content_hash` | `validContentHash()`：8–64 位十六进制 | 400 `invalid_content_hash` |

**为什么对象键也要校验。** 上传 URL 由服务端签发，键格式固定为
`u/<user.UUID>/<content_hash><ext>`；登记照片时浏览器会把键原样传回。
若不校验前缀，用户 A 只要提交用户 B 的键，响应里的三个预签名直链就会指向 B 的照片——
对象存储层的租户隔离形同虚设。`content_hash` 会被直接拼进键，因此
`CreateUploadURL` 先把它收紧成十六进制，避免借 `/`、`..` 让服务端为任意键签发 PUT。

**写路径上的兜底。** 批量操作（`DeletePlace` 的 `delete_photos` 分支、解绑照片、
`MergePlace`）都显式带 `user_id = ?`；`service.AssignPhoto()` 内部还会用
`id = ? AND user_id = ?` 再查一次地点，查不到返回 `apperr.NotFound` 而不是静默放过。

## 请求边界与错误映射

| 场景 | 状态码 / code | 落点 |
|---|---|---|
| 请求体超过 1 MiB | 413 / `payload_too_large` | `bind()` 识别 `*http.MaxBytesError`，`detail.limit_bytes` 给出上限 |
| JSON 缺字段 / 类型错 / 字段超长 | 422 / `validation_error` | `bind()`，`detail.errors` 保留原始原因 |
| 注册密码 > 72 字节 | 400 / `validation_error` | `passwordTooLong()`，**在 bcrypt 之前**拦 |
| 登录密码 > 72 字节 | 401 / `invalid_credentials` | 仍做一次 dummy bcrypt 比对，不让超长输入变成快路径 |

要点：

- **bcrypt 只接受 ≤ 72 字节**（`golang.org/x/crypto/bcrypt` 的硬上限），超过时
  `GenerateFromPassword` 直接返回错误。`dto.RegisterRequest` 里的 `max=200` 按**字符数**校验，
  管不住 73+ 字节的 UTF-8 密码，因此 `auth.go` 的字节级检查才是唯一防线。
- 上限挂在 `/api/v1` 分组（`router.go`），由 `middleware.LimitBody(1<<20)` 实现；
  探针不在版本化分组下，不受影响。后端不接收图片字节流，1 MiB 足够所有业务 JSON。

## `currentUser` 的错误边界

只把 `gorm.ErrRecordNotFound`（会话指向的用户已不存在）当 **401 `session_expired`**；
其余数据库错误一律 `h.fail(c, err)` → **500**。

早期实现把任何 DB 错误都映射成 401，后果是数据库抖动会把全站在线用户踢下线，
同时把真实故障伪装成普通的「登录过期」，日志里只看到 401、看不到 500。

## ⚠️ 归属回写那个坑

`CreatePhoto` / `UpdatePhoto` 里有一段：

```go
// AssignPhoto 只改内存里的 photo.PlaceID，而这一行是插入之后才确定的，
// 因此必须显式回写一次。
if place != nil {
    tx.Model(&model.Photo{}).Where("id = ? AND user_id = ?", photo.ID, user.ID).
        Update("place_id", place.ID)
}
```

早期版本的条件是「值不同才更新」，而 `service.AssignPhoto()` 恰好已经把它设成了
同一个值——于是**唯一需要写库的那次被跳过**。症状很有迷惑性：
聚类看着是对的（地点确实建出来了、日期范围也继承了），
但照片在地图上永远挂不到地点下，`photo_count` 恒为 0。

## 各资源处理器的要点

**auth**：登录时无论邮箱是否存在都做一次哈希比对，避免通过响应时间枚举账号；
邮箱统一小写存储。密码超过 bcrypt 的 72 字节上限时：注册 400、登录 401，
两条路径都不会进 bcrypt（登录仍付一次 dummy 比对成本），更不会变成 500。

**health**：就绪探针每项 `checks` 只返回 `"ok"` / `"error"`，真实错误用
`slog.Warn` 记录在服务端。探针面向未认证请求，错误原文里常带主机名、端口、
bucket 名等内部信息，不能回显；HTTP 200/503 与 `status` 的 ok/degraded 语义不变。

**photo**：`upload-url` 与 `photo` 登记都做幂等短路（按 `content_hash`）；
无 GPS 的照片不归属任何地点，落进前端的「未定位」托盘。
登记时 `place_id` 与三个对象键都要过归属校验（见上文「客户端提交的归属必须校验」）；
`PATCH` 改归属时重新指定 `place_id` 同样校验——否则 PATCH 就成了绕过 `POST` 校验的后门。

**place**：删除时默认保留照片（`place_id` 置空），
只有显式传 `delete_photos=true` 才连带删对象存储里的文件；
两个分支都按 `place_id = ? AND user_id = ?` 收窄，批量删对象前先取回该用户的照片。
`PATCH /places/:id` 的 `trip_id` 必须指向自己的行程。

**geo**：上游失败转成 502，而不是把第三方异常直接透给客户端。

**trip**：`slug` 由 **完整 uuid（32 位 hex，`trip-` 前缀共 37 字符）** 派生（中文标题没法
直接 slug 化）且不对外暴露。早期只取 uuid 前 12 位——UUIDv7 的高位是毫秒时间戳，
同一天创建的行程随机性只剩 2 位，同毫秒建两个旅行就会撞 `slug` 唯一索引报 500。
删除行程只解除 `places.trip_id`，地点与照片都保留——行程是**视图**，不是容器。
两个计数（地点数、照片数）在同一个请求里用两条 group by 查出，不做 N+1。

## 相关

- 上级：[`internal/`](../README.md)
- 装配：[`router.go`](router.go) · [`cmd/server/`](../../cmd/server/README.md)
- 契约：[`dto/`](../dto/README.md)
