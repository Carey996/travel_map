# `api/internal/geo` · 地理计算

> 知识树：[项目根](../../../README.md) › [`api`](../../README.md) › [`internal`](../README.md) › **`geo`**

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `geo.go` | 球面距离 | `Haversine()`, `EarthRadiusM` |

`Haversine(lat1, lng1, lat2, lng2)` 返回两点之间的球面距离（米），
是照片自动聚类的判据（1.5 km 半径）。

刻意没有引入任何地理库：公式本身只有十行，而第三方库通常捆绑整套坐标系转换，
与本项目的需求（只要距离）不匹配。

## 相关

- 上级：[`internal/`](../README.md)
- 使用方：[`service/`](../service/README.md)
