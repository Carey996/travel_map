# `api/internal/middleware` · CORS、CSRF、请求体上限与限流

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`middleware`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `cors.go` | 带凭据的跨域白名单 | `CORS()` |
| `csrf.go` | 要求变更类请求携带自定义头 | `CSRF()` |
| `bodylimit.go` | 请求体字节上限 | `LimitBody()` |
| `ratelimit.go` | 按 IP 的固定窗口限流（计数存 Redis） | `RateLimit()`, `RateCounter`, `NewRedisRateCounter()` |

只依赖 Gin、go-redis 与标准库；唯一的项目内依赖是 `apperr`（429 也要走统一错误契约），
不引用任何业务包。

## 为什么需要 CSRF 防护

因为选了 Cookie 承载会话（比 localStorage 抗 XSS），就必然引入 CSRF 面。
三层防护缺一不可：

1. **Cookie 的 `SameSite=Lax`** —— 跨站表单 POST 不会携带会话
2. **CORS 严格白名单** —— 跨站 fetch 带自定义头会触发预检并被拒
3. **本文件的 `CSRF()` ** —— HTML 表单无法设置自定义头，跨站 fetch 又过不了预检

## 中间件顺序至关重要

```go
r.Use(gin.Recovery())
r.Use(RequestLogger())
r.Use(middleware.CORS(...))   // 必须在前
r.Use(middleware.CSRF(...))
```

`r.Use` 按注册顺序执行。**CORS 必须在 CSRF 之前**，否则跨站请求会先撞上 CSRF 校验
（返回 403），而不是在预检阶段就被干净地拒绝——虽然结果都是拒绝，
但前者会让浏览器的错误信息变得难以诊断。

## 白名单之外的行为

拿不到 `Access-Control-Allow-Origin`，浏览器自行拦截。服务端**不额外报错**——
这样不会把「哪些来源被允许」暴露给探测者。

## 探针豁免

`/healthz` 与 `/healthz/ready` 被排除在 CSRF 校验外：
它们不属于版本化业务 API，且都是 GET（本就在安全方法内），豁免是为了让
未来可能加的就绪检查保持简单。

## 请求体上限

`LimitBody(maxBytes)` 用标准库的 `http.MaxBytesReader` 把 `c.Request.Body` 包一层：
读取超过 `maxBytes` 时返回 `*http.MaxBytesError`。它**自己不写响应**——
翻译成 413 `payload_too_large` 的是 `handler.bind()`，这样错误形状仍由 HTTP 层统一决定。

装配在 `handler/router.go` 的 `/api/v1` 分组上（1 MiB），并且**必须在创建子分组之前**
`v1.Use(...)`：gin 的 `Group()` 在创建时复制父级 handlers，之后再 `Use` 不会回溯到
已创建的子分组。探针挂在根路径、不在该分组上，不受影响。

后端不接收图片字节流（图片走预签名直传对象存储），业务请求都是元数据/坐标之类的小 JSON，
1 MiB 已经宽裕；上限存在的意义是防止超大 body 在反序列化时打满内存。

## 认证入口限流

`RateLimit(counter, prefix, limit, window)` 给「登录 / 注册」这类撞库入口加成本。
参数在装配处（[`handler/router.go`](../handler/README.md)）用常量声明，刻意不做成环境变量——
限流值是安全参数，应跟代码一起 review：

| 入口 | 限额 | key 前缀 |
|---|---|---|
| `POST /api/v1/auth/login` | 10 次 / 5 分钟 / IP | `tb:ratelimit:auth:login:5m` |
| `POST /api/v1/auth/register` | 5 次 / 1 小时 / IP | `tb:ratelimit:auth:register:1h` |

行为：

- **限内**：放行。**超限**：不再调用 handler，直接 **429**，带 `Retry-After: <剩余秒>` 头，
  body 仍是统一错误契约：`{"code":"rate_limited","message":"尝试过于频繁，请稍后再试",
  "detail":{"retry_after_seconds":…,"limit":…,"window_seconds":…}}`。
- **计数器故障 fail-open**：Redis 报错时放行并 `slog.Warn`，绝不因限流组件让登录整体不可用。
- **counter 为 nil**：直接放行（未装配 Redis 或显式降级）。
- key = `prefix + ":" + c.ClientIP()`。前缀里带窗口（`...:5m` / `...:1h`），
  改窗口时旧计数自然落到旧命名空间，不会污染新窗口。

### 为什么用 Redis 而不是进程内计数

硬约束是「后端无状态：不存进程内会话、不做进程内缓存、不写本地磁盘」。进程内 map 计数有两个
致命问题：多副本部署时每个实例各算各的，实际限额被放大成 N 倍；进程重启就清零，攻击者可以
用重启窗口反复试探。Redis 本来就是会话后端，计数放同一份共享存储，跨实例天然一致，
也和「无状态」约束一致（状态在外部存储，不在进程里）。

### 计数怎么做到原子

`redisRateCounter.Hit` 在一个 `TxPipeline`（MULTI/EXEC）里执行 `INCR` + `EXPIRE ... NX` + `TTL`：

- 只 `INCR` 不设 TTL：一旦写成功而后续步骤丢失，key 永不过期，一次抖动就把某个 IP 永久挡在门外。
- 每次命中都普通 `EXPIRE`：窗口被不断续期，变成「最后一次请求之后才重开」，持续访问的用户越锁越久。
- `EXPIRE ... NX` 只在 key 还没有 TTL 时生效，正好是「首次命中设置窗口」的语义。

`EXPIRE ... NX` 需要 **Redis ≥ 7.0**；仓库所有编排（根 README 快速开始、`deploy/compose`、
`deploy/k8s`）统一是 `redis:7-alpine`。若连到更老的 Redis，EXEC 会报错，上层 fail-open 放行并留告警——
限流失效，但登录不会被拦死。

### 客户端 IP 与可信代理

`c.ClientIP()` 采信 `X-Forwarded-For` 的前提是「直连对端是可信代理」。gin 的默认值信任**所有**
来源，这意味着任何客户端都能伪造 XFF：限流既可以被轮换 XFF 绕过，也可以被用来占满别人的桶。
因此 [`handler/router.go`](../handler/README.md) 用 `SetTrustedProxies` 只声明
**回环 + 私有网段**（`127.0.0.0/8`、`::1/128`、`10/8`、`172.16/12`、`192.168/16`、`fd00::/8`）——
nginx / Caddy / Pod 内代理都落在这里；公网直连 API 的对端不在其中，其 XFF 不会被采信。
`X-Forwarded-For` 由 nginx 模板用 `$proxy_add_x_forwarded_for` 追加，真实客户端 IP 在右侧，
gin 从右往左取第一个非可信地址，攻击者塞在最左边的伪造值不会被选中。

> 部署时若在 API 前面加了新的代理层（例如只有 IP 不在私有网段的云负载均衡），
> 需要把它的网段补进 `trustedProxyCIDRs`，否则限流会退化成按代理 IP 分桶（所有用户共用一个桶）。

## 相关

- 上级：[`internal/`](../README.md)
- 装配处：[`handler/router.go`](../handler/README.md)
- 413 映射：[`handler/handler.go` 的 `bind()`](../handler/README.md)
