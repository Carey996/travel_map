# travel-blog-api · 后端服务（Go + Gin）

旅行地图博客的无状态后端。负责鉴权、地点/照片元数据、旅行笔记，以及把图片直传所需的
预签名 URL 签发给浏览器——**它自己从不接收图片字节流，也不在本地磁盘写任何东西**。

- Web 框架：**Gin** v1.12
- ORM：**GORM** v2 + PostgreSQL（pgx 驱动）
- 会话：**Redis**（默认）或 Postgres
- 对象存储：任意 **S3 兼容**服务（MinIO / R2 / AWS S3），用 minio-go 签发预签名 URL
- 地名服务：**Photon**（默认）或 Nominatim
- 语言：**Go 1.24+**（本机 1.27）

---

## 快速开始

```bash
# 1. 依赖容器（Postgres / Redis / MinIO）
#    完整命令在仓库根 README「快速开始」；已经在跑就跳过这一步

# 2. 拉依赖（国内需先把模块代理指向 goproxy.cn）
go env -w GOPROXY=https://goproxy.cn,direct
go mod download

# 3. 配置：默认值即可跑通本地开发
cp ../.env.example ../.env

# 4. 运行（首次启动会自动建表）
go run ./cmd/server

# 或编译成单文件再跑
go build -o bin/server ./cmd/server && ./bin/server
```

- 存活探针：<http://127.0.0.1:8000/healthz>
- 就绪探针：<http://127.0.0.1:8000/healthz/ready>（检查 Postgres / Redis / 对象存储）
- 本服务没有 OpenAPI 文档页；路由全表见下方

**建表方式与 Python 版不同**：不再使用 Alembic，而是启动时调用 GORM 的 `AutoMigrate`
（见 `internal/db`）。它只增不删——会补齐缺失的表、列、索引，不会删除已有结构，
因此对已有库是安全且可重复执行的。表结构与 Python 版完全一致，可以直接对接同一个数据库。

---

## 知识树

每一级目录都有独立的 README，上层讲整体与约定，下层讲具体文件。

```
api/
├── cmd/server/            进程入口：装配依赖、启动 HTTP、优雅关闭
├── internal/              全部实现（internal 保证不被外部模块导入）
│   ├── config/            环境变量配置（含连接串归一化）
│   ├── apperr/            统一业务错误
│   ├── model/             GORM 模型与自定义字段类型
│   ├── db/                数据库连接与自动迁移
│   ├── dto/               HTTP 契约：请求校验与响应序列化
│   ├── geo/               球面距离计算
│   ├── geocode/           地名服务代理（Photon / Nominatim）
│   ├── storage/           S3 兼容对象存储与预签名
│   ├── session/           会话存储（Redis / Postgres）
│   ├── service/           业务逻辑：照片自动聚类
│   ├── middleware/        CORS、CSRF、请求体上限与限流
│   └── handler/           HTTP 层：路由、鉴权、各资源处理器
└── go.mod / go.sum
```

### 子目录索引

| 目录 | 一句话 |
|---|---|
| [`cmd/`](cmd/README.md) · [`cmd/server/`](cmd/server/README.md) | 进程入口与优雅关闭 |
| [`internal/`](internal/README.md) | 实现总览与依赖方向 |
| [`internal/config/`](internal/config/README.md) | 环境变量配置与连接串归一化 |
| [`internal/apperr/`](internal/apperr/README.md) | 统一错误形状 |
| [`internal/model/`](internal/model/README.md) | GORM 模型、UUID v7 主键、jsonb 字段 |
| [`internal/db/`](internal/db/README.md) | 连接池与 AutoMigrate |
| [`internal/dto/`](internal/dto/README.md) | 请求校验与响应契约 |
| [`internal/geo/`](internal/geo/README.md) | Haversine 距离 |
| [`internal/geocode/`](internal/geocode/README.md) | 地名服务代理 |
| [`internal/storage/`](internal/storage/README.md) | 预签名上传与读取 |
| [`internal/session/`](internal/session/README.md) | 会话存储两种实现 |
| [`internal/service/`](internal/service/README.md) | 照片自动聚类 |
| [`internal/middleware/`](internal/middleware/README.md) | CORS、CSRF、请求体上限与限流 |
| [`internal/handler/`](internal/handler/README.md) | 路由与各资源处理器 |

---

## 分层与依赖方向

```
cmd/server            组装：配置 -> 数据库 -> Redis -> 存储 -> 会话 -> 路由
  └─ handler          HTTP 层（依赖下面全部）
       └─ service     业务逻辑（聚类）
            └─ model  数据模型
                 └─ db / config / apperr / geo
```

依赖是单向的：`handler` 可以引用下层，下层**不允许**反过来引用 `handler`。
`dto` 只被 `handler` 使用；`storage` / `session` / `geocode` 是平级的服务包，互相独立。

---

## 「无状态」是怎么落地的

服务进程不持有任何业务状态，可以随时重启、可以横向扩展 N 个实例：

| 约束 | 做法 |
|---|---|
| 不用服务端 session 内存 | 会话 id 存 Redis（TTL 过期，撤销即时生效）；`session.Store` 接口可切到 Postgres |
| 不落本地磁盘 | 图片走预签名直传，API 进程完全不碰文件字节流 |
| 不接收图片 | EXIF 解析与缩略图生成都在浏览器完成，后端零图像依赖 |
| 不做进程内缓存 | 地名查询响应带 `Cache-Control`，缓存责任交给浏览器 |
| 请求自包含 | 鉴权状态全部来自请求携带的 Cookie + 共享存储 |
| 幂等 | `photos` 上的 `uq_photos_user_content` 唯一索引，重复上传直接短路 |

> ⚠️ **「无状态」不等于「无持久化」**：状态集中在 Postgres / Redis / S3，只是不在服务进程里。
> `cmd/server` 里的连接池与客户端属于进程级基础设施，不是业务状态。

---

## 请求生命周期

```
请求
 └─ gin.Recovery            panic 兜底
     └─ RequestLogger       slog 访问日志
         └─ CORS            白名单之外的来源拿不到 ACAO，浏览器自行拦截；预检在此短路
             └─ CSRF        变更类请求必须带 X-Requested-With
                 └─ 路由 + currentUser   解析 Cookie -> 查会话 -> 载入用户
                     └─ handler          解析/校验 -> 调用服务 -> 序列化响应
```

中间件按 `r.Use` 的注册顺序执行，所以 **CORS 必须注册在 CSRF 之前**——
否则跨站请求会先撞上 CSRF 校验，而不是在预检阶段就被拒绝。
`/api/v1` 分组还挂着请求体上限；登录/注册两条路由额外挂路由级限流（在 handler 之前短路），
两者都列在下面的「请求边界」里。

**错误契约**：任何失败都返回同一形状，前端只需处理一种结构。

```json
{ "code": "photo_not_found", "message": "照片不存在", "detail": {} }
```

### 请求边界

| 边界 | 值 | 超限行为 |
|---|---|---|
| 请求体大小 | `/api/v1` 下 **1 MiB**（`middleware.LimitBody`） | **413** `payload_too_large`，`detail.limit_bytes` 给上限 |
| 密码长度 | **≤ 72 字节**（bcrypt 硬上限，约 24 个汉字） | 注册 **400** `validation_error`；登录 **401** `invalid_credentials` |
| 登录限流 | **10 次 / 5 分钟 / IP**（`middleware.RateLimit`） | **429** `rate_limited` + `Retry-After` 头，`detail.retry_after_seconds` 给剩余秒数 |
| 注册限流 | **5 次 / 1 小时 / IP**（同上） | **429** `rate_limited` + `Retry-After`，同上 |
| 就绪探针 `checks` | 每项只有 `"ok"` / `"error"` | 真实错误只进服务端 slog 日志，**不回显**给未认证请求 |

第二条不是产品偏好而是密码算法的硬约束：bcrypt 拒绝超过 72 字节的输入，
若不提前拦截，注册会直接 500。`dto` 里的 `max=200` 按字符数校验，不能替代字节级检查。

登录/注册限流由一个**共享 Redis** 的固定窗口计数器实现（key = `tb:ratelimit:<scope>:<window>:<ip>`），
刻意不用进程内计数——后端要无状态、可多副本，进程内计数在每个实例各算各的会让限额放大 N 倍，
重启即清零。计数器故障时 **fail-open**（放行 + `slog.Warn`）：限流是附加防线，不能让 Redis
抖一下就把登录整体打挂。IP 取自 `c.ClientIP()`；`handler/router.go` 只把回环与私有网段
声明为可信代理，这样 nginx 注入的 `X-Forwarded-For` 会被采信，而公网直连伪造的 XFF 不会。
实现细节见 [`internal/middleware/README.md`](internal/middleware/README.md)。

最后一条的考虑：`/healthz/ready` 是未认证入口，错误原文里常带主机名、端口、bucket 名等
内部信息；对外只给 `ok`/`error`，排障看服务端 `slog.Warn`。

---

## 数据模型

| 表 | 说明 |
|---|---|
| `users` | 账号（bcrypt 哈希） |
| `places` | 地点：地图上的一个大头针，可以是城市也可以是景点；含摘要/标签/评分/日期范围 |
| `photos` | 照片元数据；二进制在对象存储，这里只存对象键与 EXIF 结果 |
| `notes` | 旅行笔记，挂在地点下，**一个地点可写多篇** |
| `trips` | 一次旅行；`places.trip_id` 指向它，地图据此画彩色框 |
| `trip_places` | 行程与地点的连接表，带顺序与第几天（留给后续行程页，当前未使用） |
| `shares` | 公开只读分享链接（表结构已就绪） |
| `user_sessions` | 会话记录，仅当 `SESSION_BACKEND=db` 时使用 |

几个刻意的选择：

- **主键用 UUID v7**（`model.NewID()`）：时间有序，无状态服务可各自本地生成。
- **所有列名显式声明**：GORM 的自动蛇形转换对 `SummaryMD` / `BodyMD` 这类连续大写缩写
  并不总是可靠（`MD` 不在 GORM 的 commonInitialisms 名单里），显式声明彻底消除隐患。
- **`photos.content_hash` 每用户唯一**（而非全局唯一）：两个用户拍到同一张图不应互相冲突。
- **`tags` 用 jsonb**：与 Python 版保持一致。
- **行政区划存四列**（`country` / `province` / `city` / `district`）：由前端驱动、
  复用一处 `/geo/reverse` 结果回填，地图按缩放级别取其中一层做分组标签。

---

## 路由总表

```
GET    /healthz                         存活探针
GET    /healthz/ready                   就绪探针（DB / Redis / 对象存储）

POST   /api/v1/auth/register            注册（自动登录）
POST   /api/v1/auth/login               登录
POST   /api/v1/auth/logout              登出当前设备
POST   /api/v1/auth/logout-all          登出所有设备
GET    /api/v1/auth/me                  当前用户

GET    /api/v1/places                   列出地点（bbox / 关键字过滤）
POST   /api/v1/places                   新建地点
GET    /api/v1/places/:id               地点详情
PATCH  /api/v1/places/:id               改名称/摘要/标签/评分
DELETE /api/v1/places/:id               删除（delete_photos=true 时连带照片）
POST   /api/v1/places/:id/assign        把照片移入该地点
POST   /api/v1/places/:id/merge-into    把该地点并入另一个地点

POST   /api/v1/photos/upload-url        签发预签名直传 URL（带幂等短路）
POST   /api/v1/photos                   登记照片元数据（自动归入地点）
GET    /api/v1/photos                   查询（place_id / unplaced / located / bbox / 日期）
PATCH  /api/v1/photos/:photo_id         改说明 / 改归属 / 修正坐标
DELETE /api/v1/photos/:photo_id         删除照片及其文件

GET    /api/v1/notes                    列出笔记
POST   /api/v1/notes                    写一篇笔记
PATCH  /api/v1/notes/:note_id           更新笔记
DELETE /api/v1/notes/:note_id           删除笔记

GET    /api/v1/trips                    列出旅行（带地点数 / 照片数）
POST   /api/v1/trips                    新建旅行
PATCH  /api/v1/trips/:id                改名 / 改摘要 / 改日期
DELETE /api/v1/trips/:id                删除旅行（地点保留）
POST   /api/v1/trips/:id/places         批量把地点加进旅行
DELETE /api/v1/trips/:id/places/:place_id  把某个地点移出旅行

GET    /api/v1/geo/reverse?lat=&lng=    经纬度 -> 地名
GET    /api/v1/geo/search?q=            地名搜索
```

---

## 配置

全部环境变量见仓库根目录的 `.env.example`。后端相关的重点：

| 变量 | 默认 | 说明 |
|---|---|---|
| `DATABASE_URL` | `postgres://travelblog:travelblog@127.0.0.1:5432/...` | 支持 `postgresql+asyncpg://` 等前缀，会自动归一化 |
| `REDIS_URL` | `redis://127.0.0.1:16379/0` | **注意不是 6379**，见下方排错 |
| `SESSION_BACKEND` | `redis` | 改成 `db` 即可零额外组件运行 |
| `COOKIE_SECURE` | `false` | **生产（HTTPS）必须设为 true** |
| `CORS_ORIGINS` | `http://localhost:5180,...` | 逗号分隔白名单 |
| `S3_ENDPOINT_URL` | `http://127.0.0.1:9000` | 后端调用地址 |
| `S3_PUBLIC_ENDPOINT_URL` | 空 | 浏览器直传地址；部署到远端时必填 |
| `GEOCODE_PROVIDER` | `photon` | 或 `nominatim` |

配置在 `internal/config` 里做了两处归一化，都是踩过坑之后加的：

1. **驱动前缀**：`postgresql+asyncpg://` → `postgres://`，让同一份 `.env` 兼容新旧两版。
2. **localhost → 127.0.0.1**：Windows 上 `localhost` 常被优先解析为 IPv6 `::1`，
   而容器端口只监听 IPv4，会表现为「容器明明在跑却连接被拒」。

---

## 常见任务

**加一个接口**：在 `internal/dto` 定义请求/响应 → 在 `internal/handler` 写处理器
→ 在 `internal/handler/router.go` 注册路由。业务规则放 `internal/service`，不要写进处理器。

**加一个字段**：改 `internal/model/model.go`（**必须显式写 `column:` 标签**）
→ 在 `internal/dto` 暴露 → 重启即自动迁移。

**换对象存储**：改 `internal/storage/storage.go`，它对上层只暴露
`EnsureBucket / PresignPut / PresignGet / Delete` 四个方法。

**换会话后端**：设置 `SESSION_BACKEND=db`，`cmd/server` 会自动装配 Postgres 实现。

---

## 排错

| 现象 | 原因与处理 |
|---|---|
| `/healthz/ready` 返回 503 | 逐项看 `checks`：`database` / `redis` / `storage` 哪个不是 ok。探针只回 `ok`/`error`，具体错误在服务端 `slog.Warn` 日志里（不回显给未认证请求） |
| 连接被拒，但 `podman ps` 显示容器 Up | **Windows 保留端口段**吞掉了映射。执行 `netsh interface ipv4 show excludedportrange protocol=tcp` 查看，换个主机端口重新创建容器（本项目 Redis 就因此从 6379 换到了 16379） |
| `cannot parse "postgresql+asyncpg://..."` | 连接串前缀不被 pgx 识别；已在 `internal/config` 里归一化，若仍报错请检查是否绕过了它 |
| `dial tcp [::1]:6379` | `localhost` 被解析成 IPv6；配置层已归一化为 127.0.0.1 |
| 照片挂不到地点下 | 检查 `internal/handler/photo.go` 里归属回写那一段——它的条件必须是无条件的（见文件内注释） |
| `go mod download` 卡住 | `go env -w GOPROXY=https://goproxy.cn,direct` |
| `go build` 报缺依赖 | `go mod tidy`（会自动从 goproxy.cn 拉取） |
