# deploy · 部署与容器化

> 知识树：[项目根](../README.md) › **`deploy`**

把两个应用打包成容器、并用 Kubernetes 清单拉起的全部材料。
**不含任何业务代码。**

`F`
deploy/
├── containerfiles/   容器镜像定义（后端 / 前端）
├── nginx/            前端容器的 nginx 配置模板（含 /api 反代）
└── k8s/
    ├── base/         生产向的完整清单（kustomize 基础层）
    └── local/        单 Pod 版本，用 podman 直接跑起来验证
`F`

## 两种拉起方式

| 方式 | 命令 | 适用 |
|---|---|---|
| **Kubernetes** | `scripts/k8s-up.ps1`（有 kubectl 时走 kustomize） | 生产 / 集群 |
| **本地单 Pod** | `scripts/k8s-up.ps1 -Local`（`podman play kube`） | 开发机验证，不需要集群 |

两条路径**用的是同一份镜像**，差异只在编排方式。
本地单 Pod 的存在意义是：让清单里用到的镜像、环境变量、探针真的被跑过一遍，
而不是交出一堆没验证过的 YAML。

## 快速开始

`F`powershell
# 1. 构建镜像（后端交叉编译 + 前端容器内构建）
powershell -ExecutionPolicy Bypass -File scripts\k8s-build.ps1

# 2A. 本地跑（不需要集群）
powershell -ExecutionPolicy Bypass -File scripts\k8s-up.ps1 -Local
#     -> 打开 http://127.0.0.1:8080

# 2B. 部署到集群
kubectl apply -f deploy/k8s/base/migrate-job.yaml
kubectl -n travel-blog wait --for=condition=complete job/travel-blog-migrate --timeout=180s
kubectl apply -k deploy/k8s/base

# 3. 停止本地 Pod
powershell -ExecutionPolicy Bypass -File scripts\k8s-up.ps1 -Down
`F`

## 三条值得先知道的设计

**1. 迁移不放在 API 里跑。** API 是 2 副本的，N 个 Pod 同时执行 DDL 会互相冲突。
因此 `MIGRATE_ON_START=false`，迁移收敛到 `migrate-job.yaml` 这个 Job。
代码里还额外加了 Postgres 咨询锁（见 `api/internal/db`），
即便有人手滑把开关打开、或 Job 被重复 apply，也不会出问题。

**2. 前端与 API 同源。** 前端的 nginx 把 `/api` 与 `/healthz` 反代到后端。
这样会话 Cookie 的 `SameSite=Lax` 正常工作，不必放宽到 `None`，
也不需要给前端在 CORS 白名单里额外开一个来源。

**3. MinIO 必须单独对外暴露。** 图片是浏览器拿着预签名 URL 直传/直取的，
不经过我们的 API。`S3_PUBLIC_ENDPOINT_URL` 必须与对外域名一致，
否则浏览器会因为地址不可达而加载不出图片。

## 子目录

| 目录 | 一句话 |
|---|---|
| [`containerfiles/`](containerfiles/README.md) | 后端与前端的镜像定义（含本地快速路径） |
| [`nginx/`](nginx/README.md) | nginx 配置模板：SPA 路由 + API 反代 + 缓存策略 |
| [`k8s/`](k8s/README.md) | 清单总览与两种编排方式的关系 |
| [`k8s/base/`](k8s/base/README.md) | 生产清单逐个文件说明 |
| [`k8s/local/`](k8s/local/README.md) | 单 Pod 验证清单 |

## 相关

- 上级：[项目根 README](../README.md)
- 后端：[`api/README.md`](../api/README.md) · 前端：[`web/README.md`](../web/README.md)
