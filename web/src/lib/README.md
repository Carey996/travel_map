# `web/src/lib` · 与框架无关的纯逻辑

> 知识树：[项目根](../../../README.md) › [`web`](../../README.md) › [`src/`](../README.md) › **`lib`**

**这一层不导入任何 Vue 或 Element Plus API。** 它只处理数据：坐标换算、EXIF 解析、
上传编排、Markdown 渲染、展示格式化。

这样划分的收益是实打实的：从上一版 React 实现迁移到 Vue 时，整个 `lib/` **零改动复用**；
它也天然适合单独写单测（唯一的外部依赖是浏览器 API 与后端接口）。

## 文件

| 文件 | 职责 | 关键导出 |
|---|---|---|
| `coords.ts` | WGS-84 与 GCJ-02 坐标纠偏 | `outOfChina`, `wgs84ToGcj02`, `gcj02ToWgs84`, `toMapCoords`, `fromMapCoords` |
| `tiles.ts` | 底图源定义（含坐标系标记与实测结论）；**当前只启用高德** | `TILE_PROVIDERS`, `ENABLED_TILE_PROVIDER_IDS`, `AVAILABLE_TILE_PROVIDERS`, `DEFAULT_TILE_PROVIDER`, `findProvider` |
| `exif.ts` | 浏览器端 EXIF 解析、Canvas 缩略图生成、sha256 | `readExif`, `deriveImages`, `sha256Hex` |
| `upload.ts` | 单张照片的上传编排与带并发上限的批量上传 | `uploadPhotoFile`, `uploadMany`, `UploadOutcome` |
| `markdown.ts` | 极简 Markdown 渲染（先整体转义，再做有限替换） | `renderMarkdown` |
| `format.ts` | 日期、体积、坐标等展示格式化 | `formatDate`, `formatDateTime`, `formatDayLabel`, `formatBytes`, `formatCoords` |

## 最需要注意的一条：坐标

照片 EXIF 里的 GPS 是 **WGS-84**（国际标准），而国内底图（高德 / 腾讯）用 **GCJ-02**，
两者在中国大陆相差 **100~600 米**。直接把 WGS-84 的点画在 GCJ-02 底图上会有肉眼可见的偏移。

本项目约定：

- **数据库与组件间传递的坐标永远是 WGS-84**
- 只在渲染到 GCJ-02 底图的那一刻转换：`toMapCoords()`
- 地图上点到的坐标先转回 WGS-84 再抛出：`fromMapCoords()`
- 转换依据是底图源定义里的 `crs` 字段

`gcj02ToWgs84` 用迭代反解而非简单做差，精度更好。

**当前底图是高德（GCJ-02），这条链路每次渲染都会真的走到**——少调一次 `toMapCoords()`，
大头针就会偏移 100~600 米，而且在低缩放下根本看不出来。

## 上传编排的五步

```
解析 EXIF -> 生成缩略图 -> 算 sha256 -> 取预签名 URL -> 直传 -> 登记元数据
                                       |
                                       └─ 若哈希已存在：直接返回，整个上传短路
```

`uploadMany` 默认 3 路并发——同时解码太多图片会把浏览器卡死。

## 相关

- 上级：[`src/`](../README.md)
- 同级：[`api/`](../api/README.md) · [`stores/`](../stores/README.md) · [`components/`](../components/README.md) · [`i18n/`](../i18n/README.md) · [`styles/`](../styles/README.md)
