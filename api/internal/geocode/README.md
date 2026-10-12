# `api/internal/geocode` · 地名服务

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`geocode`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `geocode.go` | 地名反查与搜索，可插拔后端 | `Geocoder`, `Result`, `SearchItem`, `New()` |

## 为什么由后端代理

浏览器直连这些服务无法声明合规的 User-Agent，也会遇到跨域问题。
后端只做转发，**不落盘缓存**——响应带 `Cache-Control`，缓存责任交给浏览器，
这样服务端保持无状态。

## 为什么默认 Photon

实测中国大陆网络下：

| 服务 | 可达性 |
|---|---|
| Nominatim（OSM 官方） | 不可达 |
| **Photon** | 可达，且**原生返回中文地名**（京都市 / 日本 / 黄浦区） |

切换只需改 `GEOCODE_PROVIDER`。

## 一个容易踩的坑

**Photon 一旦收到 `Accept-Language` 就会切换到翻译后的英文名**，
而我们要的正是本地语言地名。所以这个头只发给 Nominatim，Photon 不发——
代码里有注释标注，改动时别顺手统一。

## 节流

进程内有一个最小请求间隔（Photon 150ms / Nominatim 1050ms），
纯粹为遵守对方的使用条款。它是性能措施不是业务状态，
所以不影响「服务无状态」这个结论。

## 归一化输出

两个后端返回的原始结构差异很大，这里统一成 `Result` /
`SearchItem`，并额外提供：

- `Name`：`简称 · 国家`，用于地图大头针标签
- `Kind`：`city` / `spot` / `region` / `other`，前端用来选图标
- 无人区没有任何名称时，`Name` 退化为坐标，保证前端总有东西可显示

## 相关

- 上级：[`internal/`](../README.md)
- 使用方：[`handler/geo.go`](../handler/README.md)
