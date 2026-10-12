# travel-blog-web · 前端应用

旅行地图博客的前端：**一张地图就是主页**。照片以缩略图作大头针落在地图上，
可以搜索城市或在图上点选建点，并为每个地点写 Markdown 旅行笔记。

- 框架：**Vue 3**（`<script setup>` + Composition API）
- 组件库：**Element Plus**（全量引入，跟随系统暗色主题）
- 状态：**Pinia**
- 国际化：**vue-i18n**（中 / 英）
- 地图：**Leaflet**（直接使用，未套 Vue 封装）
- 图片元数据：**exifr**（浏览器端解析 EXIF）
- 构建：**Vite**

---

## 快速开始

```powershell
npm install
npm run dev        # http://127.0.0.1:5173
```

后端需要同时运行（见 [`../api/README.md`](../api/README.md)）；依赖容器、后端、前端
三者一起起的完整流程见仓库根 README 的「快速开始」。

| 命令 | 作用 |
|---|---|
| `npm run dev` | 开发服务器，`/api` 与 `/healthz` 代理到 `127.0.0.1:8000` |
| `npm run build` | 类型检查（vue-tsc）后生产构建到 `dist/` |
| `npm run typecheck` | 只做类型检查 |
| `npm run preview` | 预览生产构建产物 |

**开发期前后端同源**：Vite 把 `/api` 代理到后端，浏览器视角下二者同源，
因此会话 Cookie 天然可用，不必处理跨站 Cookie 的一堆限制。

---

## 知识树

每一级目录都有独立的 README：上层描述整体与约定，下层描述具体文件。

```
web/
├── index.html             SPA 入口
├── vite.config.ts         构建与开发服务器（含后端代理、依赖分块）
├── tsconfig.json          类型检查配置
└── src/
    ├── main.ts            装配：Pinia / i18n / Element Plus / 图标 / 主题
    ├── App.vue            外壳：登录态分流、装配、拖拽上传
    ├── env.d.ts           Vite 与 ImportMetaEnv 类型声明
    ├── components/        视图组件（7 个 .vue）
    ├── stores/            Pinia 状态
    ├── api/               唯一的后端出口
    ├── lib/               与框架无关的纯逻辑
    ├── i18n/              国际化
    │   └── locales/       中英词条
    └── styles/            全局与地图样式
```

### 子目录索引

| 目录 | 一句话 |
|---|---|
| [`src/`](src/README.md) | 前端源码分层与依赖方向 |
| [`src/components/`](src/components/README.md) | 7 个视图组件的职责、组合关系与 MapCanvas 的硬约束 |
| [`src/stores/`](src/stores/README.md) | Pinia 状态：登录态与相册主状态 |
| [`src/api/`](src/api/README.md) | 唯一的后端出口：CSRF 头、Cookie、统一错误、全部类型 |
| [`src/lib/`](src/lib/README.md) | 纯逻辑：坐标纠偏、EXIF、上传编排、Markdown、格式化 |
| [`src/i18n/`](src/i18n/README.md) | 国际化与 Element Plus 语言联动 |
| [`src/i18n/locales/`](src/i18n/locales/README.md) | 中英词条与单大括号插值约定 |
| [`src/styles/`](src/styles/README.md) | 全局样式与 Leaflet 地图样式 |

---

### 入口与外壳

| 文件 | 职责 |
|---|---|
| `main.ts` | 注册 Pinia、vue-i18n、Element Plus 与全部图标；把 `prefers-color-scheme` 同步到 `<html class="dark">` |
| `App.vue` | 未登录显示 `LoginView`，已登录装配顶栏 / 侧栏 / 地图 / 抽屉 / 灯箱；处理「在地图上点选建点」这一跨组件交互；全窗口拖拽上传与批量进度浮层 |

### stores/ —— 状态

| 文件 | 职责 |
|---|---|
| `auth.ts` | 当前用户；`restore()` 在启动时调 `/auth/me` 确认登录态（401 视为未登录而非错误） |
| `gallery.ts` | 相册主状态：地点、照片、未定位照片、笔记，以及选中项；写操作后统一 `refresh()`；`fillMissingNames()` 给未命名地点补反向地理编码结果 |

> 数据量不大（个人旅行博客），所以**一次性全量拉取后在内存里派生视图**，
> 而不是给每个视图单独发请求。这让组件很薄，也让「上传后各处同步更新」变成一次 refresh。

### components/ —— 视图

| 组件 | 职责 | 用到的 Element Plus |
|---|---|---|
| `TopBar.vue` | 地名搜索、新建地点、上传、底图切换、照片点开关、全览、语言、账号菜单 | `el-autocomplete`（远程搜索）、`el-select`、`el-checkbox`、`el-radio-group`、`el-dropdown` |
| `PlaceSidebar.vue` | 地点列表 / 时间线 / 未定位三个视图 | `el-tabs`、`el-scrollbar`、`el-badge`、`el-image`、`el-skeleton`、`el-empty`、`el-alert` |
| `MapCanvas.vue` | Leaflet 地图：聚合大头针、高缩放下的照片点、坐标系纠偏、相机控制 | 无（Leaflet 直接使用） |
| `PlaceDrawer.vue` | 地点详情：改名、分类、评分、标签、Markdown 摘要、照片、笔记、合并与删除 | `el-drawer`、`el-input`、`el-select`、`el-rate`、`el-tag`、`el-date-picker`、`el-collapse`、`ElMessageBox` |
| `PhotoLightbox.vue` | 大图查看、EXIF 信息、说明编辑、定位与删除，方向键切换 | `el-dialog`、`el-image`、`el-descriptions` |
| `PhotoUploader.vue` | 上传入口；Element Plus 负责拖拽区与逐张状态，真实编排交给 `lib/upload.ts` | `el-upload`（自定义 `http-request`） |
| `LoginView.vue` | 登录 / 注册，带表单校验 | `el-card`、`el-form`、`el-alert` |

### lib/ —— 与框架无关的纯逻辑

这一层不依赖任何 Vue API，因此从上一版 React 实现中原样复用，也便于单独写单测。

| 文件 | 职责 | 关键导出 |
|---|---|---|
| `coords.ts` | **WGS-84 与 GCJ-02 坐标纠偏** | `wgs84ToGcj02`, `gcj02ToWgs84`, `toMapCoords`, `fromMapCoords`, `outOfChina` |
| `tiles.ts` | 底图源定义（含坐标系标记与实测结论） | `TILE_PROVIDERS`, `DEFAULT_TILE_PROVIDER`, `findProvider` |
| `exif.ts` | 浏览器端 EXIF 解析 + Canvas 生成缩略图；`sha256Hex` 供幂等使用 | `readExif`, `deriveImages`, `sha256Hex` |
| `upload.ts` | 单张照片的完整上传编排与带并发上限的批量上传 | `uploadPhotoFile`, `uploadMany` |
| `markdown.ts` | 极简 Markdown 渲染（先整体转义再有限替换） | `renderMarkdown` |
| `format.ts` | 日期、体积、坐标等展示格式化 | `formatDate`, `formatDayLabel`, `formatBytes`, `formatCoords` |

### 其他

| 文件 | 说明 |
|---|---|
| `api/client.ts` | `apiFetch` 统一加 `X-Requested-With`（CSRF）与 `credentials: 'include'`；`api` 对象暴露全部端点；类型与后端 DTO 一一对应 |
| `i18n/index.ts` | `createI18n`（Composition 模式）、语言探测与持久化、`setLang()` |
| `i18n/locales/zh.json` 与 `en.json` | 词条 |
| `styles/base.css` | 全局与布局样式，**颜色全部引用 `--el-*` 变量**，暗色主题自动生效 |
| `styles/map.css` | Leaflet 大头针、照片点与暗色底图适配 |

---

## 关键约定

### 1. 坐标一律以 WGS-84 存储

照片 EXIF 里的 GPS 是 **WGS-84**，而国内底图（高德 / 腾讯）用 **GCJ-02**，两者在国内相差 100~600 米。

约定是：**数据库与所有组件间传递的坐标永远是 WGS-84**，只在渲染到 GCJ-02 底图的那一刻转换，
地图上点到的坐标也会先转回 WGS-84 再抛出。转换集中在 `lib/coords.ts`，
`MapCanvas.vue` 依据底图源的 `crs` 字段决定是否调用。

### 2. 变更类请求必须带 `X-Requested-With`

后端用它抵御 CSRF。这个头在 `api/client.ts` 里统一加，**不要在组件里手写 fetch**，
否则会绕过保护并被后端 403。

### 3. vue-i18n 的插值是单大括号

```json
{ "photoCount": "{n} 张照片" }
```

写成双大括号会被当作嵌套占位符并在运行时抛错——注意这**与 react-i18next 相反**，
从 React 迁移过来时最容易踩。

### 4. Element Plus 全量引入

`main.ts` 里 `app.use(ElementPlus)` 并引入完整样式。好处是零配置、模板里直接写
`<el-*>`、类型提示完整；代价是 element-plus 分块约 1.1 MB（gzip 354 KB）。

若要瘦身，可改用按需引入（`unplugin-vue-components` 配合 `ElementPlusResolver`），
代价是会产生自动生成的类型声明文件，首次克隆需先构建一次。

### 5. 暗色主题

Element Plus 的暗色变量挂在 `<html class="dark">` 上，`main.ts` 监听
`prefers-color-scheme` 自动切换；`styles/base.css` 里自定义的颜色全部引用 `--el-*`，
因此跟随主题无需额外处理。

---

## 照片上传管线

```
选文件
 └─ exifr 解析 EXIF（GPS / 拍摄时间 / 机型 / 光圈 / ISO）
     └─ createImageBitmap + Canvas 生成 thumb(480) / preview(1600)，顺带按 EXIF 方向摆正
         └─ 计算 sha256
             └─ POST /photos/upload-url
                 ├─ 若该哈希已存在 -> 直接返回已有记录，整个上传短路
                 └─ 否则拿预签名 PUT URL
                     └─ 浏览器直传对象存储（后端不碰字节流）
                         └─ POST /photos 登记元数据 -> 后端按 1.5 km 自动归入地点
```

放在浏览器做的两个好处：后端不需要任何图像依赖；用户选完图立刻就能知道位置。
代价是浏览器解不了 HEIC（部分 iPhone 原图），这类文件仍可上传但没有缩略图，界面会提示。

---

## 地图与底图源

`lib/tiles.ts` 里每个源都带 `crs` 标记，以下是实测结论（中国大陆网络）：

| 源 | 京都 z13 | 上海 z13 | 冰岛 z13 | 结论 |
|---|---|---|---|---|
| **Esri 街道**（默认） | 33 KB | 46 KB | 26 KB | 全球可用 |
| **Esri 影像** | 25 KB | 22 KB | 22 KB | 全球可用 |
| 高德矢量 | 179 B | 24 KB | 179 B | 仅中国大陆有数据 |
| 腾讯 | 169 B | 169 B | 169 B | 本网络下全空白 |
| OSM 官方 | 不可达 | 不可达 | 不可达 | 本网络下连不上 |

> 判断瓦片源是否可用的可靠信号：**单色空白瓦片 PNG 只有约 179 字节，真实瓦片是几万字节。**

---

## 常见任务

**加一个组件**：在 `src/components/` 新建 `.vue`，用 `<script setup lang="ts">`。
状态从 `useGalleryStore()` 或 `useAuthStore()` 取，不要在组件里直接调 `api`（除非是一次性查询）。

**加一条词条**：同时改 `i18n/locales/zh.json` 与 `en.json`（键名必须对齐），插值用单大括号。

**加一个底图源**：往 `TILE_PROVIDERS` 追加一项，务必正确填写 `crs`——
填错会导致所有照片系统性偏移数百米。

**改地图标记样式**：改 `styles/map.css` 中的 `.pin*` 与 `.photo-dot*`，
标记 HTML 由 `MapCanvas.vue` 的 `placeIcon()` 与 `photoIcon()` 生成。

---

## 构建产物

执行 `npm run build` 后：

| 产物 | 原始 | gzip |
|---|---|---|
| `element-plus-*.js` | 1,131 KB | 354 KB |
| `index-*.js` | 192 KB | 67 KB |
| `leaflet-*.js` | 149 KB | 43 KB |
| `element-plus-*.css` | 363 KB | 49 KB |

大依赖已通过 `manualChunks`（`vite.config.ts`）拆分成独立分块，更新业务代码时它们仍走浏览器缓存。

---

## 排错

| 现象 | 原因与处理 |
|---|---|
| 侧栏或面板整个不渲染，控制台报 `Not allowed nest placeholder` | 词条里写了双大括号；vue-i18n 用单大括号 |
| 所有照片相对底图偏移数百米 | 底图源的 `crs` 标错了（GCJ-02 底图配了 WGS-84 坐标，或反之） |
| 地图一片空白但大头针正常 | 底图源在该区域没有数据；换 `esri-street` 试试 |
| 变更类请求返回 403 `csrf_header_missing` | 绕过了 `api/client.ts` 自己发了 fetch，没带 `X-Requested-With` |
| `vue-tsc` 报 `Package subpath './lib/tsc' is not defined` | TypeScript 装成了 7.x；vue-tsc 需要 5.x，执行 `npm i -D typescript@^5.9` |
| 开发时后端 500 且日志是连接被拒 | 依赖容器没起来，按根 README「快速开始」第 1 步起 Postgres / Redis / MinIO |
