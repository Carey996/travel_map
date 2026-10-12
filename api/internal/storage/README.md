# `api/internal/storage` · 对象存储

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`storage`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `storage.go` | S3 兼容存储：建桶、预签名、删除 | `Storage`, `New()`, `EnsureBucket()`, `PresignPut()`, `PresignGet()`, `Delete()` |

基于 `minio-go`，兼容 MinIO / Cloudflare R2 / AWS S3。

## 核心设计：后端不接收图片字节流

这里只签发预签名 URL，浏览器拿到后直接 PUT 到对象存储。因此 API 进程
既不需要本地磁盘，也不会被大文件拖垮内存——这是「无状态」在文件上传场景下的具体落法。

`PresignPut` / `PresignGet` 是**纯本地计算**（不发网络请求），
所以可以在每个请求里现签，既不必在库里存易过期的 URL，也不需要任何缓存。

## 两个客户端

| 客户端 | 地址 | 用途 |
|---|---|---|
| `admin` | `S3_ENDPOINT_URL` | 建桶、删除对象 |
| `presign` | `BrowserEndpoint()` | 签发 URL |

两者相同时复用同一个客户端。分开是因为部署到远端时，容器内网地址
（如 `http://minio:9000`）与浏览器可达地址（如 `https://cdn.example.com`）
往往不同——**签出来的 URL 里的主机名必须让浏览器能访问**。

## 关于 Content-Type

Python 版把 `Content-Type` 一并签进了 URL（客户端必须发一模一样的头）。
Go 版改用 minio-go 的 `PresignedPutObject`，**不签这个头**——
未参与签名的头 S3 会忽略，客户端仍然照发（对象元数据里的类型因此正确），
但少了一个「头不匹配就 403」的失败模式。

## 删除是最终一致的

`Delete()` 走 MinIO 的批量删除接口，不存在的键会被忽略。
调用方（删除地点/照片）把删对象放在数据库事务提交之后：
即使删对象失败，也只是留下孤儿对象，不会破坏数据一致性。

## 相关

- 上级：[`internal/`](../README.md)
- 使用方：[`handler/photo.go`](../handler/README.md)
