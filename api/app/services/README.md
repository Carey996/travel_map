# `api/app/services` · 领域服务

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`app`](../README.md) › **`services`**

可复用的业务逻辑，不依赖 FastAPI 的请求对象。路由层很薄，真正的规则都在这里。

## 文件

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `storage.py` | S3 兼容对象存储：签发预签名 URL、删除对象、启动时确保 bucket 存在 | `StorageBackend`（协议）, `S3Storage` |
| `places.py` | 照片按 1.5 km 半径自动归入地点；让地点继承照片的日期范围 | `AUTO_ASSIGN_RADIUS_M`, `nearest_place()`, `assign_photo()` |
| `session_store.py` | 会话存储，同一个接口两种后端 | `SessionStore`（协议）, `RedisSessionStore`, `DbSessionStore`, `build_session_store()` |
| `geocode.py` | 地名服务代理，可插拔 | `Geocoder`, `PhotonProvider`, `NominatimProvider`, `pick_short_name()` |

## 三个关键设计

**后端不接收图片字节流。** `storage.py` 只签发预签名 URL，浏览器直接传到对象存储。
`presign_put` / `presign_get` 是**纯本地计算**（不发网络请求），所以可以在请求里现签，
既不必在库里存易过期的 URL，也不需要任何缓存。

**会话存储可替换。** `SessionStore` 是协议，`build_session_store()` 依据
`SESSION_BACKEND` 返回 Redis 或 Postgres 实现。两者都支持即时撤销——
这是 JWT 自包含方案做不到的。

**地名服务可替换。** 实测中国大陆网络下 Nominatim 不可达，而 Photon 可达且原生返回中文地名，
因此默认用 Photon。切换只需改 `GEOCODE_PROVIDER`；`Geocoder` 内部还带一个
轻量节流器，纯粹为遵守对方的使用条款，不属于业务状态。

## 约定

**服务函数接收 `AsyncSession` 而不是自己开会话。** 事务边界由路由层掌握，
服务层只负责 `db.add` / `flush`，由调用方 `commit`。

**外部调用失败要转成可读错误。** 例如 `geocode.py` 的 httpx 异常由 `api/v1/geo.py`
捕获并转成 `ApiError(502, "geocode_upstream_error", ...)`。

## 相关

- 上级：[`app/`](../README.md)
- 同级：[`core/`](../core/README.md) · [`db/`](../db/README.md) · [`schemas/`](../schemas/README.md) · [`api/`](../api/README.md)
