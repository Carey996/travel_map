# `web/src/components` · 视图组件

> 知识树：[项目根](../../../README.md) › [`web`](../../README.md) › [`src/`](../README.md) › **`components`**

全部单文件组件（`.vue`），用 `<script setup lang="ts">`。

## 组件

| 组件 | 职责 | 主要 Element Plus 组件 |
|---|---|---|
| `LoginView.vue` | 登录 / 注册，带表单校验 | `el-card`, `el-form`, `el-alert` |
| `TopBar.vue` | 地名搜索、新建地点、上传、底图切换、全览、语言、账号菜单 | `el-autocomplete`, `el-select`, `el-checkbox`, `el-radio-group`, `el-tooltip`, `el-avatar`, `el-dropdown` |
| `PlaceSidebar.vue` | 地点列表（含本地筛选）/ 时间线 / 未定位三个视图 | `el-radio-group`（分段切换）, `el-input`（筛选）, `el-scrollbar`, `el-image`, `el-skeleton`, `el-empty` |
| `MapCanvas.vue` | Leaflet 地图：按缩放归一分组、地点大头针、旅行框、坐标纠偏 | 无（Leaflet 直接使用） |
| `PlaceDrawer.vue` | 地点详情：改名、分类、评分、标签、Markdown、照片、笔记、合并与删除 | `el-drawer`, `el-input`, `el-select`, `el-rate`, `el-tag`, `el-date-picker`, `el-collapse`, `ElMessageBox` |
| `PhotoLightbox.vue` | 大图查看、EXIF 信息、说明编辑、定位与删除 | `el-dialog`, `el-image`, `el-descriptions` |
| `PhotoUploader.vue` | 上传入口，Element Plus 负责 UI，编排交给 `lib/upload.ts` | `el-upload`（自定义 `http-request`） |

## 排版约定

视觉上有几条固定约定，改样式前先看一眼：

| 约定 | 位置 |
|---|---|
| 顶栏是**单行** 58px 工具条（`--tb-topbar-h`）：左边品牌 + 固定宽搜索 + 主操作，右边工具组不压缩不换行 | `TopBar.vue` |
| 侧栏固定 336px（`--tb-sidebar-w`）：上部是分段切换 + 筛选，下部独立滚动 | `PlaceSidebar.vue` |
| 圆角、阴影、间距统一取 `--tb-*` 变量，组件里不写死数值 | `styles/base.css` |
| 地点抽屉：封面压暗 + 标题浮在图上，正文按 `section-head`（图标 + 标题 + 计数 + 操作）分区 | `PlaceDrawer.vue` |
| 灯箱：宽屏左右分栏（图 / 信息），窄屏自动堆叠 | `PhotoLightbox.vue` |

**一个 Element Plus 的坑**：`el-tooltip`、`el-autocomplete` 这类组件的根节点会被内部包装
元素替换掉，`<style scoped>` 的 `data-v-*` 属性**传不到它身上**——写成 `.my-class { width: … }`
的规则会静默失效（顶栏搜索框就踩过：宽度一直没生效，输入框撑满整行）。
解法是 `.父级 :deep(.my-class)`，或者像搜索框那样自己套一层原生 `div`。

## 组合关系

```
App.vue                       登录态分流 + 装配 + 全窗口拖拽上传
├── LoginView.vue             未登录时
└── 已登录时：
    ├── TopBar.vue            顶栏（内含 PhotoUploader）
    ├── PlaceSidebar.vue      左栏
    ├── MapCanvas.vue         地图主区
    ├── PlaceDrawer.vue       右侧抽屉（内含 PhotoUploader）
    └── PhotoLightbox.vue     模态灯箱
```

## 状态与事件的流向

**状态全部来自 Pinia**，组件之间不互相传业务数据。
跨组件的交互（例如「点地图新建地点」）由 `App.vue` 统一持有，通过 props / emits 下发给 `MapCanvas`。

**选中项存在 store 里**（`selectedPlaceId` / `selectedPhotoId`），
这样地图点击、侧栏点击、灯箱切换都只是改同一个值，不需要层层转发事件。

## MapCanvas 的几条硬约束

这个组件最容易写错，单独说明：

**Leaflet 实例不进响应式系统。** 用普通变量（`let map`）而非 `ref`——
Leaflet 对象内部有大量自引用与 DOM 引用，被 Vue 的 Proxy 包裹会出现难查的问题。

**缩放级别留在组件内部。** `zoomLevel` 只用于决定聚合到哪一级。
早期版本把它抛给父组件回写选中状态，结果点开地点面板后 `flyTo` 触发 `zoomend`，
用闭包里过期的 `null` 把面板立刻关掉了——这类 bug 非常隐蔽。

**坐标进出都要过 `lib/coords.ts`**，依据底图源的 `crs` 决定是否纠偏。

**地图上只有「地点」一种标记。** 单张照片不再单独画点：地点大头针的圆图就是该地点的
封面（第一张照片），再画一个照片点就会出现「同一张照片叠两层」。看单张照片走侧栏时间线、
地点抽屉或灯箱——这也是「以地点为主」这条设计的由来。

## MapCanvas 里的两条业务规则

**缩放归一。** `levelForZoom()` 把缩放切成五档：`z≤5` 国家、`6–7` 省、`8–9` 市、
`10–11` 区/县、`≥12` 单个地点。前四档不画单个地点，而是按
`places.country/province/city/district` 聚成一个气泡，气泡上的数字是该组的**照片总数**，
点击即 `fitBounds` 到这一组、并放大到下一档。
行政区划缺失时退化成按经纬度网格分桶（国 10°、省 2°、市 0.5°、区县 0.1°），
避免把相隔上千公里的无名地点并成一个点。

> 区/县这一档要看数据源脸色：Photon 的 `county` 有值时用它，但 **OSM 对国内很多地点
> 只到街道一级**（`district=龙门石窟街道`），这时显示的就是街道名。这是数据本身的上限，
> 不是分组逻辑的问题——只要那一带有多个地点，照样会聚成一个气泡。

**分组标记的样图。** 归一后的标记不是纯数字气泡，而是**前三张照片叠放**
（像相册 App 的分组卡片）：样图按地点轮转取——先每个地点一张，不够再取第二张，
避免三张全来自同一处。地点还没有照片时退回纯色气泡，数量徽标的颜色仍按层级区分。

**境外自动换底图。** 高德只覆盖大陆，所以 `MapCanvas` 盯着视图中心：出了大陆就换成
`esri-street`（全球、WGS-84），回到境内再换回用户选的高德样式。判断带 1° 滞回
（出去按原始范围、回来要求更深地进入境内），否则边界附近来回平移会让底图反复横跳。
注意这同时会换 CRS，因此换图时所有标记都要按新 `crs` 重新投影——现成的 `watch(provider)` 负责这件事。

**旅行框。** 每个 trip 取它名下地点的包围盒，向外留一点边（单点也有最小半径），
画成同色虚线矩形 + 一个常驻标签。颜色来自 `lib/trips.ts` 的 `tripColor(id)`，
**不存库**——服务端保持无状态，颜色纯粹是表现层的事。框的点击会上抛 `focus-trip` 事件。

## 相关

- 上级：[`src/`](../README.md)
- 同级：[`api/`](../api/README.md) · [`lib/`](../lib/README.md) · [`stores/`](../stores/README.md) · [`i18n/`](../i18n/README.md) · [`styles/`](../styles/README.md)
