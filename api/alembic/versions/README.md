# `api/alembic/versions` · 迁移脚本

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`alembic/`](../README.md) › **`versions`**

**这个目录由 Alembic 自动生成，不要手写。** 每个文件是一次不可变的结构变更。

## 现有迁移

| 版本 | 说明 |
|---|---|
| `2599520383c5_initial_schema.py` | 初始结构：`users` / `places` / `photos` / `notes` / `trips` / `trip_places` / `shares` / `user_sessions` 八张表及全部索引 |

## 工作流

```powershell
# 生成（自动对比 models.py 与数据库的差异）
.venv\Scripts\python.exe -m alembic revision --autogenerate -m "描述本次改动"

# 应用
.venv\Scripts\python.exe -m alembic upgrade head

# 回滚一步
.venv\Scripts\python.exe -m alembic downgrade -1
```

> 开发期若迁移写乱且无数据需要保留：`alembic downgrade base` → 删除本目录下的 `*.py` →
> 重新 autogenerate。**生产环境不要这样做。**

## 约定

**每次改 `app/db/models.py` 都要配一次迁移**，并且要 review 自动生成的结果——
autogenerate 对索引改名、唯一约束变更这类场景并不总是准确。

**本目录的文件一旦提交就不要修改**，需要变更请追加新迁移。

## 相关

- 上级：[`alembic/`](../README.md)
- 模型定义：[`app/db/models.py`](../../app/db/README.md)
