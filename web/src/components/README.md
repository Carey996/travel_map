# `web/src/components` · 视图组件

> 知识树：[项目根](../../../README.md) › [`web`](../../README.md) › [`src/`](../README.md) › **`components`**

全部单文件组件（`.vue`），用 `<script setup lang="ts">`。

## 组件

| 组件 | 职责 | 主要 Element Plus 组件 |
|---|---|---|
| `LoginView.vue` | 登录 / 注册，带表单校验 | `el-card`, `el-form`, `el-alert` |
| `TopBar.vue` | 地名搜索、新建地点、上传、底图切换、照片点开关、全览、语言、账号菜单 | `el-autocomplete`, `el-select`, `el-checkbox`, `el-radio-group`, `el-dropdown` |
| `PlaceSidebar.vue` | 地点列表 / 时间线 / 未定位三个视图 | `el-tabs`, `el-scrollbar`, `el-badge`, `el-image`, `el-skeleton`, `el-empty` |
| `MapCanvas.vue` | Leaflet 地图：聚合大头针、照片点、坐标纠偏、相机控制 | 无（Leaflet 直接使用） |
| `PlaceDrawer.vue` | 地点详情：改名、分类、评分、标签、Markdown、照片、笔记、合并与删除 | `el-drawer`, `el-input`, `el-select`, `el-rate`, `el-tag`, `el-date-picker`, `el-collapse`, `ElMessageBox` |
| `PhotoLightbox.vue` | 大图查看、EXIF 信息、说明编辑、定位与删除 | `el-dialog`, `el-image`, `el-descriptions` |
| `PhotoUploader.vue` | 上传入口，Element Plus 负责 UI，编排交给 `lib/upload.ts` | `el-upload`（自定义 `http-request`） |

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

**缩放级别留在组件内部。** `zoomLevel` 只用于决定「要不要显示单张照片点」。
早期版本把它抛给父组件回写选中状态，结果点开地点面板后 `flyTo` 触发 `zoomend`，
用闭包里过期的 `null` 把面板立刻关掉了——这类 bug 非常隐蔽。

**坐标进出都要过 `lib/coords.ts`**，依据底图源的 `crs` 决定是否纠偏。

**在高缩放级别渲染照片点时有上限**（400 个），避免一次插入过多 DOM 节点。

## 相关

- 上级：[`src/`](../README.md)
- 同级：[`api/`](../api/README.md) · [`lib/`](../lib/README.md) · [`stores/`](../stores/README.md) · [`i18n/`](../i18n/README.md) · [`styles/`](../styles/README.md)
