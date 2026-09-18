# `api/app/api` · HTTP 层

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`app`](../README.md) › **`api`**

把 HTTP 请求翻译成服务层调用所需的一切：依赖注入、参数解析、版本化路由。
**这里不写业务规则**，规则在 [`../services/`](../services/README.md)。

## 内容

| 条目 | 类型 | 职责 |
|---|---|---|
| [`v1/`](v1/README.md) | 目录 | 版本化的业务路由（auth / places / photos / notes / geo / health） |
| `deps.py` | 文件 | 依赖注入：数据库会话、当前用户、存储后端、会话存储 |
| `query.py` | 文件 | 视口参数解析 `parse_bbox()` |

## deps.py 提供的依赖

| 依赖 | 类型别名 | 说明 |
|---|---|---|
| `get_db` | `DbSession` | 每个请求一个 `AsyncSession`，退出时自动关闭 |
| `get_current_user` | `CurrentUser` | 读 Cookie → 查会话 → 载入用户；任一环节失败抛 401 |
| `get_optional_user` | `OptionalUser` | 同上但允许匿名（供后续公开分享页使用） |
| `get_storage` | `Storage` | 对象存储客户端 |
| `get_session_store` | `Sessions` | 会话存储 |
| `get_app_settings` | `AppSettings` | 只读配置 |

用类型别名（`CurrentUser` 而不是 `Annotated[User, Depends(...)]`）的目的是让路由签名保持简短可读。

## 约定

**所有共享资源从 `request.app.state` 取。** 它们在 `lifespan` 里创建，
属于进程级基础设施（连接池、客户端），不是业务状态。

**不要在这里 `commit`。** 依赖只负责提供会话，事务由路由决定何时提交。

**新增版本时新建 `v2/` 目录**，不要改动 `v1/`——已发布的契约应当保持稳定。

## 相关

- 上级：[`app/`](../README.md)
- 子目录：[`v1/`](v1/README.md)
- 同级：[`core/`](../core/README.md) · [`db/`](../db/README.md) · [`schemas/`](../schemas/README.md) · [`services/`](../services/README.md)
