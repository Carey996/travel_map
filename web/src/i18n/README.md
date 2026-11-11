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

## 新增词条后跑 `npm run check:i18n`

新增/修改界面文案后，在 `web/` 目录下执行一次巡检：

```bash
cd web && npm run check:i18n      # 等价于 node scripts/check-i18n.mjs
```

脚本（[`../../scripts/check-i18n.mjs`](../../scripts/check-i18n.mjs)，纯 Node 内置模块、无依赖）会：

1. 递归扫描 `src/` 下所有 `.vue` / `.ts`，提取 `t('...')` 与 `$t('...')` 用到的键；
2. 校验每个字面量键在 `locales/zh.json` 与 `locales/en.json` 中**都存在**——
   缺失会打印「文件:行号 + 缺哪个键」，非 0 退出；
3. 动态拼接（如 `t('place.kind.' + kind)`）按**前缀**处理：存在以该前缀开头的
   实际词条即视为已覆盖，不再算缺失；
4. 校验 zh / en 的**键集合完全一致**（单边多出来的键也会报）；
5. 兜底检查词条值里是否误写了 `{{...}}` 双大括号
   （正确写法见 [`locales/README.md`](locales/README.md) 的「插值用单大括号」，提交前必须修掉）。

输出示例：`引用键：101（字面量 100 / 动态前缀 1），词条数：zh 125 / en 125`。
改完词条再跑一遍，退出码为 0 即通过；建议与 `npm run typecheck` 一起作为提交前自检。

## 与 Element Plus 的联动

Element Plus 的分页、日期选择器等组件自带文案，需要单独喂 locale。
`App.vue` 用 `<el-config-provider :locale="epLocale">` 包住整棵树，
`epLocale` 依据当前语言在 `element-plus/es/locale/lang/zh-cn` 与 `/en` 之间切换。
**改语言时这两处会同时生效**，不会出现「界面中文、日期选择器英文」的割裂。

## 相关

- 上级：[`src/`](../README.md)
- 子目录：[`locales/`](locales/README.md)
