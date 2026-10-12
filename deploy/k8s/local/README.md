# deploy/k8s/local · 本地验证清单

> 知识树：[项目根](../../../README.md) › [`deploy/`](../../README.md) › [`k8s/`](../README.md) › **`local`**

| 文件 | 说明 |
|---|---|
| `pod.yaml` | 单 Pod，5 个容器 + Podman 自动创建的 infra 容器 |

`F`powershell
podman play kube deploy/k8s/local/pod.yaml
podman play kube --down deploy/k8s/local/pod.yaml

# 或走脚本（会先构建镜像）
powershell -ExecutionPolicy Bypass -File scripts\k8s-up.ps1 -Local
`F`

## 为什么是一个 Pod 而不是五个

Pod 内的容器**共享网络命名空间**，所以：

- API 用 `127.0.0.1` 连 Postgres / Redis / MinIO
- nginx 用 `127.0.0.1:8000` 反代 API
- 不需要 Service、不需要 DNS、不需要 CNI

这与集群里的区别仅体现在地址上（那里是 `travel-blog-api:8000` 这样的 Service DNS），
而配置的注入方式完全一致——都靠环境变量。

## 端口

| 容器 | hostPort | 用途 |
|---|---|---|
| web | 8080 | 从宿主机打开应用 |
| minio | 9000 | **必须暴露**：预签名 URL 里的地址要浏览器可达 |

Postgres 与 Redis 刻意不映射到宿主机，避免和 `scripts/dev-up.ps1` 那套开发容器抢端口。

## 跑之前先停掉开发容器

`podman play kube` 会真的去绑 9000 端口。如果 `scripts/dev-up.ps1` 起的
`travelblog-minio` 还在跑，Pod 会因为端口冲突而起不来（报
`address already in use`）。先 `scripts/dev-down.ps1`。

## 这份清单刻意省略的东西

安全上下文、资源限额、PDB、Service、Ingress——这些在 `base/` 里有，
但 `podman play kube` 不支持后两者，为保持"能跑"的一致性就没放进本地版本。
本地版本只验证**镜像能跑、环境变量正确、探针路径准确**这三件事。

## 相关

- 上级：[`k8s/`](../README.md) · 生产版：[`base/`](../base/README.md)
