# `api/internal/session` · 会话存储

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`session`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `session.go` | 同一接口两种实现 | `Store`, `RedisStore`, `DBStore`, `NewToken()`, `HashToken()` |

## 为什么不使用 JWT

会话 id 是**不透明随机串**（256 位熵）而非自包含令牌。服务端只保存它：
Redis 直接以它为键，Postgres 存 sha256 摘要。因此即使存储泄露，也无法直接冒用会话。

更重要的是：两种实现都支持**即时撤销**，这是自包含 JWT 做不到的
（除非再引入一张黑名单表，那就已经不是无状态了）。

## 两种实现

| | RedisStore（默认） | DBStore（`SESSION_BACKEND=db`） |
|---|---|---|
| 过期 | TTL 自动清理 | 查询时比对 `expires_at` |
| 撤销 | `DEL` + 从索引集合移除 | 标记 `revoked_at` |
| 每请求成本 | 一次 GET | 一次带条件的 SELECT |
| 额外组件 | 需要 Redis | 无（复用 Postgres） |

`DBStore` 的价值在于「零额外组件起步」——不跑 Redis 也能把项目跑起来。

## Redis 实现的两个键

`F`
tb:sess:<sid>            -> user_id，带 TTL
tb:user:<user_id>:sess   -> 该用户的 sid 集合（用于「退出所有设备」）
`F`

反向索引不能省：没有它就无法实现 `RevokeAllForUser()`，
而「登出所有设备」是安全基本盘。

## 会话 id 的形态

`NewToken()` 生成 32 字节随机数后做 base64url 编码（无填充），
约 43 个字符，可以安全地放进 Cookie。**不要**换成可预测的格式。

## 相关

- 上级：[`internal/`](../README.md)
- 使用方：[`handler/auth.go`](../handler/README.md)
