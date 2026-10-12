# `api/cmd/server` · HTTP 服务入口

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`cmd/`](../README.md) › **`server`**

| 文件 | 职责 |
|---|---|
| `main.go` | 装配全部依赖、启动 HTTP 服务、处理优雅关闭 |

`main()` 只做一件事：调用 `run()` 并在失败时以非零码退出。
真正的装配都在 `run()` 里，顺序即依赖顺序：

`F`
config.Load()            读环境变量（含连接串归一化）
  -> db.Connect()        连接池 + Ping
  -> db.Migrate()        GORM AutoMigrate（只增不删，可重复执行）
  -> redis.NewClient()   + Ping
  -> storage.New()       + EnsureBucket
  -> session.NewRedisStore / NewDBStore   按 SESSION_BACKEND 选择
  -> handler.New()       注入全部依赖
  -> http.Server{Handler: h.Router()}
`F`

## 几个刻意的决定

**数据库连不上就退出，对象存储连不上只告警。** 前者是硬依赖，没有它任何接口都无意义；
后者只影响图片，让服务先起来、由就绪探针如实报告，比直接拒绝启动更有用。

**连接池参数写死在代码里**（最大 20 / 空闲 5 / 生命周期 1 小时）。
它们是容量问题而非配置问题，等真有压力时再抽成环境变量。

**优雅关闭**：监听 `SIGINT` / `SIGTERM`，给在途请求 10 秒完成时间。
容器编排滚动更新时不会切断正在处理的请求。

**超时全部显式设置**（`ReadHeaderTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout`）。
Go 的 `http.Server` 默认**没有**超时，裸用会留下慢连接攻击面。

## 相关

- 上级：[`cmd/`](../README.md)
- 依赖装配：[`internal/handler/`](../../internal/handler/README.md)
