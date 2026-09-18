# `web/src/styles` · 全局样式

> 知识树：[项目根](../../../README.md) › [`web`](../../README.md) › [`src/`](../README.md) › **`styles`**

组件自身的样式写在各自的 `<style scoped>` 里；这里只放**全局**与**跨越组件边界**的部分。

| 文件 | 内容 |
|---|---|
| `base.css` | 重置、应用骨架布局、侧栏、照片网格、Markdown 渲染、标签编辑、登录页、上传浮层、响应式 |
| `map.css` | Leaflet 容器与控件、地点大头针、单张照片点、地图上的空状态 |

## 颜色一律引用 Element Plus 变量

自定义样式不定义自己的调色板，全部用 `--el-*`：

```css
.sidebar {
  background: var(--el-bg-color);
  border-right: 1px solid var(--el-border-color-lighter);
}

.pin-count {
  background: var(--el-color-primary);
}
```

带来两个好处：**暗色主题自动生效**（Element Plus 的暗色变量挂在 `html.dark` 上，
`main.ts` 负责跟随系统切换），以及主题色调整时无需改任何自定义样式。

## 为什么 Leaflet 样式单独放

Leaflet 有自己的类名体系（`.leaflet-*`）与定位逻辑，不参与 Element Plus 的变量系统。
大头针是我们用 `L.divIcon` 自己生成的 HTML，样式必须覆盖 Leaflet 默认的
`.leaflet-div-icon` 外观——所以 `.pin-wrap` 与 `.photo-dot-wrap` 上有 `!important` 重置。

## 大头针的两态

| 类名 | 含义 |
|---|---|
| `.pin` | 常规地点大头针（封面缩略图 + 数量徽标 + 名称标签） |
| `.pin.is-selected` | 当前选中：放大、主题色描边、标签变实心 |
| `.pin.is-draft` | 尚未命名（等待反向地理编码）：虚线边框 |

## 相关

- 上级：[`src/`](../README.md)
- 同级：[`api/`](../api/README.md) · [`lib/`](../lib/README.md) · [`stores/`](../stores/README.md) · [`components/`](../components/README.md) · [`i18n/`](../i18n/README.md)
