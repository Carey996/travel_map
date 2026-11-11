# deploy/compose · 单机部署（最低成本）

> 知识树：[项目根](../../README.md) › [`deploy/`](../README.md) › **`compose`**

一台最便宜的云主机 + 一个域名，`docker compose up -d --build` 起全套。
**这是成本与省心之间性价比最好的一条路**——不需要集群，也不需要任何托管中间件
（真要零元，看下面表里的 Oracle 免费机型，代价是不稳）。

> 下面的命令都**在 `deploy/compose/` 目录里**执行：`.env` 就放在 compose 文件旁边，
> 这样不用去猜 compose 各版本「`.env` 从哪个目录加载」的差异。

| 文件 | 用途 |
|---|---|
| `docker-compose.yml` | 六个服务的编排：postgres / redis / minio / api / web / caddy |
| `Caddyfile` | 两个域名的反代 + 自动申请 HTTPS 证书 |
| `.env.example` | 域名与密码，复制成 `.env` 后改 |

## 成本账

| 项 | 选择 | 花费 |
|---|---|---|
| 计算 | 一台 2C2G 轻量应用服务器 | ¥38/月起（刊例价；长租与促销常见 ¥24~33） |
| Postgres / Redis / 对象存储 | 与 API 同机跑容器 | ¥0 |
| HTTPS 证书 | Caddy 自动申请 Let's Encrypt | ¥0 |
| 域名 | 自备 | ¥30~60/年 |
| **合计** | | **约 ¥40/月以内** |

相比之下：

- **托管 K8s**（ACK / TKE 集群）光控制面和节点就远超这个数，而本项目的 API 是无状态单进程，
  2 副本的收益主要是滚动更新不中断——个人站点用不上。
- **托管数据库 + 云对象存储**：起步阶段比自托管贵，等照片上了几百 GB、或者真需要异地容灾再换更合适。
  （好消息是换它们只改环境变量，代码一行不动。）
- **PaaS**（Railway / Zeabur）：不用碰服务器，但五个服务按内存计费，通常 $15~25/月（≈¥110~180），
  而且对象存储往往要另外接一个。

## 选哪家

| 方案 | 价格 | ICP 备案 | 大陆访问延迟 | 适合 |
|---|---|---|---|---|
| **腾讯云轻量 · 中国香港 2C2G** | ¥38/月（40GB SSD / 20Mbps / 0.5TB 流量） | **不需要** | 40~80ms | **首选**：省事与性价比的最佳平衡 |
| 腾讯云轻量 · 大陆 锐驰型 2C2G | ¥45/月（200Mbps / 无限流量） | 必须（1~3 周） | 10~30ms | 用户全在大陆、要最快 |
| 阿里云轻量 · 中国香港 2C2G | 约 ¥24~40/月（看促销） | 不需要 | 40~80ms | 腾讯云缺货时的等价替代 |
| Oracle Cloud Always Free（4C24G ARM） | ¥0 | 不需要 | 不稳（首尔 / 大阪 / 新加坡） | 先跑起来试水，别当正式站 |
| 海外廉价 VPS（RackNerd 等） | ¥7~15/月 | 不需要 | 150ms+、忽快忽慢 | 只有自己看 |

**结论：要长期稳定又不想备案，选腾讯云轻量香港 2C2G。**
如果用户全在大陆、并且愿意走备案，大陆锐驰型那 200Mbps 才叫真的快（香港只有 20Mbps）。

> 价格是 2026-10 查到的官方公告价 / 刊例价（[香港调价公告](https://cloud.tencent.com/announce/detail/2131)、
> [价格总览](https://cloud.tencent.com/document/product/1207/73452)）。促销与时长折扣变化很快，下单前以购买页为准。

两个地域的硬差别只有一条：**大陆地域的域名必须完成 ICP 备案才能用 80/443**，香港地域没有这个要求。
备案要走域名实名 + 服务商代办，个人 1~3 周，急不来。

## 一台 2C2G 装得下吗

| 组件 | 常驻内存（按各组件的常规占用估算） | 说明 |
|---|---|---|
| Postgres | 80~150 MB | 默认配置，未调 shared_buffers |
| MinIO | 150~250 MB | 只做对象存储 |
| API | 30~60 MB | 静态编译的 Go 二进制 |
| nginx（web） | 10~20 MB | 只服务静态文件 |
| Caddy | 20~30 MB | 含证书管理 |
| Redis | 5~15 MB | 只存会话 |
| 系统 + Docker | 150~250 MB | |
| **合计** | **约 0.5~0.8 GB** | 2C2G 有余量；4G 只是更从容 |

**磁盘**：40GB SSD 扣掉系统与镜像约剩 30GB，按「原图 5MB + 预览 300KB + 缩略图 50KB」估算，
约能放 5000 张。不够就挂一块云硬盘，或者把 MinIO 换云对象存储。

**带宽**：香港 2C2G 峰值 20Mbps、每月 0.5TB 出流量，图片全部走这台机器。
一个人上传/浏览绰绰有余；把相册分享给很多人看时要盯着流量包。

## 部署步骤

### 0. 前置

- 一台 Linux 云主机（2C2G 起，能装 Docker）
- 两个域名，都解析到这台机器：站点域名 + 对象存储域名（**不能是同一个**）
- 大陆地域才需要备案；香港地域不需要

### 1. 买机器，放行端口

控制台的安全组 / 防火墙里确认 **22、80、443** 都放行（443 尤其容易漏）。

### 2. 装 Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo systemctl enable --now docker
```

国内主机拉不到 `docker.io` 时，先给 dockerd 配镜像加速（`/etc/docker/daemon.json` 里的
`registry-mirrors`），再重启 docker。

### 3. 拿代码

```bash
git clone https://github.com/Carey996/travel_map.git
cd travel_map/deploy/compose
```

`github.com` 时通时不通的网络里，改用 `rsync -av --exclude node_modules ./ user@host:/srv/travel_map/`
从开发机直推更省时间。

### 4. 配置

```bash
cp .env.example .env
$EDITOR .env
```

`POSTGRES_PASSWORD` **只用字母数字**——它会被拼进 `DATABASE_URL`，带 `@ : /` 会把连接串切错。

### 5. 起

```bash
docker compose up -d --build
docker compose ps
```

第一次会构建两个镜像：后端多阶段（拉 `golang:1.27-alpine`）约 2~4 分钟，
前端（`npm ci` + `vite build`）约 2~3 分钟。之后重建只走缓存，很快。

### 6. 验证

```bash
# 存活探针 / 就绪探针（就绪会逐项报告 Postgres、Redis、对象存储哪个不通）
curl -fsS https://站点域名/healthz
curl -fsS https://站点域名/healthz/ready

# 证书是不是真的签下来了
curl -sI https://站点域名 | head -1
```

然后浏览器打开站点、注册账号、上传一张**带 GPS** 的照片，确认三件事：

1. 地图上出现大头针 → 预签名 PUT 直传成功
2. 刷新后图片还在 → 预签名 GET 链路正常
3. 开发者工具里 cookie `tb_session` 带 `Secure` → `COOKIE_SECURE=true` 生效

### 7. 备份

数据只在三个卷里：`pgdata`、`miniodata`、`caddy_data`。

```bash
# 元数据
docker compose exec -T postgres pg_dump -U travelblog travelblog | gzip > backup-$(date +%F).sql.gz

# 图片（卷名前缀就是 compose 里的 name: travel-blog）
docker run --rm -v travel-blog_miniodata:/data -v "$PWD":/backup alpine \
  tar czf /backup/minio-$(date +%F).tar.gz -C /data .
```

放进 cron 每天跑，再往异地放一份。云厂商的快照能当第二道保险，
但快照恢复是整机级操作，别拿它代替上面的逻辑备份。

### 8. 升级

```bash
git pull
docker compose up -d --build
```

前端静态资源带 30 天缓存、`index.html` 不缓存，刷新即可看到新版本。

## 安全响应头（已内置）

两个容器分工下发安全头，不必再手动加：

| 层 | 响应头 | 说明 |
|---|---|---|
| web 的 nginx | `X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy`、`Permissions-Policy`、`Content-Security-Policy` | 随页面与 `/api` 响应一起返回，见 `deploy/nginx/default.conf.template` |
| Caddy | `Strict-Transport-Security`（HSTS） | 只在 HTTPS 成立时有意义，因此放在 Caddy 层 |

上线后确认一遍：

```bash
curl -sI https://站点域名 | grep -iE 'strict-transport|x-content-type|x-frame|referrer|permissions|content-security'
curl -sI https://对象存储域名 | grep -i strict-transport
```

两条前提，缺一不可：

1. **必须走 Caddy 的 443**。两件事都依赖 HTTPS：HSTS 只对 HTTPS 生效；
   `COOKIE_SECURE=true` 的会话 Cookie 在纯 HTTP 下会被浏览器直接丢弃，登录态根本存不住。
2. **CSP 是按当前前端写的**（同源脚本、高德瓦片、Canvas 缩略图、预签名直传）。
   以后如果引入新的第三方源，浏览器 Console 会报被 CSP 拦截的 URL，按提示追加到
   `default.conf.template` 的 `img-src` / `connect-src` 即可；改完重建 web 镜像生效。

> ⚠️ 裸 IP + 纯 HTTP 的临时部署（不经过 Caddy）拿不到 HSTS，且只能把
> `COOKIE_SECURE` 关掉才能登录——那是调试形态，不要当成正式配置。

## 三个必须知道的坑

**1. 对象存储的「内外两套地址 + 两个 TLS 开关」。**
后端走容器内网 `http://minio:9000`，浏览器走 `https://对象存储域名`。
minio-go 要求「地址里的协议」与「secure 开关」严格一致，所以
`S3_USE_SSL=false` 和 `S3_PUBLIC_USE_SSL=true` 必须分别配对——配错时进程直接起不来，
报错会点名是哪个变量（`S3_PUBLIC_ENDPOINT_URL 里的协议与 S3_PUBLIC_USE_SSL 是否一致`）。

**2. Host 头不能改写。** 预签名 URL 把 Host 头签进了签名。
`Caddyfile` 里显式写了 `header_up Host {host}`；谁要是把它改成上游地址，
浏览器直传就会全部 403（而服务器日志里看不出原因）。

**3. 预签名直传要求对象存储配 CORS。**
MinIO 默认放行，所以这套编排开箱即用；换成阿里云 OSS / 腾讯云 COS / Cloudflare R2 时，
**必须**在桶上配 CORS（允许站点域名发 PUT/GET、暴露 ETag），否则浏览器预检会被拦掉。

## 想再省 / 再稳可以怎么改

| 想要 | 怎么做 |
|---|---|
| 再省一点内存 | 删掉 `redis` 服务，设 `SESSION_BACKEND=db`（会话落 Postgres，代价是撤销稍慢） |
| 图片别占这台机器带宽 | 换成云对象存储：只改 `S3_ENDPOINT_URL` / `S3_PUBLIC_ENDPOINT_URL` / 凭证 |
| 数据库更稳 | 换托管 Postgres：只改 `DATABASE_URL` |
| 扩到多副本 | `MIGRATE_ON_START=false` → `docker compose run --rm api -migrate-only` → `--scale api=2`；web 的上游要改成在 Docker DNS 里轮询 |
| ARM 机器（Oracle 免费机型） | 直接在机器上 `--build`，多阶段构建会自己产出 arm64；只有跨架构构建才需要 buildx |

## 与其余部署路径的关系

| 路径 | 场景 |
|---|---|
| **本目录** | 一台机器、最低成本、单副本 |
| [`../k8s/base/`](../k8s/base/README.md) | 真集群、多副本、滚动更新 |
| [`../k8s/local/`](../k8s/local/README.md) | 开发机上验证清单本身 |

两条路用**同一份镜像**，环境变量几乎一致，差异只在编排方式。
先在本地 Pod 里验证过清单再去买机器，比在服务器上试错便宜得多。

## 相关

- 上级：[`deploy/`](../README.md)
- 后端：[`api/README.md`](../../api/README.md) · 前端：[`web/README.md`](../../web/README.md)
