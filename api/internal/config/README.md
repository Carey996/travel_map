# `api/internal/config` · 配置

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`config`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `config.go` | 从环境变量读取全部配置；两处连接串归一化 | `Config`, `Load()`, `normalizeDatabaseURL()`, `normalizeLocalhost()` |

加载顺序：`.env`（从当前目录向上最多 4 层查找）→ 环境变量 → 代码内默认值。
**每个字段都有默认值**，所以本地开发复制一份 `.env.example` 就能跑通。

## 两处归一化都是踩过坑才加的

**1. 驱动前缀。** Python 版用 SQLAlchemy，连接串写成 `postgresql+asyncpg://...`；
Go 的 pgx 只认 `postgres://`。归一化后同一份 `.env` 在新旧两版之间通用，
迁移期间不用改配置。

**2. `localhost` → `127.0.0.1`。** Windows 上 `localhost` 常被优先解析成 IPv6 的 `::1`，
而 Podman / Docker 的端口映射只监听 IPv4。症状是「`podman ps` 显示容器 Up，
连接却报 actively refused」——很容易误判成容器没起。

## 派生方法

| 方法 | 说明 |
|---|---|
| `IsDev()` | 环境是否为 dev |
| `BrowserEndpoint()` | 浏览器直传用的对象存储地址（`S3_PUBLIC_ENDPOINT_URL` 为空时回落到 `S3_ENDPOINT_URL`） |
| `GeoBaseURL()` | 当前地名服务的基地址（按 `GEOCODE_PROVIDER` 选择） |

## 约定

**业务代码不要直接读 `os.Getenv`**，一律走 `Config`。这样默认值、归一化、
类型转换都只有一处，也便于日后换成别的配置源。

`Load()` 会校验 `SESSION_BACKEND` 只能是 `redis` 或 `db`，非法值直接返回错误——
配置错误应该在启动时就炸掉，而不是等到半夜某个请求失败。

## 相关

- 上级：[`internal/`](../README.md)
