# `api/alembic` · 数据库迁移

> 知识树：[项目根](../../README.md) › [`api`](../README.md) › **`alembic`**

Alembic 迁移环境。**由脚本自动生成，通常不需要手改**，除非要写数据迁移。

## 内容

| 条目 | 职责 |
|---|---|
| [`versions/`](versions/README.md) | 迁移脚本本身（自动生成） |
| `env.py` | 运行环境：从应用配置注入连接串，使用异步引擎 |
| `script.py.mako` | 新迁移文件的模板 |

配置在上一级的 `alembic.ini`，注意两点：

- `prepend_sys_path = .` 让 `app` 包可被导入
- **不能写中文注释**——alembic 用系统区域编码读配置，在中文 Windows 上会直接解析失败

## 常用命令

```powershell
.venv\Scripts\python.exe -m alembic current        # 当前版本
.venv\Scripts\python.exe -m alembic history        # 历史
.venv\Scripts\python.exe -m alembic upgrade head   # 应用到最新
.venv\Scripts\python.exe -m alembic downgrade -1   # 回滚一步
```

## 相关

- 上级：[`api/`](../README.md)
- 子目录：[`versions/`](versions/README.md)
- 模型定义：[`app/db/`](../app/db/README.md)
