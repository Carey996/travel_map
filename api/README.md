# travel-blog-api · 后端服务

旅行地图博客的无状态后端。负责鉴权、地点/照片元数据、旅行笔记，以及把图片直传所需的
预签名 URL 签发给浏览器——**它自己从不接收图片字节流，也不在本地磁盘写任何东西**。

- 框架：**FastAPI** + Pydantic v2
- 数据：**SQLAlchemy 2.0 async** + asyncpg + Alembic
- 会话：**Redis**（默认）或 Postgres
- 对象存储：任意 **S3 兼容**服务（MinIO / R2 / AWS S3）
- 地名：**Photon**（默认）或 Nominatim
- Python：≥ 3.12（本机 3.14，全部依赖都有预编译轮子）

---

## 快速开始

```powershell
# 1. 依赖容器（Postgres / Redis / MinIO）
powershell -ExecutionPolicy Bypass -File ..\scripts\dev-up.ps1

# 2. 安装依赖（uv 会自动创建 .venv）
uv sync

# 3. 配置：默认值即可跑通本地开发
Copy-Item ..\.env.example ..\.env

# 4. 建表
.venv\Scripts\python.exe -m alembic upgrade head

# 5. 启动（开发模式带热重载）
.venv\Scripts\python.exe -m uvicorn app.main:app --reload --port 8000
```

- 交互式 API 文档：<http://127.0.0.1:8000/docs>
- OpenAPI 规范：<http://127.0.0.1:8000/openapi.json>
- 存活探针：<http://127.0.0.1:8000/healthz>
- 就绪探针：<http://127.0.0.1:8000/healthz/ready>（检查 Postgres / Redis / 对象存储）

---

## 知识树

每一级目录都有独立的 README：上层描述整体与约定，下层描述具体文件。
按依赖方向自底向上阅读即可。

```
api/
├── app/
│   ├── main.py             应用入口与 lifespan
│   ├── core/               配置、安全、错误、中间件等横切关注点
│   ├── db/                 ORM 模型与数据库引擎
│   ├── schemas/            Pydantic 请求/响应模型
│   ├── services/           对象存储、会话、地名、地点聚类
│   └── api/                依赖注入与 HTTP 路由
│       └── v1/             版本化业务路由
├── alembic/                数据库迁移
│   └── versions/           迁移脚本（自动生成）
└── tests/                  测试（尚未编写）
```

### 子目录索引

| 目录 | 一句话 |
|---|---|
| [`app/`](app/README.md) | 应用包：分层结构与单向依赖 |
| [`app/core/`](app/core/README.md) | 横切关注点：配置、安全、错误、CSRF、UUID v7、地理计算 |
| [`app/db/`](app/db/README.md) | 数据层：异步引擎、ORM 模型、迁移元数据 |
| [`app/schemas/`](app/schemas/README.md) | 接口契约：Pydantic 请求与响应模型 |
| [`app/services/`](app/services/README.md) | 领域服务：对象存储、地点聚类、会话存储、地名代理 |
| [`app/api/`](app/api/README.md) | HTTP 层：依赖注入与参数解析 |
| [`app/api/v1/`](app/api/v1/README.md) | 版本化业务路由（auth / places / photos / notes / geo） |
| [`alembic/`](alembic/README.md) | 迁移环境与常用命令 |
| [`alembic/versions/`](alembic/versions/README.md) | 迁移脚本清单 |
| [`tests/`](tests/README.md) | 测试现状与补测建议 |

---

### app/main.py

| 符号 | 说明 |
|---|---|
| `create_app()` | 组装应用：中间件 → 错误处理 → 路由 |
| `lifespan()` | 启动时创建引擎/Redis/存储客户端并确保 bucket 存在；关闭时统一释放 |

### app/core/ —— 横切关注点

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `config.py` | 全部配置来自环境变量；`CORS_ORIGINS` 支持逗号分隔 | `Settings`, `get_settings()` |
| `security.py` | 密码哈希与会话令牌 | `hash_password`, `verify_password`, `new_session_token`, `hash_token` |
| `errors.py` | 把任何异常归一成 `{code, message, detail}` | `ApiError`, `register_error_handlers` |
| `middleware.py` | CSRF 防护：变更类请求必须带 `X-Requested-With` | `CsrfHeaderMiddleware` |
| `ids.py` | 主键生成 | `new_id()`（UUID v7，时间有序） |
| `geo.py` | 球面距离 | `haversine_m` |
| `redis.py` | Redis 客户端工厂 | `create_redis` |

### app/db/ —— 数据层

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `session.py` | 异步引擎与会话工厂 | `Base`, `create_engine`, `create_session_factory` |
| `models.py` | 全部 ORM 模型 | `User`, `Place`, `Photo`, `Note`, `Trip`, `TripPlace`, `Share`, `UserSession` |
| `base.py` | 汇总导入供 Alembic autogenerate 使用 | `target_metadata` |

### app/services/ —— 领域服务

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `storage.py` | S3 兼容对象存储：签发预签名 URL、删除对象、启动时确保 bucket 存在 | `StorageBackend`(协议), `S3Storage` |
| `places.py` | 照片按 1.5 km 半径自动归入地点，并让地点继承照片日期范围 | `AUTO_ASSIGN_RADIUS_M`, `nearest_place`, `assign_photo` |
| `session_store.py` | 会话存储的两种实现，接口一致 | `SessionStore`, `RedisSessionStore`, `DbSessionStore`, `build_session_store` |
| `geocode.py` | 地名服务代理，可插拔；Photon 免 key 且原生中文 | `Geocoder`, `PhotonProvider`, `NominatimProvider` |

### app/api/ —— HTTP 层

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `deps.py` | 依赖注入：数据库会话、当前用户、存储、会话存储 | `get_db`, `get_current_user`, `CurrentUser`, `DbSession`, `Storage`, `Sessions` |
| `query.py` | 视口参数解析 | `parse_bbox` |
| `v1/auth.py` | 注册/登录/登出，会话写入 HttpOnly Cookie | `register`, `login`, `logout`, `logout_all`, `me` |
| `v1/places.py` | 地点 CRUD、照片归属、地点合并 | `list_places`, `create_place`, `assign_photos`, `merge_place` |
| `v1/photos.py` | 预签名签发、元数据登记、查询与维护 | `create_upload_url`, `create_photo`, `list_photos`, `update_photo`, `delete_photo` |
| `v1/notes.py` | 旅行笔记 CRUD | `list_notes`, `create_note`, `update_note`, `delete_note` |
| `v1/geo.py` | 地名代理接口，响应带 `Cache-Control` | `reverse`, `search` |
| `v1/router.py` | 汇总 v1 路由 | `api_router` |

### app/schemas/ —— 契约

| 文件 | 主要模型 |
|---|---|
| `auth.py` | `RegisterRequest`, `LoginRequest`, `UserOut` |
| `place.py` | `PlaceCreate`, `PlaceUpdate`, `PlaceOut`, `PlaceDetailOut`, `AssignPhotosRequest`, `MergePlaceRequest` |
| `photo.py` | `UploadUrlRequest/Response`, `SignedUpload`, `PhotoCreate`, `PhotoUpdate`, `PhotoOut`, `photo_to_out()` |
| `note.py` | `NoteCreate`, `NoteUpdate`, `NoteOut` |
| `geo.py` | `ReverseResult`, `SearchResult` |

---

## 「无状态」是怎么落地的

服务进程不持有任何业务状态，可以随时重启、可以横向扩展 N 个实例，任意实例都能处理任意请求：

| 约束 | 做法 |
|---|---|
| 不用服务端 session 内存 | 会话 id 存 Redis（TTL 过期，撤销即时生效）；`SessionStore` 接口可切到 Postgres |
| 不落本地磁盘 | 图片走预签名直传，API 进程完全不碰文件字节流 |
| 不接收图片 | EXIF 解析与缩略图生成都在浏览器完成，后端零图像依赖 |
| 不做进程内缓存 | 地名查询响应带 `Cache-Control`，缓存责任交给浏览器 |
| 请求自包含 | 鉴权状态全部来自请求携带的 Cookie + 共享存储 |
| 幂等 | `photos` 上的 `uq_photos_user_content` 唯一索引，重复上传直接短路 |

> ⚠️ **「无状态」不等于「无持久化」**：状态集中在 Postgres / Redis / S3，只是不在服务进程里。

---

## 请求生命周期

```
请求
 └─ CORSMiddleware          预检与跨站请求在这里被拦下（白名单之外直接拒绝）
     └─ CsrfHeaderMiddleware 变更类请求必须带 X-Requested-With
         └─ 路由 + Depends   解析 Cookie → 查会话 → 载入当前用户
             └─ 服务层        业务逻辑（存储 / 聚类 / 地名）
                 └─ 响应
```

中间件按「后添加者更靠外」生效，所以 CORS 必须加在 CSRF 之后，跨站请求才能在预检阶段就被拒绝。

**错误契约**：任何失败都返回同一形状，前端只需处理一种结构。

```json
{ "code": "photo_not_found", "message": "照片不存在", "detail": {} }
```

---

## 数据模型

| 表 | 说明 |
|---|---|
| `users` | 账号（bcrypt 哈希） |
| `places` | 地点：地图上的一个大头针，可以是城市也可以是景点；含摘要/标签/评分/日期范围 |
| `photos` | 照片元数据；二进制在对象存储，这里只存对象键与 EXIF 结果 |
| `notes` | 旅行笔记，挂在地点下，**一个地点可写多篇** |
| `trips` / `trip_places` | 把多个地点串成一次旅行（表结构已就绪，界面待启用） |
| `shares` | 公开只读分享链接（表结构已就绪） |
| `user_sessions` | 会话记录，仅当 `SESSION_BACKEND=db` 时使用 |

几个刻意的选择：

- **主键用 UUID v7**：时间有序，无状态服务可各自本地生成，无需向数据库申请序列。
- **`photos.content_hash` 每用户唯一**（而非全局唯一）：两个用户拍到同一张图不应互相冲突。
- **「旅行笔记」是独立表**而非 `places` 上的字段：这样一个地点可以按天写多篇。

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
GET    /api/v1/places/{id}              地点详情
PATCH  /api/v1/places/{id}              改名称/摘要/标签/评分
DELETE /api/v1/places/{id}              删除（可选择是否连带照片）
POST   /api/v1/places/{id}/assign       把照片移入该地点
POST   /api/v1/places/{id}/merge-into   把该地点并入另一个地点

POST   /api/v1/photos/upload-url        签发预签名直传 URL（带幂等短路）
POST   /api/v1/photos                   登记照片元数据（自动归入地点）
GET    /api/v1/photos                   查询（place_id / unplaced / bbox / 日期）
PATCH  /api/v1/photos/{id}              改说明 / 改归属 / 修正坐标
DELETE /api/v1/photos/{id}              删除照片及其文件

GET    /api/v1/notes                    列出笔记
POST   /api/v1/notes                    写一篇笔记
PATCH  /api/v1/notes/{id}               更新笔记
DELETE /api/v1/notes/{id}               删除笔记

GET    /api/v1/geo/reverse?lat=&lng=    经纬度 -> 地名
GET    /api/v1/geo/search?q=            地名搜索
```

---

## 配置

全部环境变量见仓库根目录的 `.env.example`。后端相关的重点：

| 变量 | 默认 | 说明 |
|---|---|---|
| `DATABASE_URL` | `postgresql+asyncpg://travelblog:travelblog@localhost:5432/travelblog` | |
| `REDIS_URL` | `redis://localhost:6379/0` | |
| `SESSION_BACKEND` | `redis` | 改成 `db` 即可零额外组件运行 |
| `SESSION_TTL_SECONDS` | `2592000` | 30 天 |
| `COOKIE_SECURE` | `false` | **生产（HTTPS）必须设为 true** |
| `CORS_ORIGINS` | `http://localhost:5173,…` | 逗号分隔白名单 |
| `S3_ENDPOINT_URL` | `http://localhost:9000` | 后端调用地址 |
| `S3_PUBLIC_ENDPOINT_URL` | 空 | 浏览器直传地址；部署到远端时必填 |
| `GEOCODE_PROVIDER` | `photon` | 或 `nominatim` |

---

## 数据库迁移

修改 `app/db/models.py` 之后：

```powershell
# 生成迁移（自动对比模型与库的差异）
.venv\Scripts\python.exe -m alembic revision --autogenerate -m "描述本次改动"

# 应用
.venv\Scripts\python.exe -m alembic upgrade head

# 回滚一步
.venv\Scripts\python.exe -m alembic downgrade -1
```

> 开发期若迁移写乱且无数据需要保留，可以 `alembic downgrade base` → 删除 `alembic/versions/*.py` → 重新 autogenerate。

---

## 常见任务

**加一个接口**：在 `app/schemas/` 定义请求/响应模型 → 在 `app/api/v1/` 新建或修改路由模块 →
若新增模块，在 `app/api/v1/router.py` 里 `include_router`。

**加一个字段**：改 `app/db/models.py` → 在 `app/schemas/` 暴露出去 → 生成迁移。

**换对象存储**：实现 `StorageBackend` 协议（`ensure_ready` / `presign_put` / `presign_get` / `delete`），
在 `lifespan` 里替换 `app.state.storage` 即可，业务代码无需改动。

**换会话后端**：设置 `SESSION_BACKEND=db`，`build_session_store` 会自动切换到 Postgres 实现。

---

## 排错

| 现象 | 原因与处理 |
|---|---|
| `/healthz/ready` 返回 503 | 逐项看 `checks`：`database` / `redis` / `storage` 哪个不是 ok |
| 注册返回 500 且日志有 `ConnectionRefusedError` | 依赖容器没起来，跑 `..\scripts\dev-up.ps1` |
| `alembic` 报 `UnicodeDecodeError` | `alembic.ini` 里不能写中文注释（alembic 用系统区域编码读配置） |
| 上传成功但图片打不开 | `S3_PUBLIC_ENDPOINT_URL` 必须是**浏览器可达**的地址 |
| 地名一直是坐标 | 地名服务不可达；Photon 也失败时可以手动改名，或换 `GEOCODE_PROVIDER` |
