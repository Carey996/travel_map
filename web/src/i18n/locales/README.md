# `web/src/i18n/locales` · 词条

> 知识树：[项目根](../../../../README.md) › [`web`](../../../README.md) › [`src/`](../../README.md) › [`i18n/`](../README.md) › **`locales`**

| 文件 | 语言 |
|---|---|
| `zh.json` | 简体中文 |
| `en.json` | English |

## 结构

按界面区域分组，两个文件的**键名必须完全一致**：

```json
{
  "app":    { "title": "...", "subtitle": "..." },
  "common": { "loading": "...", "save": "...", "cancel": "..." },
  "auth":   { "login": "...", "email": "...", "signIn": "..." },
  "tabs":   { "places": "...", "timeline": "...", "unplaced": "..." },
  "map":    { "searchPlaceholder": "...", "newPlace": "...", "dots": "..." },
  "upload": { "add": "...", "done": "...", "partialFail": "..." },
  "place":  { "unnamed": "...", "kind": { "city": "...", "spot": "..." } },
  "note":   { "new": "...", "untitled": "..." },
  "photo":  { "takenAt": "...", "locate": "...", "delete": "..." }
}
```

## 插值用单大括号

```json
{ "photoCount": "{n} 张照片" }
```

使用：`t('place.photoCount', { n: 3 })`

**不要写成双大括号。** vue-i18n 会把 `{{n}}` 当作嵌套占位符并**在运行时抛错**，
而这个错误会让整个组件树渲染失败——症状是「侧栏突然空白」这类难以定位的问题。

> ⚠️ 这一点与 react-i18next 相反。从 React 迁移过来时，这是最容易踩的坑，
> 本项目在迁移过程中就真实踩过一次。

## 复数与格式

刻意**不使用 vue-i18n 的复数语法**，而是统一用 `{n}` 配合中文/英文各自的自然表达
（「3 张照片」/「3 photos」）。原因是中英文复数规则差异大，而这里的数量词都很简单，
引入复数规则反而增加维护成本。

日期与数字的本地化由 `lib/format.ts` 依据 `locale` 处理，不放在词条里。

## 相关

- 上级：[`i18n/`](../README.md)
