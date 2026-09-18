# `web/src/api` · 后端接口出口

> 知识树：[项目根](../../../README.md) › [`web`](../../README.md) › [`src/`](../README.md) › **`api`**

**全应用唯一的后端出口。** 所有请求都经这里发出，组件里不应该出现裸 `fetch`。

## 内容

| 文件 | 职责 |
|---|---|
| `client.ts` | `apiFetch()` 统一封装 + `api` 对象（全部端点）+ 与后端 Pydantic 模型一一对应的 TypeScript 类型 |

## `apiFetch()` 承担的三件事

```typescript
// 1. 变更类请求自动加 CSRF 头（后端据此抵御跨站请求）
if (UNSAFE_METHODS.has(method)) headers.set('X-Requested-With', 'XMLHttpRequest');

// 2. 会话走 HttpOnly Cookie，前端读不到也不需要读
credentials: 'include'

// 3. 把后端的统一错误体转成可 catch 的 ApiError
throw new ApiError(status, code, message, detail);
```

后端所有失败都返回同一形状 `{code, message, detail}`，因此这里只需处理一种结构，
调用方用 `instanceof ApiError` 判断即可拿到 `status` 与业务错误码。

## 类型位置

响应类型（`Place` / `Photo` / `Note` / `User` / `GeoSearchItem` …）
与请求载荷类型（`PhotoCreatePayload` / `PhotoQuery` …）都定义在这个文件里，
组件从 `@/api/client` 导入，**不要就地重复定义**。

改动后端契约时，这里的类型是唯一需要同步的地方。

## 约定

**不要在组件里绕过它。** 绕过会同时失去 CSRF 头、Cookie 凭据与统一错误处理，
后端会直接返回 403 `csrf_header_missing`。

**`lib/upload.ts` 是唯一合理的外部使用者**——它需要用 `api` 拿预签名 URL 并登记元数据。

## 相关

- 上级：[`src/`](../README.md)
- 同级：[`lib/`](../lib/README.md) · [`stores/`](../stores/README.md) · [`components/`](../components/README.md) · [`i18n/`](../i18n/README.md) · [`styles/`](../styles/README.md)
