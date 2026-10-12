# AGENTS.md · 协作约定

本文件被 DSH 作为工作区指令自动加载（项目根 → 工作目录链上的 `AGENTS.md`）。
请把这里的规则视为默认行为，除非用户当次明确要求例外。

---

## 一、Git 工作流：本地优先，不主动推送

```
origin = https://github.com/Carey996/travel_map
```

**凭据已缓存，随时能推——正因如此更需要克制。**

| 操作 | 默认行为 |
|---|---|
| 改文件 | 直接改，留在工作区 |
| `git commit` | **可以**，保持提交信息清晰、单个提交只做一件事 |
| `git add` / `git status` / `git diff` / `git log` | **可以**，随意 |
| `git push` | **不要做**，除非用户当次明确说「推送 / push / 传上去」 |
| `git push --force` / `reset --hard` / `rebase` 改写已提交历史 | **不要做**，除非用户明确要求 |
| `git remote set-url` / 改凭据配置 | **不要做** |

用户的原话是「后续改动还是优先本地，不直接往上推」。所以：

- 做完改动后，**在回复里说明有哪些本地改动、是否已提交**，把推送的决定权留给用户
- 不要说「已经上传/已同步到 GitHub」，除非确实执行过并验证过
- 如果用户要求推送，**推送后要实际验证**（比对 `git rev-parse HEAD` 与远端 ref，或查 API），
  不要仅凭命令退出码为 0 就断言成功

---

## 二、先读文档再改代码

项目有三层文档，改动前先定位相关那一份，不要凭空猜结构：

| 文档 | 用途 |
|---|---|
| `README.md` | 项目总览、架构、快速开始、关键设计决策、底图选型实测数据 |
| `api/README.md` | 后端总览；下层每级目录各有 `README.md`（见其「知识树」章节） |
| `web/README.md` | 前端总览；下层同理 |

**每一级目录都有自己的 README**，自底向上构成知识树。改某个目录的代码前，
先看那个目录的 README；改了行为就顺手更新它——文档和代码一起提交。

---

## 三、改代码时必须遵守的硬约束

这几条违反了不会立刻报错，但会造成难查的问题：

1. **坐标只存 WGS-84。** EXIF 是 WGS-84，国内底图是 GCJ-02，两者在大陆差 100~600 米。
   只在渲染到 GCJ-02 底图的那一刻转换，逻辑集中在 `web/src/lib/coords.ts`，
   依据底图源的 `crs` 字段决定。**不要绕过它**。
2. **前端不要手写 `fetch`。** 变更类请求必须带 `X-Requested-With`（后端据此抵御 CSRF），
   统一走 `web/src/api/client.ts` 的 `apiFetch()`。
3. **vue-i18n 插值是单大括号。** 写 `{n}` 而不是 `{{n}}`——后者会被判为嵌套占位符
   并**在运行时抛错，导致整个组件树渲染失败**（症状是「侧栏突然空白」）。
   注意这与 react-i18next 相反。
4. **后端保持无状态。** 不存进程内会话、不写本地磁盘、不做进程内缓存、不接收图片字节流。
   图片一律由浏览器经预签名 URL 直传对象存储。
5. **照片响应必须经 `dto.NewPhotoOut()` 生成**（`api/internal/dto/photo.go`），
   它负责把对象键换成预签名直链。直接返回 ORM 对象会泄露存储键、前端也拿不到可用地址。
6. **每个数据库查询都要带 `user_id` 过滤**，这是多租户隔离的唯一防线。

---

## 四、常用命令

**仓库里只写 bash 命令**——平台无关，Linux / macOS / WSL / Git Bash 都能跑；
完整可复制的一套在根 README 的「快速开始」与「用 Kubernetes 部署」两节。

```bash
# 依赖容器（Postgres / Redis / MinIO）：命令见根 README「快速开始」第 1 步
until podman exec travelblog-postgres pg_isready -U travelblog -d travelblog >/dev/null 2>&1; do sleep 1; done

# 单独跑后端 / 前端
cd api && go run ./cmd/server          # http://127.0.0.1:8000
cd web && npm run dev                  # http://127.0.0.1:5173

# 类型检查与构建
cd web && npx vue-tsc --noEmit && npx vite build

# 后端依赖与构建（国内需先把模块代理指向 goproxy.cn）
go env -w GOPROXY=https://goproxy.cn,direct
go mod tidy
cd api && go build -o bin/server ./cmd/server

# 镜像与本地 K8s Pod
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -C api -trimpath -ldflags '-s -w' -o bin/server-linux ./cmd/server
podman build -f deploy/containerfiles/Containerfile.api.local -t localhost/travel-blog-api:dev .
podman build -f deploy/containerfiles/Containerfile.web       -t localhost/travel-blog-web:dev .
podman play kube --replace deploy/k8s/local/pod.yaml   # 本地跑，不需要集群
podman play kube --down    deploy/k8s/local/pod.yaml

# 停止
kill "$(cat .logs/api.pid)" "$(cat .logs/web.pid)" 2>/dev/null   # 只停前后端
podman rm -f travelblog-postgres travelblog-redis travelblog-minio   # 停依赖容器
```

**`scripts/*.ps1` 是本机自用的 PowerShell 脚本，已写进 `.gitignore`，不在仓库里**，
文档也不要再引用它们。本机仍可直接跑（`dev-up.ps1` / `dev.ps1` / `dev-down.ps1` /
`stop-dev.ps1` / `k8s-build.ps1` / `k8s-up.ps1`），但对外只暴露上面的 bash 命令。

---

## 五、改动后的验证要求

**不要只跑类型检查就宣布完成。** 本项目的既有验证力度是「真实依赖 + 真实浏览器」：

| 层次 | 手段 |
|---|---|
| 前端静态 | `npx vue-tsc --noEmit` 与 `npx vite build` |
| 后端接口 | 起真实服务，用真实 Postgres / Redis / MinIO 跑接口 |
| 页面渲染 | 无头 Edge + CDP 协议截图，并统计 DOM 与 console 异常数 |

无头浏览器验证的参考做法（本项目验证前端时一直这么做）：

```
启动 msedge --headless=new --remote-debugging-port=<PORT> --user-data-dir=<临时目录>
通过 http://127.0.0.1:<PORT>/json/list 拿 webSocketDebuggerUrl
用 WebSocket 发 CDP 命令：Network.setCookie 注入会话 -> Page.navigate
-> 等待 -> Runtime.evaluate 统计 DOM -> Page.captureScreenshot 截图人工核对
```

**注意：调试端口要避开 Windows 保留端口段**（本机 WSL2 占用 9052~9610，
用 `netsh interface ipv4 show excludedportrange protocol=tcp` 查看；实测 9666 可用）。

---

## 六、本机环境备忘（都是实际踩过的）

| 现象 | 原因与处理 |
|---|---|
| 容器起不来 / 接口 500 且日志是连接被拒 | Podman 机器重启后容器会停，重跑根 README「快速开始」第 1 步（本机也可 `scripts\dev-up.ps1`） |
| `go mod download` 卡住 | `go env -w GOPROXY=https://goproxy.cn,direct`（官方 proxy 在本机不可达） |
| 容器 Up 但连接被拒 | 端口落在 **Windows 保留段**；`netsh interface ipv4 show excludedportrange protocol=tcp` 查看，换主机端口重建容器（本项目 Redis 因此用 16379） |
| `dial tcp [::1]:...` | `localhost` 被解析成 IPv6；配置层已统一归一化为 127.0.0.1 |
| PowerShell 脚本中文乱码报错 | Windows PowerShell 5.1 按 ANSI 读 `.ps1`，脚本必须存为 **UTF-8 with BOM** |
| `podman` 找不到 | 本机在 `%LOCALAPPDATA%\Programs\Podman\podman.exe`；调外部命令时用绝对路径 |
| `vue-tsc` 报 `Package subpath './lib/tsc' is not defined` | TypeScript 装成了 7.x，需 `npm i -D typescript@^5.9` |
| 无头浏览器 CDP 起不来（`bind() 0x271D`） | 端口落在保留段，换端口 |
| `github.com` 时通时不通 | `api.github.com` 与 `codeload.github.com` 稳定，`github.com` 间歇；大文件下载基本过不去 |
| 用 `pwsh` 工具执行命令失败 | 该工具在本机不稳定；改用 Node 的 `child_process` 调**绝对路径**（裸命令名解析不了，因为进程环境为空） |
| 前端镜像构建报 `sh: vue-tsc: Permission denied` | 构建上下文把 Windows 的 `node_modules` 拷进了容器，而它的 `.bin` 没有可执行位。根目录的 `.containerignore` / `.dockerignore` 就是为此存在的，**不要删** |
| `podman play kube` 报 `address already in use` | 开发容器还占着 9000；先 `podman rm -f travelblog-minio`（本机也可 `scripts/dev-down.ps1`） |
| `podman play kube` 报 `a dependency of container ... failed to start` | 多半是上面那条端口冲突的连带症状；也可能引用了不存在的镜像名（要用 `localhost/travel-blog-api:dev` 这种全名） |
| 单次代码执行结束后进程被回收 | 长驻服务（后端 / Vite / 无头浏览器）必须在**同一次执行内**启动、验证、收尾 |
