# `api/internal/handler` · HTTP 层

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`handler`**

把 HTTP 请求翻译成服务层调用所需的一切。**这里不写业务规则**，
规则在 [`../service/`](../service/README.md) 与各服务包。

| 文件 | 职责 |
|---|---|
| `handler.go` | 依赖容器 `Handler`、统一错误输出 `fail()`、请求校验 `bind()`、鉴权 `currentUser()`、Cookie 读写 |
| `router.go` | 路由与中间件装配 |
| `query.go` | 查询参数解析（`parseBBox` / `queryInt` / `queryBool` / `queryDate`） |
| `health.go` | 存活与就绪探针 |
| `auth.go` | 注册 / 登录 / 登出 / 登出所有设备 / 当前用户 |
| `place.go` | 地点 CRUD、照片归属、地点合并 |
| `photo.go` | 预签名签发、元数据登记、查询、更新、删除 |
| `note.go` | 笔记 CRUD |
| `geo.go` | 地名代理接口 |
| `logging.go` | slog 访问日志 |

## Handler 结构体

`Handler` 持有全部依赖（配置、GORM、Redis、会话存储、对象存储、地名服务），
在 `cmd/server` 里一次性装配。它们是进程级基础设施，不是业务状态。

## 三条必须遵守的约定

**1. 每个数据库查询都要带 `user_id` 过滤。** 这是多租户隔离的唯一防线。
`ownedPlace()` / `ownedPhoto()` / `ownedNote()` 这几个小助手就是为了避免漏写。

**2. 错误一律走 `h.fail(c, err)`。** 它先尝试提取 `apperr.Error`，
提取不到就记日志并返回 500——既不泄露内部细节，也不让意外错误静默消失。

**3. 照片归属回写必须无条件执行。** 见下方说明。

## ⚠️ 归属回写那个坑

`CreatePhoto` / `UpdatePhoto` 里有一段：

```go
// AssignPhoto 只改内存里的 photo.PlaceID，而这一行是插入之后才确定的，
// 因此必须显式回写一次。
if place != nil {
    tx.Model(&model.Photo{}).Where("id = ?", photo.ID).Update("place_id", place.ID)
}
```

早期版本的条件是「值不同才更新」，而 `service.AssignPhoto()` 恰好已经把它设成了
同一个值——于是**唯一需要写库的那次被跳过**。症状很有迷惑性：
聚类看着是对的（地点确实建出来了、日期范围也继承了），
但照片在地图上永远挂不到地点下，`photo_count` 恒为 0。

## 各资源处理器的要点

**auth**：登录时无论邮箱是否存在都做一次哈希比对，避免通过响应时间枚举账号；
邮箱统一小写存储。

**photo**：`upload-url` 与 `photo` 登记都做幂等短路（按 `content_hash`）；
无 GPS 的照片不归属任何地点，落进前端的「未定位」托盘。

**place**：删除时默认保留照片（`place_id` 置空），
只有显式传 `delete_photos=true` 才连带删对象存储里的文件。

**geo**：上游失败转成 502，而不是把第三方异常直接透给客户端。

## 相关

- 上级：[`internal/`](../README.md)
- 装配：[`router.go`](router.go) · [`cmd/server/`](../../cmd/server/README.md)
- 契约：[`dto/`](../dto/README.md)
