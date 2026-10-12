# deploy/k8s/local · 本地验证清单

> 知识树：[项目根](../../../README.md) › [`deploy/`](../../README.md) › [`k8s/`](../README.md) › **`local`**

| 文件 | 说明 |
|---|---|
| `pod.yaml` | 单 Pod，5 个容器 + Podman 自动创建的 infra 容器 |

```bash
# 先构建镜像（后端交叉编译 + 前端容器内构建）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -C api -trimpath -ldflags '-s -w' -o bin/server-linux ./cmd/server
podman build -f deploy/containerfiles/Containerfile.api.local -t localhost/travel-blog-api:dev .
podman build -f deploy/containerfiles/Containerfile.web       -t localhost/travel-blog-web:dev .

# 拉起 / 停止
podman play kube --replace deploy/k8s/local/pod.yaml
podman play kube --down    deploy/k8s/local/pod.yaml
```

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

Postgres 与 Redis 刻意不映射到宿主机，避免和 README「快速开始」里那套开发容器抢端口。

## 跑之前先停掉开发容器

`podman play kube` 会真的去绑 9000 端口。如果开发用的 `travelblog-minio` 还在跑，
Pod 会因为端口冲突而起不来（报 `address already in use`）。先把开发容器停掉：

```bash
podman rm -f travelblog-postgres travelblog-redis travelblog-minio
```

## 这份清单刻意省略的东西

安全上下文、资源限额、PDB、Service、Ingress——这些在 `base/` 里有，
但 `podman play kube` 不支持后两者，为保持"能跑"的一致性就没放进本地版本。
本地版本只验证**镜像能跑、环境变量正确、探针路径准确**这三件事。

## 相关

- 上级：[`k8s/`](../README.md) · 生产版：[`base/`](../base/README.md)
