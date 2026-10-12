# deploy/k8s/base · 生产向清单

> 知识树：[项目根](../../../README.md) › [`deploy/`](../../README.md) › [`k8s/`](../README.md) › **`base`**

用 `kubectl apply -k deploy/k8s/base` 部署。`kustomization.yaml` 是入口。

| 文件 | kind | 说明 |
|---|---|---|
| `namespace.yaml` | Namespace | `travel-blog` |
| `configmap.yaml` | ConfigMap | 非敏感配置 |
| `secret.example.yaml` | Secret | **示例值，真实部署请另行创建** |
| `postgres.yaml` | Service + StatefulSet | 元数据；生产建议换托管库 |
| `redis.yaml` | Service + Deployment | 会话；**刻意不持久化** |
| `minio.yaml` | Service + StatefulSet | 对象存储；生产建议换 S3/R2 |
| `migrate-job.yaml` | Job | 结构迁移，先于 API 跑 |
| `api.yaml` | Service + Deployment + PDB | 后端，2 副本 |
| `web.yaml` | Service + Deployment | 前端 nginx，2 副本 |
| `ingress.yaml` | Ingress | 对外入口，web 与 MinIO 各一条规则 |

## 部署顺序不是可有可无的

```powershell
kubectl apply -f deploy/k8s/base/migrate-job.yaml
kubectl -n travel-blog wait --for=condition=complete job/travel-blog-migrate --timeout=180s
kubectl apply -k deploy/k8s/base
```

`kubectl apply -k` 是一次性提交所有资源的，它**不保证顺序**。
如果 API 先于迁移就绪，它会在表不存在时报错——就绪探针会一直失败，直到 Job 完成。
先单独 apply 迁移 Job 并等它结束，可以完全避免这段抖动。

## 几个刻意的决定

**Redis 用 Deployment 且不持久化。** 会话是临时数据，丢了重新登录即可；
无状态也让调度器可以随时把它挪走。

**API 的存活探针不碰依赖**（`/healthz`），就绪探针才检查依赖（`/healthz/ready`）。
数据库挂掉时不该把 API 进程重启掉——重启也没用，还会放大故障；
正确做法是让它留在原地、被摘掉流量、等依赖恢复。

**`readOnlyRootFilesystem: true`。** 这是「无状态」带来的直接好处：
服务不写任何文件，根文件系统可以完全只读。web 容器需要写的几个 nginx 目录
用 emptyDir 顶上去。

**`preStop: sleep 5` 配合 `terminationGracePeriodSeconds: 30`。**
滚动更新时先让 Endpoint 从 Service 摘除，再让进程退出；
进程自身还会再等 10 秒处理完在途请求。

**`PodDisruptionBudget` minAvailable: 1。** 节点维护时不会把 2 个副本一起驱逐掉。

## 生产环境应当替换的部分

| 组件 | 现在 | 建议 |
|---|---|---|
| Postgres | StatefulSet + PVC | 托管数据库（RDS / Cloud SQL / Neon），只改 `DATABASE_URL` |
| MinIO | StatefulSet + PVC | S3 / R2，只改 `S3_ENDPOINT_URL` 与 `S3_PUBLIC_ENDPOINT_URL` |
| Secret | 明文示例 | Sealed Secrets / External Secrets |
| Ingress | `*.travel-blog.local` 假域名 | 真实域名 + cert-manager，并把 `COOKIE_SECURE` 置 true |

替换数据库与对象存储**不需要改一行代码**——后端只用 SQL 与 S3 协议，
没有供应商特有调用。

## 相关

- 上级：[`k8s/`](../README.md) · 验证版：[`local/`](../local/README.md)
