# 旅行地图博客 · Travel Map Blog

一个以**地图为首页**的旅行记录应用：上传旅途中拍摄的照片，自动读取 EXIF 里的 GPS 坐标，
在真实地图上生成地点大头针；选择任意城市或地点，为它写下旅行笔记。

前端 Vue 3 + Element Plus，后端 Go + Gin，**前后端完全分离**，后端保持无状态。

---

## 知识树

**每一级目录都有自己的 README**，自底向上构成一棵可导航的知识树：
上层描述整体职责与约定，下层描述具体文件与符号，每份文档都带面包屑与同级跳转。

```
README.md                              项目总览（本文件）
├── api/README.md                      后端总览（Go + Gin）
│   ├── cmd/README.md                  可执行入口
│   │   └── server/README.md           进程装配与优雅关闭
│   └── internal/README.md             实现总览与依赖方向
│       ├── config/README.md           环境变量与连接串归一化
│       ├── apperr/README.md           统一错误形状
│       ├── model/README.md            GORM 模型、UUID v7、jsonb 字段
│       ├── db/README.md               连接池与 AutoMigrate
│       ├── dto/README.md              请求校验与响应契约
│       ├── geo/README.md              Haversine 距离
│       ├── geocode/README.md          Photon / Nominatim 代理
│       ├── storage/README.md          S3 预签名
│       ├── session/README.md          Redis / Postgres 会话存储
│       ├── service/README.md          照片自动聚类
│       ├── middleware/README.md       CORS 与 CSRF
│       └── handler/README.md          路由与各资源处理器
└── web/README.md                      前端总览
    └── src/README.md                  前端源码分层与依赖方向
        ├── components/README.md       7 个视图组件的职责与硬约束
        ├── stores/README.md           Pinia 状态与「全量拉取 + 派生」的取舍
        ├── api/README.md              唯一的后端出口
        ├── lib/README.md              与框架无关的纯逻辑
        ├── i18n/README.md             国际化与 Element Plus 语言联动
        │   └── locales/README.md      中英词条与单大括号插值约定
        └── styles/README.md           全局样式与 Leaflet 地图样式
```

### 文档索引

| 文档 | 内容 |
|---|---|
| **本文件** | 项目总览、架构、快速开始、关键设计决策、底图选型实测数据 |
| [`api/README.md`](api/README.md) | 后端：知识树导航、分层与依赖方向、请求生命周期、数据模型、无状态约束 |
| [`web/README.md`](web/README.md) | 前端：组件索引、状态流、坐标与 i18n 约定、上传管线、构建产物 |

---
## 功能

- **地图主页** —— 照片以其缩略图为大头针落在地图上，带数量徽标；缩放到 14 级以上显示单张照片的位置
- **拖拽上传** —— 浏览器端解析 EXIF（GPS / 拍摄时间 / 机型 / 光圈 / ISO）并生成缩略图，直传对象存储
- **自动聚类** —— 1.5 km 内的照片自动归入同一地点，超出则新建；地点自动继承照片的日期范围
- **选点建点** —— 搜索城市（中文地名），或直接在地图上点选坐标新建地点
- **旅行笔记** —— 每个地点可写多篇 Markdown 笔记，按天记录；地点本身还带摘要、评分、标签
- **时间线视图** —— 全部照片按天分组浏览，灯箱支持键盘左右切换、逐张设置位置
- **中英双语** —— 界面可实时切换
- **多用户** —— 注册登录，会话走 HttpOnly Cookie + Redis

---

## 架构

```
┌──────────────────────┐        ┌──────────────────────┐
│ 浏览器 (Vue 3 SPA)    │        │  对象存储 (MinIO/S3)  │
│  · Leaflet 地图       │──PUT──▶│  原图 / 预览 / 缩略图  │
│  · exifr 解析 EXIF    │  预签名 │                      │
│  · Canvas 生成缩略图   │  直传   └──────────────────────┘
└──────────┬───────────┘
           │ /api/v1  (JSON)
┌──────────▼───────────┐        ┌──────────────────────┐
│  Gin（无状态）         │───────▶│  Postgres（元数据）    │
│  · 不存会话/不落盘      │        └──────────────────────┘
│  · 不接收图片字节流     │        ┌──────────────────────┐
└──────────────────────┘───────▶│  Redis（会话）         │
                                 └──────────────────────┘
```

### 「无状态」具体指什么

服务进程不持有任何业务状态，可以随时重启、可以横向扩展 N 个实例，任意实例都能处理任意请求：

| 约束 | 做法 |
|---|---|
| 不用服务端 session 内存 | 会话 id 存 **Redis**（TTL 过期，撤销即时生效），接口抽象成 `SessionStore`，可一行配置切换到 Postgres |
| 不用 Cookie 明文令牌 | Cookie 里是**不透明随机串**，服务端只存会话；令牌本身从不落库 |
| 不落本地磁盘 | 图片走**预签名直传**，API 进程完全不碰文件字节流 |
| 不接收图片 | EXIF 解析与缩略图生成都在**浏览器**完成，后端零图像依赖 |
| 不做进程内缓存 | 地名查询响应带 `Cache-Control`，缓存责任交给浏览器 |
| 请求自包含 | 鉴权状态全部来自请求携带的 Cookie + 共享存储，不依赖上一次请求 |

> ⚠️ 一个必须说清的推论：**「无状态」不等于「无持久化」**。状态集中在 Postgres / Redis / S3，
> 只是不在服务进程里。

---

## 快速开始

### 前置条件

- **Python ≥ 3.12**（本机为 3.14，全部依赖都有预编译轮子）与 [uv](https://docs.astral.sh/uv/)
- **Node.js ≥ 20**
- **Podman**（或 Docker）用于跑 Postgres / Redis / MinIO

### 1. 启动依赖容器

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev-up.ps1
```

会创建并启动三个容器：

| 容器 | 地址 | 用途 |
|---|---|---|
| travelblog-postgres | localhost:5432 | 元数据 |
| travelblog-redis | localhost:6379 | 会话 |
| travelblog-minio | localhost:9000（控制台 9001） | 图片对象存储 |

### 2. 安装依赖

```powershell
cd api; go mod download   # 后端
cd ..\web; npm install   # 前端
```

### 3. 配置环境变量

```powershell
Copy-Item .env.example .env
```

默认值即可直接跑通本地开发（MinIO 的 bucket 由后端启动时自动创建）。

### 4. 拉取后端依赖

```powershell
cd api
go env -w GOPROXY=https://goproxy.cn,direct   # 官方模块代理在国内不可达
go mod download
```

建表不需要单独一步：后端启动时会执行 GORM 的 `AutoMigrate`（只增不删，可重复执行）。

### 5. 一键启动

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev.ps1
```

打开 <http://127.0.0.1:5173>，注册一个账号即可开始。API 文档在 <http://127.0.0.1:8000/docs>。

停止：`scripts\stop-dev.ps1`（只停前后端）或 `scripts\dev-down.ps1`（停依赖容器）。

---

## 目录结构

```
travel_blog/
  api/                        # Go + Gin 后端
    cmd/server/               # 进程入口：装配与优雅关闭
    internal/
      config/                 # 环境变量配置
      model/  db/             # GORM 模型与连接
      dto/                    # 请求校验与响应契约
      handler/                # 路由与各资源处理器
      middleware/            # CORS 与 CSRF
      service/                # 照片自动聚类
      storage/ session/ geocode/   # 对象存储、会话、地名
    go.mod
  web/                        # Vue 3 + Element Plus 前端
    src/
      components/             # MapCanvas / PlaceSidebar / PlaceDrawer / PhotoLightbox ...
      lib/                    # 坐标纠偏、瓦片源、EXIF、上传编排、Markdown
      api/client.ts           # 唯一的 API 出口（含 CSRF 头与 Cookie 约定）
      i18n/                   # 中英双语
  scripts/                    # Podman 与开发流程脚本
  .env.example
```

---

## 关键设计决策

**为什么 EXIF 解析和缩略图放在浏览器？**
后端因此不需要 Pillow 之类的图像依赖，也不必接收和转存图片字节流——这是「预签名直传」能成立的前提。
代价是浏览器解不了 HEIC（部分 iPhone 原图），这类文件仍可上传，但生成不出缩略图，界面会明确提示。

**为什么地图坐标要纠偏？**
照片 EXIF 里的 GPS 是 **WGS-84**（国际标准），而国内底图（高德 / 腾讯）用 **GCJ-02**（俗称火星坐标），
两者在国内相差 100~600 米。约定是：**数据库永远只存 WGS-84**，只在渲染到 GCJ-02 底图的那一刻转换
（见 `web/src/lib/coords.ts`）。这样坐标数据始终干净、可移植。

**为什么用预签名直传？**
`POST /photos/upload-url` 签发一次性 PUT URL，浏览器直接传到对象存储。后端只登记元数据，
因此不会被大文件拖垮内存，也不依赖任何本地磁盘。

**上传是幂等的。**
客户端先算文件的 sha256，`upload-url` 若发现该用户已有同一哈希的记录会直接返回，
连上传都省掉；即便重复提交元数据也只会返回同一条记录。

**「旅行笔记」是独立表而非地点上的一个字段。**
这样一个地点可以按天写多篇（"在京都的第三天"），才像旅行日志。

---

## 底图与地名服务

这部分经过实测选型，因为**不同网络环境下的可达性差异极大**。以下是本机（中国大陆网络）的实测结果：

| 瓦片源 | 京都 z13 | 上海 z13 | 冰岛 z13 | 结论 |
|---|---|---|---|---|
| **Esri 街道**（默认） | 33 KB | 46 KB | 26 KB | ✅ 全球可用 |
| **Esri 影像** | 25 KB | 22 KB | 22 KB | ✅ 全球可用 |
| 高德矢量 | 179 B ⚠️ | 24 KB | 179 B ⚠️ | 仅中国大陆有数据 |
| 腾讯 | 169 B ⚠️ | 169 B ⚠️ | 169 B ⚠️ | 本网络下全空白 |
| OSM 官方 | 不可达 | 不可达 | 不可达 | 本网络下连不上 |

> 关于"空白"：单色瓦片 PNG 只有约 179 字节，而真实瓦片是几万字节——这是判断瓦片源是否可用的可靠信号。

因此默认底图是 **Esri（全球覆盖、WGS-84 无需纠偏）**，界面上可随时切到高德（会自动启用 GCJ-02 纠偏）
或 OpenStreetMap。地名服务默认用 **Photon**（OSM 数据、免 key、原生返回中文地名），
可通过 `GEOCODE_PROVIDER` 切到 Nominatim。

---

## API 一览

```
GET    /healthz                         存活探针
GET    /healthz/ready                   就绪探针（DB / Redis / 对象存储）

POST   /api/v1/auth/register            注册（自动登录）
POST   /api/v1/auth/login               登录
POST   /api/v1/auth/logout              登出当前设备
POST   /api/v1/auth/logout-all          登出所有设备
GET    /api/v1/auth/me                  当前用户

GET    /api/v1/places                   列出地点（支持 bbox / 关键字）
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

所有错误统一为 `{"code": "...", "message": "...", "detail": {...}}`。

---

## 环境变量

见 `.env.example`。几个值得注意的：

| 变量 | 说明 |
|---|---|
| `SESSION_BACKEND` | `redis`（默认，支持即时撤销）或 `db`（零额外组件的回落方案） |
| `GEOCODE_PROVIDER` | `photon`（默认，中文友好）或 `nominatim` |
| `S3_PUBLIC_ENDPOINT_URL` | 部署到远端时填浏览器可达的对象存储地址 |
| `CORS_ORIGINS` | 前端来源白名单，逗号分隔；与 CSRF 防护共同生效 |
| `COOKIE_SECURE` | 生产环境（HTTPS）必须设为 `true` |

---

## 安全设计

因为选了 Cookie 承载会话（比 localStorage 抗 XSS），就必然引入 CSRF 面，因此做了三层防护：

1. Cookie 设 `SameSite=Lax`，跨站表单 POST 不会携带会话
2. CORS 严格白名单，跨站 fetch 带自定义头会触发预检并被拒绝
3. 所有变更类请求必须带 `X-Requested-With: XMLHttpRequest`——HTML 表单设置不了自定义头

前端在 `web/src/api/client.ts` 里统一加这个头，不必逐处记得。

---

## 已知限制

- **浏览器无法解码 HEIC**（部分 iPhone 原图）——可上传、有定位，但缩略图要等后续加后端兜底
- Element Plus 为全量引入，element-plus 分块约 354 KB（gzip）；需要瘦身可改按需引入，见 `web/README.md`
- 上传是**逐张**登记的（每张需要自己的预签名 URL），超大相册会慢一些
- 行程（把多个地点串成一次旅行）与公开只读分享页尚未实现，数据模型已预留（`trips` / `trip_places` / `shares` 表）

---

## 排错备忘

这几个是搭建时真实踩到并解决的坑：

- **Podman 拉不到镜像**：默认配置把 `docker.io` 指向了早已下线的 `docker.mirrors.ustc.edu.cn`。
  实测可用的替代是 `docker.m.daocloud.io`：
  ```powershell
  podman machine ssh "sed -i 's|docker.mirrors.ustc.edu.cn|docker.m.daocloud.io|' /etc/containers/registries.conf"
  podman machine stop; podman machine start   # 必须重启，服务进程会缓存旧配置
  ```
- **PowerShell 脚本中文乱码报错**：Windows PowerShell 5.1 按 ANSI 读取 `.ps1`，
  脚本必须保存为 **UTF-8 with BOM**（`scripts/` 下的脚本已处理）
- **容器 Up 但连接被拒**：端口落在 Windows 保留段。用 `netsh interface ipv4 show excludedportrange protocol=tcp` 查看，换一个主机端口重建容器（本项目 Redis 就因为 6379 落在保留段而改用了 16379）
- **`dial tcp [::1]:...`**：`localhost` 被解析成 IPv6，而容器端口只监听 IPv4；后端配置层已统一归一化为 `127.0.0.1`
- **Compose provider 缺失**：`podman compose` 需要 `docker-compose` 或 `podman-compose`，
  本项目的脚本直接用 `podman run`，不依赖它
