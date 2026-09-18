# `web/src/i18n` · 国际化

> 知识树：[项目根](../../../README.md) › [`web`](../../README.md) › [`src/`](../README.md) › **`i18n`**

| 条目 | 职责 |
|---|---|
| `index.ts` | 创建 i18n 实例、语言探测与持久化、`setLang()` |
| [`locales/`](locales/README.md) | 中英词条 |

## index.ts

| 导出 | 说明 |
|---|---|
| `i18n` | vue-i18n 实例，在 `main.ts` 里 `app.use(i18n)` |
| `SUPPORTED_LANGS` | `['zh', 'en']`，语言切换 UI 遍历它渲染 |
| `setLang(lang)` | 写入 localStorage、切换语言、同步 `<html lang>` |
| `currentLang()` | 读取当前语言 |

配置要点：

- `legacy: false` —— Composition API 模式，组件里用 `useI18n()`
- `globalInjection: true` —— 模板里可直接用 `$t`
- 语言探测顺序：**localStorage → 浏览器语言 → 英文兜底**

## 与 Element Plus 的联动

Element Plus 的分页、日期选择器等组件自带文案，需要单独喂 locale。
`App.vue` 用 `<el-config-provider :locale="epLocale">` 包住整棵树，
`epLocale` 依据当前语言在 `element-plus/es/locale/lang/zh-cn` 与 `/en` 之间切换。
**改语言时这两处会同时生效**，不会出现「界面中文、日期选择器英文」的割裂。

## 相关

- 上级：[`src/`](../README.md)
- 子目录：[`locales/`](locales/README.md)
