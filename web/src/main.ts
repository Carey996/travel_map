import { createApp } from 'vue';
import { createPinia } from 'pinia';
// Element Plus 已改为按需引入：模板里的 <el-*> 组件由 vite.config.ts 里的
// unplugin-vue-components + ElementPlusResolver 自动 import，并各自带上所需 CSS。
// 这里只处理「不走组件解析」的入口：v-loading 指令、ElMessage/ElMessageBox 服务、暗色变量。
import { ElLoading } from 'element-plus';
// 指令与命令式服务的样式不会被 resolver 自动带出，必须显式引入（按需，不是整包 index.css）
import 'element-plus/es/components/loading/style/css';
import 'element-plus/es/components/message/style/css';
import 'element-plus/es/components/message-box/style/css';
// Element Plus 官方暗色主题，靠 <html class="dark"> 切换
import 'element-plus/theme-chalk/dark/css-vars.css';
import 'leaflet/dist/leaflet.css';
import './styles/base.css';
import './styles/map.css';

import App from './App.vue';
import { i18n } from './i18n';

// 只注册模板里实际用到的图标（原来是 @element-plus/icons-vue 的 293 个全量注册）。
// 模板中以 <el-icon><Location /></el-icon> 的形式引用，因此必须是全局组件。
// 新增图标时：从 '@element-plus/icons-vue' 显式 import 后加进下面这个清单。
import {
  Aim,
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  Calendar,
  Clock,
  Close,
  Delete,
  Document,
  EditPen,
  Flag,
  FullScreen,
  Loading,
  Location,
  LocationFilled,
  Lock,
  MagicStick,
  MapLocation,
  Message,
  Notebook,
  Picture,
  Place,
  Plus,
  PriceTag,
  Search,
  UploadFilled,
  User,
  Warning,
  WarningFilled,
} from '@element-plus/icons-vue';

const icons = {
  Aim,
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  Calendar,
  Clock,
  Close,
  Delete,
  Document,
  EditPen,
  Flag,
  FullScreen,
  Loading,
  Location,
  LocationFilled,
  Lock,
  MagicStick,
  MapLocation,
  Message,
  Notebook,
  Picture,
  Place,
  Plus,
  PriceTag,
  Search,
  UploadFilled,
  User,
  Warning,
  WarningFilled,
};

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
// v-loading 指令：App.vue 的启动遮罩、PlaceDrawer 的保存遮罩都依赖它
app.use(ElLoading);

for (const [name, component] of Object.entries(icons)) {
  app.component(name, component);
}

app.mount('#app');
