# `api/internal/model` · 数据模型

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`model`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `model.go` | 全部 GORM 模型 | `User`, `Place`, `Photo`, `Note`, `Trip`, `TripPlace`, `Share`, `UserSession`, `NewID()`, `AllModels()` |
| `types.go` | 自定义字段类型 | `StringList`（`[]string` ↔ jsonb） |

表结构与 Python 版**完全一致**，可以直接对接同一个数据库。

`F`
User ─┬─< Place ─┬─< Photo
      │          └─< Note
      ├─< Trip ──< TripPlace >── Place      （行程串联，界面待启用）
      ├─< Share                             （公开分享，界面待启用）
      └─< UserSession                       （仅 SESSION_BACKEND=db 时使用）
`F`

## 三条约定

**1. 所有列名都显式声明 `column:` 标签。** 这不是啰嗦，是必要的：GORM 的自动蛇形转换
对 `SummaryMD` / `BodyMD` 这类连续大写缩写并不可靠——`MD` 不在 GORM 的
commonInitialisms 名单里，转换结果可能是 `summary_m_d`。显式声明彻底消除这类隐患。

**2. 表名用 `TableName()` 固定。** 不依赖 GORM 的复数化推断，
模型改名时不会悄悄换表。

**3. 主键在 `BeforeCreate` 里生成 UUID v7。** 时间有序，且无状态服务可本地生成，
无需向数据库申请序列。`uuid.NewV7()` 失败时退化为 v4——理论上不会发生，
但也好过 panic。

## 字段可见性

带 `json:"-"` 的字段**永远不会出现在 API 响应里**，这是刻意的：

| 字段 | 为什么不外露 |
|---|---|
| `User.PasswordHash` | 显然不能 |
| `Photo.StorageKey` / `PreviewKey` / `ThumbKey` | 对象键只在服务端使用；前端拿到的应是预签名直链 |
| `Photo.ContentHash` | 内部去重依据，无对外意义 |
| `Place.UserID` / `Photo.UserID` / `Note.UserID` | 多租户隔离依据；客户端不需要也不该看到 |

## 关于 StringList

`tags` 列用 jsonb 而非 Postgres 原生 `text[]`，是为了与 Python 版保持一致
（当时选它是因为 SQLite 也能用）。`StringList` 实现了 `driver.Valuer` /
`sql.Scanner`，因此可以像普通字段一样赋值。

## 新增字段的流程

改本文件 → 在 `dto` 暴露出去 → 重启（自动迁移会补列）。
**改动模型后必须同步更新本 README 的表格。** 若列名有歧义风险，务必写 `column:` 标签。

## 相关

- 上级：[`internal/`](../README.md)
- 迁移：[`db/`](../db/README.md) · 契约：[`dto/`](../dto/README.md)
