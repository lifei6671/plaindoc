# PlainDoc

一套面向中小团队的轻量级文档管理系统。  
如果你熟悉 MinDoc，PlainDoc 的目标很直接：保留文档系统的核心价值，同时带来更现代的编辑体验和更省心的工程化部署。

开箱即可获得：

- Go 后端（Gin + GORM）
- React 前端编辑器（Vite + CodeMirror）
- 首页/分类页 SSR
- 空间阅读页 SSR（对 SEO 友好）
- 后台管理（用户、空间、文档、主题、系统配置、审计）

---

## 产品定位

PlainDoc 是一个真正可落地的文档平台，而不是“仅能写 Markdown 的编辑器”。

我们希望它帮助团队实现三件事：

- 用 Markdown 快速搭建团队知识库/技术文档库
- 提供“编辑、阅读、管理”一体化能力，而不是单纯编辑器
- 以最低维护成本长期运行（单体部署、权限治理、可观测）

适用场景：

- 团队内部知识沉淀
- 技术文档中心
- 小中型项目文档门户
- 运维手册/流程文档管理

---

## 核心功能与亮点

### 功能

- 空间与文档树：支持空间、目录、文档的层级组织
- Markdown 编辑：左树 + 中间编辑 + 右侧预览的工作台体验
- 阅读系统：独立阅读页（`/r/:spaceId/:docId`），适合分享与访问
- 权限模型：`owner / collaborator / reader` 多角色访问控制
- 版本能力：文档版本号、冲突检测、修订记录
- 图片链路：上传、鉴权、静态回源、Markdown 直接渲染
- 后台治理：用户、空间、文档、主题、系统配置、审计管理

### 亮点

- 轻量但完整：一套系统同时覆盖编辑、阅读、权限与后台治理
- 单体部署体验：Go 主服务 + Node SSR Worker 子进程，无需独立 Node SSR 服务
- 渲染一致性：阅读 SSR 与编辑器预览复用同一套 Markdown 渲染链路
- 双 SSR 架构：
  - 首页/分类页：Go 模板 SSR
  - 阅读页：React SSR（由 Go 调度 Worker）
- 开发和生产都顺手：前端统一走后端 API 模式（`http`），联调与生产一致
- 工程化可维护：统一响应协议、错误码体系、Docker + Compose 开箱部署

---

## 1. 项目现状（截至 2026-02-23）

已落地核心能力：

- 文档编辑器（左侧文档树 + 中间编辑 + 右侧预览）
- 空间与文档权限模型（`owner / collaborator / reader`）
- 图片上传与本地静态回源（`/uploads/*`）
- 首页与分类页 SSR（`/`、`/explore/:categoryId`）
- 空间阅读页 SSR（`/r/:spaceId`、`/r/:spaceId/:docId`）
- 阅读页友好错误页与登录重定向
- 后台管理台与空间治理能力

参考：

- `docs/DAILY_PROGRESS_2026-02-23.md`
- `docs/HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`
- `docs/SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`

---

## 2. 技术栈

### 后端（`apps/server`）

- Go `1.26`
- Gin `1.11`
- GORM `1.31`
- 数据库驱动：SQLite / MySQL / PostgreSQL
- 默认 SQLite 驱动：`modernc.org/sqlite`

### 前端（`apps/web`）

- React `19`
- Vite `7`
- TypeScript `5.9`
- CodeMirror `6`
- React Markdown + remark/rehype 渲染链
- Mermaid / KaTeX / 语法高亮

### SSR

- 首页/分类页：Go `html/template`
- 阅读页：Go 通过 `stdin/stdout` 调用 Node SSR Worker（非独立 Node 服务）

---

## 3. 仓库结构

```text
.
├── apps
│   ├── server                    # Go 服务端
│   └── web                       # React 前端
├── docs                          # 方案、里程碑、日报、踩坑记录
├── Dockerfile                    # 多阶段构建镜像（web + ssr worker + go）
├── docker-compose.yml            # 单容器部署（含 SSR Worker）
└── README.md
```

---

## 4. 运行方式

### 4.1 方式 A：Docker Compose（推荐）

要求：

- Docker Engine + Docker Compose v2

在仓库根目录执行：

```bash
docker compose build
docker compose up -d
```

访问地址：

- `http://localhost:8080`

常用命令：

```bash
docker compose logs -f plaindoc
docker compose up -d --build
docker compose down
docker compose down -v
```

默认持久化卷：

- `plaindoc_data` -> `/app/data`（SQLite 数据）
- `plaindoc_uploads` -> `/app/uploads`（上传文件）

---

### 4.2 方式 B：本地开发（前后端分开启动）

前置要求：

- Node.js `>= 20`（当前 Docker 使用 Node 24）
- npm
- Go `1.26+`

### 1) 安装前端依赖

```bash
npm install
```

### 2) 启动后端

```bash
cd apps/server
cp .env.example .env
go mod tidy
go run ./cmd/server
```

默认后端地址：`http://localhost:8080`

### 3) 启动前端开发服务

```bash
# 回到仓库根目录
npm run web:dev
```

默认前端地址：`http://localhost:3001`

> 提示：开发时建议通过 Vite 反向代理访问后端，不要直接在浏览器跨端口访问 `/api`。

---

## 5. 构建与发布

### 5.1 本地构建

```bash
npm run web:build
```

等价于执行：

- `build:client` -> `apps/web/dist`
- `build:ssr-worker` -> `apps/web/dist-ssr`

### 后端测试

```bash
cd apps/server
go test ./...
```

### Docker 镜像（本地手动构建）

```bash
docker build -t plaindoc:latest .
docker run --rm -p 8080:8080 plaindoc:latest
```

### 5.2 GitHub Release 产物说明

当你推送 tag 后，`Release` 工作流会上传以下文件：

- `plaindoc-server-linux-amd64`：Linux amd64 的后端可执行文件（Go 编译产物）。
- `plaindoc-server-linux-amd64-<tag>.tar.gz`：后端二进制压缩包（包含上面的可执行文件）。
- `plaindoc-web-<tag>.tar.gz`：前端发布产物压缩包，包含 `dist`（前端静态资源）和 `dist-ssr`（阅读页 SSR Worker 产物）。
- `checksums-<tag>.txt`：上述产物的 SHA256 校验清单。

### 5.3 使用 Release 产物部署（Linux 裸机/云主机）

1. 下载对应 tag 的 3 个核心文件：
`plaindoc-server-linux-amd64-<tag>.tar.gz`、`plaindoc-web-<tag>.tar.gz`、`checksums-<tag>.txt`。  
2. 校验文件完整性：

```bash
sha256sum -c checksums-<tag>.txt
```

3. 解压到部署目录：

```bash
mkdir -p /opt/plaindoc/apps/web
tar -xzf plaindoc-server-linux-amd64-<tag>.tar.gz -C /opt/plaindoc
tar -xzf plaindoc-web-<tag>.tar.gz -C /opt/plaindoc/apps/web
chmod +x /opt/plaindoc/plaindoc-server-linux-amd64
```

4. 配置并启动（示例）：

```bash
export APP_ENV=production
export APP_ADDR=:8080
export WEB_ORIGIN=http://your-domain-or-ip:8080
export WEB_DIST_DIR=/opt/plaindoc/apps/web/dist
export DB_DRIVER=sqlite
export DB_DSN='file:/opt/plaindoc/data/plaindoc.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)'
export SSR_WORKER_ENABLED=true
export SSR_WORKER_EXEC=node
export SSR_WORKER_ENTRY=/opt/plaindoc/apps/web/dist-ssr/worker-entry.js

/opt/plaindoc/plaindoc-server-linux-amd64
```

提示：阅读页 SSR 依赖 Node 子进程。若服务器未安装 Node，请先安装 Node，或临时设置 `SSR_WORKER_ENABLED=false`。

### 5.4 DockerHub 部署（推荐）

说明：工作流会始终推送 `<tag>` 镜像标签；当 Git tag 以 `v` 开头时，额外推送 `latest`。

拉取镜像：

```bash
docker pull lifei6671/plaindoc:latest
```

直接运行：

```bash
docker run -d \
  --name plaindoc \
  -p 8080:8080 \
  -e APP_ENV=production \
  -e WEB_ORIGIN=http://localhost:8080 \
  -e DB_DRIVER=sqlite \
  -e DB_DSN='file:/app/data/plaindoc.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)' \
  -v plaindoc_data:/app/data \
  -v plaindoc_uploads:/app/uploads \
  --restart unless-stopped \
  lifei6671/plaindoc:latest
```

### 5.5 Docker Compose（使用 DockerHub 镜像）

如果不想本地 build，可使用以下 compose：

```yaml
services:
  plaindoc:
    image: lifei6671/plaindoc:latest
    container_name: plaindoc
    ports:
      - "8080:8080"
    environment:
      APP_ENV: "production"
      APP_ADDR: ":8080"
      WEB_ORIGIN: "http://localhost:8080"
      WEB_DIST_DIR: "/app/apps/web/dist"
      DB_DRIVER: "sqlite"
      DB_DSN: "file:/app/data/plaindoc.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
      DB_AUTO_MIGRATE: "true"
      SSR_WORKER_ENABLED: "true"
      SSR_WORKER_EXEC: "node"
      SSR_WORKER_ENTRY: "/app/apps/web/dist-ssr/worker-entry.js"
    volumes:
      - plaindoc_data:/app/data
      - plaindoc_uploads:/app/uploads
    restart: unless-stopped

volumes:
  plaindoc_data:
  plaindoc_uploads:
```

启动：

```bash
docker compose up -d
```

---

## 6. 路由与页面结构

### 页面路由（非 `/api`）

- `/`：首页 SSR
- `/explore/:categoryId`：分类页 SSR
- `/r/:spaceId`：空间阅读入口（跳转首篇可读文档）
- `/r/:spaceId/:docId`：阅读页 SSR（Go -> Node Worker）
- `/login`、`/register`、`/editor/*`、`/admin/*`：Web SPA（生产由 Go 托管 `WEB_DIST_DIR`）
- `/uploads/*path`：本地上传文件公开访问

### API 入口

- `/api/healthz`
- `/api/auth/*`
- `/api/spaces/*`、`/api/nodes/*`、`/api/docs/*`
- `/api/admin/*`
- `/api/uploads/images`

完整注册点见：`apps/server/internal/server/router.go`

---

## 7. SSR 架构说明

### 7.1 首页/分类页 SSR

- 由 Go 模板直接渲染
- 模板静态资源走 `/assets/*`

### 7.2 阅读页 SSR（子进程模式）

流程：

1. 浏览器请求 `/r/:spaceId/:docId`
2. Go 侧聚合阅读数据并鉴权
3. Go 通过 JSONL 协议把 payload 发给 Node Worker
4. Node Worker 返回 HTML/head/metrics
5. Go 输出完整页面

关键配置（服务端）：

- `SSR_WORKER_ENABLED`
- `SSR_WORKER_EXEC`
- `SSR_WORKER_ENTRY`
- `SSR_WORKER_COUNT`
- `SSR_RENDER_TIMEOUT`
- `SSR_WORKER_START_TIMEOUT`
- `SSR_WORKER_MAX_PAYLOAD_BYTES`
- `SSR_PROTOCOL_VERSION`

当 Worker 不可用时，阅读页会降级到基础 HTML 兜底，而不是直接雪崩 500。

---

## 8. 统一响应协议（JsonResult）

后端 API 统一返回结构：

```json
{
  "code": 0,
  "message": "success",
  "requestId": "xxxx",
  "data": {}
}
```

说明：

- 业务成功：`code = 0`
- 业务失败：`code != 0`（例如 `3001 ROUTE_NOT_FOUND`）
- 当前错误 HTTP 状态会收敛（认证类为 403，其它多为 200），前端应优先按 `code` 处理

定义见：

- `apps/server/internal/server/response/error.go`
- `apps/server/internal/server/response/error_codes.go`

---

## 9. 配置说明

### 后端环境变量

样例文件：`apps/server/.env.example`

常用项：

- `APP_ENV`、`APP_ADDR`
- `WEB_ORIGIN`（CORS）
- `WEB_DIST_DIR`（生产托管 SPA 的 dist 目录）
- `DB_DRIVER`、`DB_DSN`、`DB_AUTO_MIGRATE`
- `JWT_*`
- `SSR_WORKER_*`

`.env` 加载顺序（启动时自动读取）：

1. `.env`
2. `apps/server/.env`
3. `cmd/server/.env`
4. `apps/server/cmd/server/.env`

且系统环境变量优先级更高，不会被 `.env` 覆盖。

### 前端环境变量

样例文件：`apps/web/.env.example`

核心项：

- `VITE_API_BASE_URL`
- `VITE_DEV_PROXY_TARGET`

---

## 10. 常见问题

### 1) 阅读页返回 “route not found” 或图片不显示

- 检查是否走了正确路径：
  - 阅读：`/r/...`
  - 图片：`/uploads/...`
- 开发模式确认 Vite 代理是否开启（`/r`、`/uploads`、`/api`）

### 2) SSR Worker 启动失败

- 检查 `SSR_WORKER_ENTRY` 是否存在
- 检查 Node 可执行文件是否在 PATH（`SSR_WORKER_EXEC`）
- 检查协议版本是否一致（`SSR_PROTOCOL_VERSION`）

### 3) 修改了 `.env` 但配置未生效

- 重启服务
- 查看 `server starting` 日志中的配置摘要

### 4) 阅读页样式与编辑预览不一致

- 当前实现已把预览样式统一注入阅读 SSR
- 如出现差异，优先检查 `apps/web/src/ssr/render-space-reader.tsx` 与 `apps/web/src/styles.css`

---

## 11. 文档索引（docs）

核心建议先读：

- `docs/IMPLEMENTATION_PHASES.md`
- `docs/BACKEND_IMPLEMENTATION_PHASES.md`
- `docs/HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`
- `docs/SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`
- `docs/SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`
- `docs/DAILY_PROGRESS_2026-02-22.md`
- `docs/DAILY_PROGRESS_2026-02-23.md`

后台治理相关：

- `docs/ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`
- `docs/ADMIN_CONSOLE_RELEASE_CHECKLIST.md`

分类与封面相关：

- `docs/SPACE_CATEGORY_REFACTOR_NOTES.md`
- `docs/ADMIN_SPACE_CREATE_WITH_COVER_IMPLEMENTATION_PHASES.md`

---

## 12. License

本项目使用仓库根目录 `LICENSE`。
