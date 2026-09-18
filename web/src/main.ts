import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus from 'element-plus';
import * as ElementPlusIcons from '@element-plus/icons-vue';

import 'element-plus/dist/index.css';
// Element Plus 官方暗色主题，靠 <html class="dark"> 切换
import 'element-plus/theme-chalk/dark/css-vars.css';
import 'leaflet/dist/leaflet.css';
import './styles/base.css';
import './styles/map.css';

import App from './App.vue';
import { i18n } from './i18n';

// 跟随系统主题：Element Plus 的暗色变量挂在 .dark 类上
const darkQuery = window.matchMedia('(prefers-color-scheme: dark)');
const applyTheme = (dark: boolean): void => {
  document.documentElement.classList.toggle('dark', dark);
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
};
applyTheme(darkQuery.matches);
darkQuery.addEventListener('change', (event) => applyTheme(event.matches));

const app = createApp(App);

app.use(createPinia());
app.use(i18n);
app.use(ElementPlus);

// 全量注册图标组件，模板里可直接用 <el-icon><Location /></el-icon>
for (const [name, component] of Object.entries(ElementPlusIcons)) {
  app.component(name, component);
}

app.mount('#app');
