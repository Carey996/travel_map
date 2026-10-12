# `api/internal/db` · 数据库

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`db`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `db.go` | 连接池与结构迁移 | `Connect()`, `Migrate()` |

## Connect

用 pgx 驱动打开 GORM，然后**显式 Ping 一次**——否则首次请求才发现数据库不可达，
错误会以 500 的形式暴露给用户而不是在启动日志里。

连接池参数（最大 20 / 空闲 5 / 生命周期 1 小时）是容量问题而非配置问题，
写死在代码里，等真有压力时再抽出去。`NowFunc` 固定为 UTC，避免容器时区差异带来的隐性 bug。

## Migrate

```go
gdb.AutoMigrate(model.AllModels()...)
```

**与 Python 版的 Alembic 是两种思路，需要理解差异：**

| | Alembic（旧） | GORM AutoMigrate（现） |
|---|---|---|
| 迁移记录 | 有版本表，可前进后退 | 无版本概念，每次启动比对实际结构 |
| 行为 | 按脚本执行 DDL | **只增不删**：补齐缺失的表/列/索引 |
| 回滚 | `alembic downgrade` | 需手工写 SQL |
| 适用 | 需要审计与灰度 | 开发期快速迭代、单实例部署 |

「只增不删」意味着它**不会**删掉你手工删过的列，也不会改窄字段类型——
对本项目这种结构相对稳定的场景是安全的，且可重复执行。

如果要上生产且需要迁移审计，应换成 golang-migrate 或 goose 管理显式 SQL 文件；
届时记得把 `cmd/server` 里的 `Migrate()` 调用去掉。

## 相关

- 上级：[`internal/`](../README.md)
- 模型：[`model/`](../model/README.md)
