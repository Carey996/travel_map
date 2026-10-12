# `api/internal` · 实现总览

> 知识树：[项目根](../../README.md) › [`api`](../README.md) › **`internal`**

放在 `internal/` 下是 Go 的可见性约定：**这些包不允许被本模块之外的代码导入**。
对一个应用（而非库）来说，这是正确的默认值——没有对外 API 承诺，重构就自由。

## 依赖方向

`F`
handler/       HTTP 层：路由、鉴权、请求校验、响应序列化
  ├─ dto/      契约模型（只被 handler 使用）
  └─ service/  业务逻辑：照片聚类
       └─ model/ 数据模型
            └─ db/ · config/ · apperr/ · geo/
middleware/    与业务无关的横切关注点，只依赖 gin
storage/ session/ geocode/   平级的服务包，互不依赖
`F`

**依赖是单向的**：下层不允许引用上层。判断方法很简单——
一个包如果 import 了 `handler` 或 `dto`，那它放错层了。

## 子目录

| 目录 | 一句话 |
|---|---|
| [`config/`](config/README.md) | 环境变量配置与两处连接串归一化 |
| [`apperr/`](apperr/README.md) | 统一业务错误形状 |
| [`model/`](model/README.md) | GORM 模型、UUID v7、jsonb 字段类型 |
| [`db/`](db/README.md) | 连接池与 AutoMigrate |
| [`dto/`](dto/README.md) | 请求校验与响应契约 |
| [`geo/`](geo/README.md) | Haversine 距离 |
| [`geocode/`](geocode/README.md) | 地名服务代理（Photon / Nominatim） |
| [`storage/`](storage/README.md) | 预签名上传与读取 |
| [`session/`](session/README.md) | 会话存储两种实现 |
| [`service/`](service/README.md) | 照片自动聚类与日期范围继承 |
| [`middleware/`](middleware/README.md) | CORS 与 CSRF |
| [`handler/`](handler/README.md) | 路由与各资源处理器 |

## 相关

- 上级：[`api/`](../README.md)
- 入口装配：[`cmd/server/`](../cmd/server/README.md)
