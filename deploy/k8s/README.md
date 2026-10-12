# deploy/k8s · Kubernetes 清单

> 知识树：[项目根](../../README.md) › [`deploy/`](../README.md) › **`k8s`**

| 目录 | 定位 |
|---|---|
| [`base/`](base/README.md) | **生产向**完整清单，kustomize 基础层 |
| [`local/`](local/README.md) | **验证向**单 Pod 清单，`podman play kube` 直接跑 |

## 两者的关系

它们不是"正式版与草稿"，而是同一套设计在两种编排器上的落地：

| | base/ | local/ |
|---|---|---|
| 编排单位 | 每个组件一个 Deployment/StatefulSet | 全部容器合成一个 Pod |
| 服务发现 | Service DNS | 127.0.0.1（共享网络命名空间） |
| 存储 | PVC（StatefulSet volumeClaimTemplates） | emptyDir |
| 安全上下文 | 全套加固（非 root、只读根、drop caps、seccomp） | 省略（本地图省事） |
| 探针 | 存活 / 就绪 / 启动 | 就绪为主 |
| 副本 | API 与 web 各 2 副本 + PDB | 各 1 |

**镜像、环境变量、探针路径是共用的**——所以 local/ 跑通，意味着 base/ 里
最容易被写错的那部分（镜像能不能跑、环境变量对不对、健康检查路径准不准）已经过验证。

## 为什么需要 local/

本机没有 kubectl / kind / minikube，而 `podman play kube` 能直接消费 K8s YAML
（支持 Pod / Deployment / DaemonSet / Job / PVC 这些 kind）。

与其交出一堆没跑过的 YAML，不如让其中可验证的部分真的跑起来。
**代价要说清楚**：Service、Ingress、HPA、PDB 这些 kind `podman play kube` 不支持，
因此它们**只经过静态审查，未经运行时验证**。

## 相关

- 上级：[`deploy/`](../README.md)
- 拉起脚本：[`scripts/k8s-up.ps1`](../../scripts/k8s-up.ps1)
