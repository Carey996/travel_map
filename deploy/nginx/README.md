# deploy/nginx · 前端容器配置

> 知识树：[项目根](../../README.md) › [`deploy/`](../README.md) › **`nginx`**

| 文件 | 说明 |
|---|---|
| `default.conf.template` | 前端容器的 nginx 配置 |

## 为什么是 `.template`

官方 nginx 镜像会对 `/etc/nginx/templates/*.template` 做 envsubst 后写到
`/etc/nginx/conf.d/`，因此上游地址可以用环境变量注入：

| 环境 | `API_UPSTREAM` |
|---|---|
| Kubernetes | `travel-blog-api:8000`（Service DNS） |
| 本地单 Pod | `127.0.0.1:8000`（同 Pod 内容器共享网络） |

一份配置同时适配两种编排，不必维护两份。

## 监听 8080 而不是 80

容器因此可以完全以非 root（UID 101）运行。K8s 清单里配了 `runAsNonRoot: true`
与 `readOnlyRootFilesystem: true`，二者配合形成纵深防御。

## 三个 location 的意图

**`/`** 用 `try_files $uri $uri/ /index.html` 兜底——SPA 的前端路由
（`/timeline`、`/place/xxx`）在服务端并不存在实体文件，必须交回 index.html。

**`/api/` 与 `/healthz`** 反代到后端。这是**同源策略的关键**：
浏览器看到的前后端在同一个 origin 下，会话 Cookie 的 `SameSite=Lax` 正常工作，
也不必给前端在 CORS 白名单里额外开一个来源。

**静态资源**长缓存 30 天（hashed 文件名，`expires 30d`），但 `index.html` 显式
`no-cache`（`expires -1`）——否则发版后用户会拿到引用旧资源哈希的旧 HTML。

## 安全响应头

server 级统一下发，location 不再重复：

| 头 | 值 | 作用 |
|---|---|---|
| `X-Content-Type-Options` | `nosniff` | 禁止 MIME 嗅探 |
| `X-Frame-Options` | `SAMEORIGIN` | 防点击劫持（与 CSP `frame-ancestors 'self'` 一致） |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | 跨站只带 origin，不带路径 |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | 关掉用不到的浏览器能力 |
| `Content-Security-Policy` | 见模板 | 脚本/字体同源；样式允许行内；图片允许 `data:`/`blob:`/`https:`/`http:`；连接允许 `https:`/`http:` |
| `server_tokens` | `off` | 不暴露 nginx 版本 |

CSP 里的 `http:` 是给「单 Pod / 本地验证」留的口子（那里的对象存储是明文地址）；
HTTPS 页面下浏览器会按混合内容规则拦掉 http 资源，所以不放松线上安全。
新增第三方源（地图、字体、埋点）时，按浏览器 Console 里被拦的 URL 追加即可。

HSTS 不在这里加：它只对 HTTPS 生效，由 compose 的 Caddy 层负责（见
[`compose/`](../compose/README.md)「安全响应头」一节）。

## 为什么 location 里不写 `add_header`

nginx 的规则是：**子块里只要出现一条 `add_header`，就不再继承 server 级的全部 `add_header`**。
若在静态资源 location 里写 `add_header Cache-Control …`，上面那批安全头会在
这些响应上悄悄消失；同时 `expires` 与 `add_header Cache-Control` 叠加还会输出两条
`Cache-Control`（线上实际踩到过）。

因此缓存只用 `expires` 指令表达，安全头只在 server 级写一份——改头时只改一处。

## 相关

- 上级：[`deploy/`](../README.md) · 使用者：[`containerfiles/`](../containerfiles/README.md)
