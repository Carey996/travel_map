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
├── deploy/README.md                   部署与容器化（K8s 清单 / 镜像 / nginx）
│   ├── containerfiles/README.md       后端与前端的镜像定义
│   ├── nginx/README.md                SPA 路由与 /api 反代策略
│   └── k8s/README.md                  清单总览：生产向与验证向
│       ├── base/README.md             生产清单逐个文件说明
│       └── local/README.md            单 Pod 验证清单
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
| [`deploy/README.md`](deploy/README.md) | 部署：容器镜像、K8s 清单（生产 + 本地验证）、nginx 同源反代 |

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

## 快速开始（本地开发）

> 下面全部是**通用 bash**，Linux / macOS / WSL / Git Bash 都能直接跑；把 `podman` 换成 `docker` 即可，
> 两者的子命令在这里完全一致。仓库里**不含任何平台专属脚本**——每一步都以命令的形式写在文档里，
> 复制粘贴就能跑通，不需要先信任一个 `.ps1`。

### 前置条件

- **Go ≥ 1.24**（本机 1.27）
- **Node.js ≥ 20**
- **Podman** 或 **Docker**（用来跑 Postgres / Redis / MinIO）
- **bash**（Windows 下用 WSL 或 Git Bash）

### 1. 启动依赖容器

```bash
# 网络与数据卷：已存在会报错，忽略即可（幂等）
podman network create travelblog-net 2>/dev/null || true
for v in pgdata redisdata miniodata; do podman volume create "travelblog-$v" 2>/dev/null || true; done

# 每段都是「已存在就 start，不存在才创建」——Podman 机器重启后容器会停，
# 重跑这一整段即可，不会报 name already in use
podman start travelblog-postgres >/dev/null 2>&1 || \
  podman run -d --restart=unless-stopped --name travelblog-postgres --network travelblog-net \
    -p 5432:5432 \
    -e POSTGRES_USER=travelblog -e POSTGRES_PASSWORD=travelblog -e POSTGRES_DB=travelblog \
    -v travelblog-pgdata:/var/lib/postgresql/data \
    docker.io/library/postgres:16-alpine

# 主机端口用 16379 而不是 6379，原因见文末「排错备忘」
podman start travelblog-redis >/dev/null 2>&1 || \
  podman run -d --restart=unless-stopped --name travelblog-redis --network travelblog-net \
    -p 16379:6379 \
    -v travelblog-redisdata:/data \
    docker.io/library/redis:7-alpine redis-server --appendonly yes

podman start travelblog-minio >/dev/null 2>&1 || \
  podman run -d --restart=unless-stopped --name travelblog-minio --network travelblog-net \
    -p 9000:9000 -p 9001:9001 \
    -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
    -v travelblog-miniodata:/data \
    quay.io/minio/minio:latest server /data --console-address :9001
```

| 容器 | 地址 | 用途 |
|---|---|---|
| travelblog-postgres | localhost:5432 | 元数据 |
| travelblog-redis | localhost:16379 | 会话 |
| travelblog-minio | localhost:9000（控制台 9001） | 图片对象存储 |

等它们真正可用再往下走（容器 Up ≠ 服务可用）：

```bash
until podman exec travelblog-postgres pg_isready -U travelblog -d travelblog >/dev/null 2>&1; do sleep 1; done
until podman exec travelblog-redis redis-cli ping >/dev/null 2>&1; do sleep 1; done
until curl -fsS http://127.0.0.1:9000/minio/health/live >/dev/null 2>&1; do sleep 1; done
echo '依赖已就绪'
```

### 2. 安装依赖、准备配置

```bash
cp .env.example .env            # 默认值即可跑通本地开发

cd api
go env -w GOPROXY=https://goproxy.cn,direct   # 官方模块代理在国内不可达时才需要
go mod download

cd ../web
npm install
```

MinIO 的 bucket 由后端启动时自动创建；**建表也不需要单独一步**——
后端启动时会执行 GORM 的 `AutoMigrate`（只增不删，可重复执行）。

### 3. 启动

**方式 A · 两个终端**，各自看日志，适合开发：

```bash
# 终端 1：后端 API
cd api && go run ./cmd/server        # http://127.0.0.1:8000

# 终端 2：前端 Vite
cd web && npm run dev                # http://127.0.0.1:5173
```

**方式 B · 一条命令后台起**，日志与 PID 落在 `.logs/`（已在 .gitignore 里）：

```bash
mkdir -p .logs
# go env GOEXE 在 Windows 上是 .exe、在 Linux / macOS 上是空串，两个平台都能直接 exec
( cd api && go build -o "bin/server$(go env GOEXE)" ./cmd/server && exec "./bin/server$(go env GOEXE)" ) \
  >.logs/api.log 2>&1 & echo $! >.logs/api.pid
# 前端直接跑 vite 的可执行入口而不是 npm run dev：npm 会把 vite 当子进程启动，
# 记下的是 npm 的 PID，kill 掉它之后 vite 仍然占着 5173
( cd web && exec node ./node_modules/vite/bin/vite.js ) >.logs/web.log 2>&1 & echo $! >.logs/web.pid

# 停止
kill "$(cat .logs/api.pid)" "$(cat .logs/web.pid)" 2>/dev/null
```

打开 <http://127.0.0.1:5173>，注册一个账号即可开始。
存活探针 <http://127.0.0.1:8000/healthz>，就绪探针 <http://127.0.0.1:8000/healthz/ready>
（会逐项报告 Postgres / Redis / 对象存储哪个不通）。
后端**没有 OpenAPI 文档页**，路由全表见下方「API 一览」。

### 4. 停止

```bash
# 只停前后端（方式 B）
kill "$(cat .logs/api.pid)" "$(cat .logs/web.pid)" 2>/dev/null

# 停依赖容器；数据在具名卷里，删容器不丢数据
podman rm -f travelblog-postgres travelblog-redis travelblog-minio
# 连数据一起清掉
podman volume rm travelblog-pgdata travelblog-redisdata travelblog-miniodata
```

---

## 目录结构

```
travel_blog/
  api/                        # Go + Gin 后端
    cmd/server/               # 进程入口：装配与优雅关闭
    internal/
      config/                 # 环境变量配置与连接串归一化
      model/  db/             # GORM 模型与连接、AutoMigrate
      dto/                    # 请求校验与响应契约
      handler/                # 路由与各资源处理器
      middleware/             # CORS 与 CSRF
      service/                # 照片自动聚类
      storage/ session/ geocode/   # 对象存储、会话、地名
    go.mod
  web/                        # Vue 3 + Element Plus 前端
    src/
      components/             # MapCanvas / PlaceSidebar / PlaceDrawer / PhotoLightbox ...
      lib/                    # 坐标纠偏、瓦片源、EXIF、上传编排、Markdown
      api/client.ts           # 唯一的 API 出口（含 CSRF 头与 Cookie 约定）
      i18n/                   # 中英双语
  deploy/                     # 镜像定义、nginx 配置、K8s 清单
  .env.example
```

---

## 用 Kubernetes 部署

两套编排，**同一份镜像**：

```bash
# 1. 构建镜像
#    后端走「本机交叉编译 + 极简运行时镜像」：国内网络拉 golang 基础镜像经常超时，
#    而 Go 交叉编译只需要一条环境变量，产物与多阶段构建完全等价（同为静态 linux/amd64、
#    同一个 alpine 运行时、同样以非 root 运行）。想用标准多阶段构建，换成 Containerfile.api 即可。
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -C api -trimpath -ldflags '-s -w' -o bin/server-linux ./cmd/server

podman build -f deploy/containerfiles/Containerfile.api.local -t localhost/travel-blog-api:dev .
podman build -f deploy/containerfiles/Containerfile.web       -t localhost/travel-blog-web:dev .

# 2A. 本地验证：不需要集群，podman play kube 直接跑起同一个 Pod
#     ⚠️ Pod 里的 minio 也要绑 9000，先停掉「快速开始」那套开发容器，
#        否则报 listen tcp4 :9000: bind: address already in use
podman rm -f travelblog-postgres travelblog-redis travelblog-minio
#     起失败过就先清干净，不然会留下一组 Created 状态的容器
podman play kube --down    deploy/k8s/local/pod.yaml
podman play kube --replace deploy/k8s/local/pod.yaml     # -> http://127.0.0.1:8080

# 2B. 生产：kustomize 应用到集群（先跑迁移 Job，再应用其余资源）
kubectl apply -f deploy/k8s/base/migrate-job.yaml
kubectl -n travel-blog wait --for=condition=complete job/travel-blog-migrate --timeout=180s
kubectl apply -k deploy/k8s/base
```

三个和后端无状态直接相关的地方：

- **API 是 2 副本的**，迁移因此收敛到独立 Job（`MIGRATE_ON_START=false`）；
  代码里还加了 Postgres 咨询锁，重跑或多副本并发也不会出错
- **`readOnlyRootFilesystem: true`** —— 服务不写任何文件，根文件系统可以完全只读
- **存活探针不碰依赖**（`/healthz`），就绪探针才检查（`/healthz/ready`）；
  数据库挂了不该重启进程，而应摘掉流量

细节见 [`deploy/README.md`](deploy/README.md)。

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

所有错误统一为 `{"code": "...", "message": "...", "detail": {}}`。

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
  ```bash
  podman machine ssh "sed -i 's|docker.mirrors.ustc.edu.cn|docker.m.daocloud.io|' /etc/containers/registries.conf"
  podman machine stop; podman machine start   # 必须重启，服务进程会缓存旧配置
  ```
- **容器 Up 但连接被拒**：端口落在 Windows 保留段。用
  `netsh interface ipv4 show excludedportrange protocol=tcp` 查看，
  换一个主机端口重建容器（本项目 Redis 就因为 6379 落在保留段而改用了 16379）
- **`dial tcp [::1]:...`**：`localhost` 被解析成 IPv6，而容器端口只监听 IPv4；
  后端配置层已统一归一化为 `127.0.0.1`
- **`podman play kube` 报 `address already in use`**：上面那套开发容器还占着 9000；
  先 `podman rm -f travelblog-minio`。另外 `play kube` 只认全名，
  清单里的镜像必须写成 `localhost/travel-blog-api:dev` 这种形式
- **`podman compose` 不可用**：它需要 `docker-compose` 或 `podman-compose` 提供者；
  本项目上面的命令直接用 `podman run`，不依赖它
