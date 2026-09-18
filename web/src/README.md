# `web/src` · 前端源码

> 知识树：[项目根](../../README.md) › [`web`](../README.md) › **`src`**

## 分层

```
main.ts        装配：Pinia / vue-i18n / Element Plus + 全部图标 + 主题跟随
  │
App.vue        外壳：登录态分流、装配、全窗口拖拽上传
  ├─ components/   视图（7 个 .vue）
  │    └─ 读写 stores/
  ├─ stores/       Pinia：auth + gallery
  │    └─ 调用 api/client.ts，复杂流程交给 lib/
  ├─ lib/          与框架无关的纯逻辑（坐标 / 元数据 / 上传 / 渲染）
  ├─ api/          唯一的后端出口
  ├─ i18n/         词条与语言切换
  └─ styles/       全局样式
```

依赖是单向的：`components → stores → api`，而 `lib` 被 `stores` 与 `components` 共用，
自身不依赖任何上层。**`lib/` 不导入 Vue 或 Element Plus**——这条约束让它可以直接单测，
也是它能从上一版 React 实现零改动复用的原因。

## 子目录

| 目录 | 一句话 |
|---|---|
| [`components/`](components/README.md) | 视图组件：顶栏、侧栏、地图、抽屉、灯箱、上传、登录 |
| [`stores/`](stores/README.md) | Pinia 状态：登录态与相册主状态 |
| [`api/`](api/README.md) | 唯一的后端出口（CSRF 头、Cookie、统一错误、全部类型） |
| [`lib/`](lib/README.md) | 与框架无关的纯逻辑：坐标纠偏、EXIF、上传编排、Markdown |
| [`i18n/`](i18n/README.md) | 国际化与 Element Plus 语言联动 |
| [`styles/`](styles/README.md) | 全局样式与 Leaflet 地图样式 |

## 顶层文件

| 文件 | 职责 |
|---|---|
| `main.ts` | `createApp` → 注册 Pinia、i18n、Element Plus 与全部图标；把 `prefers-color-scheme` 同步到 `<html class="dark">` |
| `App.vue` | 未登录显示 `LoginView`，已登录装配顶栏 / 侧栏 / 地图 / 抽屉 / 灯箱；持有「点选建点」这一跨组件状态；全窗口拖拽上传与批量进度 |
| `env.d.ts` | Vite 客户端类型与 `ImportMetaEnv` 声明 |

## 相关

- 上级：[`web/`](../README.md)
