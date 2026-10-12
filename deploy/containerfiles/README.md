# deploy/containerfiles · 镜像定义

> 知识树：[项目根](../../README.md) › [`deploy/`](../README.md) › **`containerfiles`**

构建上下文一律是**仓库根目录**，因为前后端镜像都要跨目录取文件。

| 文件 | 用途 |
|---|---|
| `Containerfile.api` | 后端：多阶段从源码构建（标准路径，CI/集群用） |
| `Containerfile.api.local` | 后端：本机交叉编译后拷入（本地快速路径） |
| `Containerfile.web` | 前端：node 构建 SPA，nginx 提供服务 |

## 为什么后端有两个 Containerfile

不是重复，是取舍。`Containerfile.api` 从 `golang:1.27-alpine` 构建，是标准做法；
但那个基础镜像约 300 MB，**在国内网络下拉它经常超时**（实测 300 秒仍未完成）。

`Containerfile.api.local` 换了个思路：Go 交叉编译只需要 `GOOS=linux` 一条环境变量，
本机有 Go 工具链就够。**产物完全等价**——同为静态 Linux amd64 二进制、同一个 alpine 运行时、
同样以 UID 10001 非 root 运行。

```powershell
# 标准路径
podman build -f deploy/containerfiles/Containerfile.api -t travel-blog-api:dev .

# 本地快速路径（需先 scripts/k8s-build.ps1 生成 api/bin/server-linux）
podman build -f deploy/containerfiles/Containerfile.api.local -t travel-blog-api:dev .
```

两者都内置了 `CGO_ENABLED=0` / `-trimpath` / `-ldflags "-s -w"`，成品约 86 MB。

## ⚠️ 构建上下文必须排除 node_modules

仓库根有 `.containerignore`（同时放了份 `.dockerignore` 兼容 Docker），
**这不是可有可无的优化**：

Windows 上的 `web/node_modules/.bin/vue-tsc` 没有可执行位，
一旦被 `COPY web/ ./` 覆盖掉容器内正常安装的那份，前端构建会直接报
`sh: vue-tsc: Permission denied`。此外上下文体积会从几 MB 涨到几百 MB。

## 相关

- 上级：[`deploy/`](../README.md) · 构建脚本：[`scripts/k8s-build.ps1`](../../scripts/k8s-build.ps1)
