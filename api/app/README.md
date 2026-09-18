# `api/app` · 应用包

> 知识树：[项目根](../../README.md) › [`api`](../README.md) › **`app`**

后端应用的全部代码。**依赖方向是单向的**，理解这一点就理解了整个包的形状：

```
main.py                    组装：中间件 -> 错误处理 -> 路由
  └─ api/                  HTTP 层（依赖注入、路由）
       └─ services/        领域服务（存储、聚类、会话、地名）
            └─ db/         ORM 模型与会话
                 └─ core/  基础设施（配置、安全、错误、ID、地理）
```

`core` 不导入任何业务模块；`db` 只依赖 `core`；`services` 依赖 `db` 与 `core`；
`api` 依赖所有下层；`main.py` 只做组装。**反向依赖是不允许的。**

## 子目录

| 目录 | 一句话 |
|---|---|
| [`core/`](core/README.md) | 横切关注点：配置、安全、错误、CSRF 中间件、UUID v7、球面距离 |
| [`db/`](db/README.md) | 数据层：异步引擎、全部 ORM 模型、迁移元数据 |
| [`schemas/`](schemas/README.md) | Pydantic 契约：请求与响应模型 |
| [`services/`](services/README.md) | 领域服务：对象存储、地点聚类、会话存储、地名代理 |
| [`api/`](api/README.md) | HTTP 层：依赖注入与版本化路由 |

## 顶层文件

| 文件 | 职责 |
|---|---|
| `main.py` | `create_app()` 组装应用；`lifespan()` 启动时建引擎 / Redis / 存储客户端并确保 bucket 存在，关闭时统一释放 |

## 中间件顺序

```
CORSMiddleware          最外层：预检与白名单之外的跨站请求在这里被拦下
  └─ CsrfHeaderMiddleware   变更类请求必须带 X-Requested-With
       └─ 路由
```

FastAPI 中「后添加的中间件更靠外」，所以 `main.py` 里先加 CSRF、后加 CORS。
顺序写反会导致预检请求先撞上 CSRF 校验而失败。

## 相关

- 上级：[`api/`](../README.md)
