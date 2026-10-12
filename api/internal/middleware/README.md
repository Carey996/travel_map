# `api/internal/middleware` · CORS 与 CSRF

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`middleware`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `cors.go` | 带凭据的跨域白名单 | `CORS()` |
| `csrf.go` | 要求变更类请求携带自定义头 | `CSRF()` |

只依赖 Gin，与业务无关。

## 为什么需要 CSRF 防护

因为选了 Cookie 承载会话（比 localStorage 抗 XSS），就必然引入 CSRF 面。
三层防护缺一不可：

1. **Cookie 的 `SameSite=Lax`** —— 跨站表单 POST 不会携带会话
2. **CORS 严格白名单** —— 跨站 fetch 带自定义头会触发预检并被拒
3. **本文件的 `CSRF()` ** —— HTML 表单无法设置自定义头，跨站 fetch 又过不了预检

## 中间件顺序至关重要

`F`go
r.Use(gin.Recovery())
r.Use(RequestLogger())
r.Use(middleware.CORS(...))   // 必须在前
r.Use(middleware.CSRF(...))
`F`

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

## 相关

- 上级：[`internal/`](../README.md)
- 装配处：[`handler/router.go`](../handler/README.md)
