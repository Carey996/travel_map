# `api/app/core` · 横切关注点

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`app`](../README.md) › **`core`**

与具体业务无关的基础设施：配置读取、密码与会话令牌、统一错误、CSRF 中间件、主键生成、地理计算。
这些模块**不导入任何业务模块**，可以被任意一层安全引用。

## 文件

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `config.py` | 全部配置来自环境变量（支持 `.env`）；用绝对路径定位配置文件 | `Settings`, `get_settings()` |
| `security.py` | bcrypt 密码哈希；生成不透明会话令牌并计算其 sha256 摘要 | `hash_password`, `verify_password`, `new_session_token`, `hash_token` |
| `errors.py` | 把任何异常归一成 `{code, message, detail}` | `ApiError`, `register_error_handlers()` |
| `middleware.py` | 变更类请求必须携带自定义头，抵御 CSRF | `CsrfHeaderMiddleware`, `SAFE_METHODS` |
| `ids.py` | 主键生成：UUID v7（时间有序），无状态服务可本地生成 | `new_id()` |
| `geo.py` | 球面距离计算，供地点聚类使用 | `haversine_m()`, `EARTH_RADIUS_M` |
| `redis.py` | Redis 异步客户端工厂 | `create_redis()` |

## 约定

**配置只从这里读。** 业务代码通过 `get_settings()` 获取配置，不要直接读 `os.environ`——
它带 `lru_cache`，且集中处理了类型转换与默认值。

**错误只抛 `ApiError`。** 抛出后由 `register_error_handlers()` 统一转成 JSON，
前端因此只需要处理一种响应形状。

**主键只从 `new_id()` 来。** 不要用数据库自增——那会让无状态服务重新需要协调。
`ids.py` 在 Python 3.14+ 直接用标准库 `uuid.uuid7()`，低版本走等价的 RFC 9562 实现。

**令牌只存摘要。** `new_session_token()` 生成的是不透明随机串，服务端只保存
`hash_token()` 的结果，即使存储泄露也无法直接冒用会话。

## 相关

- 上级：[`app/`](../README.md)
- 同级：[`db/`](../db/README.md) · [`schemas/`](../schemas/README.md) · [`services/`](../services/README.md) · [`api/`](../api/README.md)
