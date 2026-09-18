# `api/app/api/v1` · HTTP 路由

> 知识树：[项目根](../../../../README.md) › [`api`](../../../README.md) › [`app`](../../README.md) › [`api/`](../README.md) › **`v1`**

版本化的业务接口。每个文件对应一组资源，职责是**解析请求 → 调用服务 → 返回契约模型**，
业务规则不写在这里。

## 文件

| 文件 | 前缀 | 端点 |
|---|---|---|
| `health.py` | 根路径 | `GET /healthz`（存活）、`GET /healthz/ready`（就绪，检查 DB / Redis / 对象存储） |
| `auth.py` | `/auth` | `POST /register`、`/login`、`/logout`、`/logout-all`、`GET /me` |
| `places.py` | `/places` | `GET /`、`POST /`、`GET /{id}`、`PATCH /{id}`、`DELETE /{id}`、`POST /{id}/assign`、`POST /{id}/merge-into` |
| `photos.py` | `/photos` | `POST /upload-url`、`POST /`、`GET /`、`PATCH /{id}`、`DELETE /{id}` |
| `notes.py` | `/notes` | `GET /`、`POST /`、`PATCH /{id}`、`DELETE /{id}` |
| `geo.py` | `/geo` | `GET /reverse`、`GET /search` |
| `router.py` | —— | `api_router`：汇总以上模块，由 `main.py` 挂到 `/api/v1` |

## 几个实现要点

**健康探针挂在根路径而不是 `/api/v1` 下。** 它们不属于版本化业务 API，
便于容器编排与网关直接访问，同时也被排除在 CSRF 校验之外。

**每个查询都必须带 `user_id` 过滤。** 这是多租户隔离的唯一防线；
`_get_owned()` 这类小助手就是为了避免漏写。

**归属与合并是「改照片的 place_id」而不是「搬地点」。** `assign_photos` 把照片移入目标地点；
`merge_place` 在移动照片和笔记后删除被合并的那个地点。

**删除地点默认保留照片**（`place_id` 置空，落进「未定位」托盘），
只有显式传 `delete_photos=true` 才连同对象存储里的文件一起删。

**Geo 接口的响应带 `Cache-Control`**，缓存责任交给浏览器——后端不保存任何地名结果。

## 相关

- 上级：[`api/`](../README.md)
- 同级：[`core/`](../../core/README.md) · [`db/`](../../db/README.md) · [`schemas/`](../../schemas/README.md) · [`services/`](../../services/README.md)
