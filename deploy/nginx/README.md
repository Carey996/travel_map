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

**静态资源**长缓存 30 天并加 `immutable`，但 `index.html` 显式 `no-cache`——
否则发版后用户会拿到引用旧资源哈希的旧 HTML。

## 相关

- 上级：[`deploy/`](../README.md) · 使用者：[`containerfiles/`](../containerfiles/README.md)
