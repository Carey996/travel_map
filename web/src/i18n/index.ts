import { createI18n } from 'vue-i18n';

import en from './locales/en.json';
import zh from './locales/zh.json';

export const SUPPORTED_LANGS = ['zh', 'en'] as const;
export type Lang = (typeof SUPPORTED_LANGS)[number];

const STORAGE_KEY = 'travelblog.lang';

function isLang(value: unknown): value is Lang {
  return value === 'zh' || value === 'en';
}

function detectLang(): Lang {
  const saved = localStorage.getItem(STORAGE_KEY);
  if (isLang(saved)) return saved;
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

export const i18n = createI18n({
  // legacy: false 开启 Composition API 模式，配合 useI18n() 使用
  legacy: false,
  globalInjection: true,
  locale: detectLang(),
  fallbackLocale: 'en',
  messages: { zh, en },
});

export function setLang(lang: Lang): void {
  localStorage.setItem(STORAGE_KEY, lang);
  i18n.global.locale.value = lang;
  document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
}

export function currentLang(): Lang {
  return i18n.global.locale.value === 'zh' ? 'zh' : 'en';
}
