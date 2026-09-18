# `api/app/db` · 数据层

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`app`](../README.md) › **`db`**

数据库引擎、ORM 模型与迁移元数据的汇总入口。

## 文件

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `session.py` | 异步引擎与会话工厂 | `Base`, `create_engine()`, `create_session_factory()`, `get_session()` |
| `models.py` | 全部 ORM 模型 | `TimestampMixin`, `User`, `Place`, `Photo`, `Note`, `Trip`, `TripPlace`, `Share`, `UserSession` |
| `base.py` | 汇总导入全部模型，供 Alembic autogenerate 发现所有表 | `target_metadata` |

## 数据模型速览

```
User ─┬─< Place ─┬─< Photo
      │          └─< Note
      ├─< Trip ──< TripPlace >── Place      （行程串联，界面待启用）
      ├─< Share                             （公开分享，界面待启用）
      └─< UserSession                       （仅 SESSION_BACKEND=db 时使用）
```

几个刻意的选择：

- **UUID v7 主键**：时间有序，服务端本地生成，无需向数据库申请序列。
- **`photos` 上的 `uq_photos_user_content`**：`(user_id, content_hash)` 唯一，
  让上传幂等；注意是「每用户唯一」而非全局唯一，两个用户拍到同一张图不应互相冲突。
- **`notes` 是独立表**而非 `places` 上的字段：一个地点可以按天写多篇。
- **外键都带 `ondelete`**：删用户级联删地点，删地点时照片的 `place_id` 置空（照片不丢）。

## 约定

**新增模型后必须更新 `base.py`。** 它只是一个导入汇总，漏了会让 autogenerate 看不到新表。

**改动模型后必须生成迁移**，见 [`api/README.md`](../../README.md#数据库迁移)。

## 相关

- 上级：[`app/`](../README.md)
- 同级：[`core/`](../core/README.md) · [`schemas/`](../schemas/README.md) · [`services/`](../services/README.md) · [`api/`](../api/README.md)
- 迁移脚本：[`alembic/versions/`](../../alembic/versions/README.md)
