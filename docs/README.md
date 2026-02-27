# Docs 导航说明

**Last Updated**: 2026-02-27  
**目标**: 帮助后续开发人员和 AI Agent 在最短时间内找到正确文档、理解项目现状并开始开发。

---

## 1. 先读这三份主文档（推荐）

这三份是当前统一口径，建议优先阅读：

1. `FRONTEND_DEVELOPER_GUIDE.md`  
前端技术栈、已实现能力、打包方式、常见踩坑、上手路径。

2. `BACKEND_DEVELOPER_GUIDE.md`  
后端技术栈、接口能力、编译发布、运维要点、常见踩坑。

3. `ENGINEERING_STANDARDS.md`  
前后端代码规范、代码结构、跨端契约、测试门禁、文档协作规范。

---

## 2. 按角色的阅读路径

### 前端开发者

1. `FRONTEND_DEVELOPER_GUIDE.md`
2. `ENGINEERING_STANDARDS.md`
3. `ai-handoff-pitfalls.md`
4. `HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`（涉及首页模板/SSR时）
5. `SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`（涉及阅读页 SSR 时）

### 后端开发者

1. `BACKEND_DEVELOPER_GUIDE.md`
2. `ENGINEERING_STANDARDS.md`
3. `BACKEND_IMPLEMENTATION_PHASES.md`
4. `ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`
5. `SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`
6. `LDAP_DIRECT_AUTH_IMPLEMENTATION_PLAN.md`（涉及 LDAP/统一认证改造时）

### AI Agent / 新同学快速上手

1. `FRONTEND_DEVELOPER_GUIDE.md`
2. `BACKEND_DEVELOPER_GUIDE.md`
3. `ENGINEERING_STANDARDS.md`
4. `DAILY_PROGRESS_2026-02-23.md`（最近阶段快照）
5. `LDAP_DIRECT_AUTH_IMPLEMENTATION_PLAN.md`（认证体系改造任务清单）

---

## 3. 按主题索引

### A. 当前主文档（持续维护）

1. `FRONTEND_DEVELOPER_GUIDE.md`
2. `BACKEND_DEVELOPER_GUIDE.md`
3. `ENGINEERING_STANDARDS.md`

### B. 后台治理与发布

1. `ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`
2. `ADMIN_CONSOLE_RELEASE_CHECKLIST.md`
3. `ADMIN_SPACE_CREATE_WITH_COVER_IMPLEMENTATION_PHASES.md`

### C. SSR 与阅读链路

1. `HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`
2. `SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`
3. `SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`

### D. 数据模型与改造说明

1. `SPACE_CATEGORY_REFACTOR_NOTES.md`
2. `BACKEND_IMPLEMENTATION_PHASES.md`
3. `LDAP_DIRECT_AUTH_IMPLEMENTATION_PLAN.md`

### E. 交接与踩坑记录

1. `ai-handoff-pitfalls.md`
2. `backend-ai-handoff.md`
3. `DAILY_PROGRESS_2026-02-22.md`
4. `DAILY_PROGRESS_2026-02-23.md`

### F. 文档配图素材

1. `screenshot-*.png`

---

## 4. 如何使用这些文档

1. 需要“开始开发”：先读主文档（第 1 节）。
2. 需要“理解某条链路设计”：读对应专题实施文档（第 3 节 B/C/D）。
3. 需要“排查历史问题”：读踩坑与日报（第 3 节 E）。
4. 需要“上线/发布”：优先走 `ADMIN_CONSOLE_RELEASE_CHECKLIST.md`。
5. 需要“接入 LDAP / 认证方式改造”：优先走 `LDAP_DIRECT_AUTH_IMPLEMENTATION_PLAN.md`。

---

## 5. 文档维护约定

1. 三份主文档是当前事实口径，功能变化后必须优先更新。
2. 历史阶段文档保留用于追溯，不作为唯一事实来源。
3. 新增文档时，需同步更新本导航文件对应分类。
4. 文档建议包含：
   - `Last Updated`
   - 适用范围
   - 与其他文档的关系（依赖/补充/替代）

---

## 6. 推荐最小验证命令

前端改动后：

```bash
npm run check:dropdown-menu -w @plaindoc/web
npm run web:build
```

后端改动后：

```bash
cd apps/server && go test ./... -count=1
```

---

## 7. 常见名词解释与入口速查

| 名词 | 用户入口（URL） | 前端/模板入口 | 后端入口 | 说明 |
| --- | --- | --- | --- | --- |
| 首页 | `/` | `apps/server/internal/server/view/templates/home.tmpl` | `apps/server/internal/server/router.go` + `apps/server/internal/server/handler/home.go` + `apps/server/internal/service/home_service.go` | 首页是 SSR 页面，不走 React SPA 路由。 |
| 分类页 | `/explore/:categoryId`（如 `/explore/all`） | `apps/server/internal/server/view/templates/explore.tmpl` | 同上（`home.go` 的 `Explore`） | 分类筛选和分页都在服务端完成。 |
| 阅读页 | `/r/:spaceId/:docId`（空间入口 `/r/:spaceId`） | `apps/web/src/ssr/render-space-reader.tsx` | `apps/server/internal/server/handler/reader_page.go` + `apps/server/internal/service/reader_page_service.go` | 页面 HTML 由 Go 调 Node SSR Worker 生成。 |
| 阅读页文档树 | 阅读页左侧区域 | `render-space-reader.tsx` 的 `ReaderTree` 与 `.reader-sidebar__tree-scroll` | `reader_page_service.go` 的树模型组装 | 左侧目录树支持折叠、激活态、可见性标识。 |
| 阅读页内容区 | 阅读页中间正文区域 | `render-space-reader.tsx` 的 `article#plaindoc-preview-body` + `.reader-article-shell` | `reader_page_service.go` 文档内容读取 | 正文渲染与编辑器预览链路共享 Markdown 规则。 |
| 阅读页大纲区 | 阅读页右侧区域（有标题时显示） | `render-space-reader.tsx` 的 `.reader-outline`，异步行为在 `render-space-reader.async-script.ts` | 无独立 handler，数据来自阅读页 payload | 大纲点击滚动、激活同步由前端注入脚本处理。 |
| 编辑器工作台 | `/editor/:spaceId/:docId` | `apps/web/src/App.tsx` + `apps/web/src/workspace/use-workspace.ts` | `router.go` 下 `/api/spaces/*`、`/api/docs/*` | React SPA 路由，左树 + 编辑 + 预览。 |
| 后台页面入口 | `/admin`、`/admin/*` | `apps/web/src/admin/AdminApp.tsx` | `apps/server/internal/server/web_spa.go`（SPA 托管路由） | 后台页面前端在 `apps/web`，但路由入口由 `apps/server` 托管。 |
| 空间管理页 | `/admin/spaces` | `apps/web/src/admin/pages/AdminSpacesPage.tsx` | `/api/admin/spaces*` 路由在 `apps/server/internal/server/router.go` | 空间列表、状态治理、分类、成员、转让、删除。 |
| 新建空间页 | `/admin/spaces` 内“新建空间”按钮弹窗 | `apps/web/src/admin/components/AdminCreateSpaceDialog.tsx` | `POST /api/admin/spaces` + `POST /api/admin/spaces/cover-assets` | 当前是弹窗形态，不是单独 URL。 |

---

## 8. AI 快速定位（按任务）

1. 改前端路由解析：`apps/web/src/App.tsx`（`parseAppRoute`、登录守卫、编辑器路由同步）。
2. 改浏览器标题规则：`apps/web/src/App.tsx`（编辑器标题）与 `apps/web/src/admin/AdminApp.tsx`（后台标题）。
3. 改后台菜单与模块挂载：`apps/web/src/admin/AdminApp.tsx`（`buildAdminMenu` + 页面分发）。
4. 改空间管理行为：`apps/web/src/admin/pages/AdminSpacesPage.tsx`。
5. 改新建空间流程：`apps/web/src/admin/components/AdminCreateSpaceDialog.tsx`。
6. 改首页/分类页样式或结构：`apps/server/internal/server/view/templates/*.tmpl` 与 `templates/partials/*.tmpl`。
7. 改首页/分类页数据规则：`apps/server/internal/service/home_service.go` + `apps/server/internal/server/handler/home.go`。
8. 改阅读页 SSR 结构/样式：`apps/web/src/ssr/render-space-reader.tsx` + `apps/web/src/ssr/render-space-reader.base.css`。
9. 改阅读页异步增强（目录树切换/大纲滚动/无刷新跳转）：`apps/web/src/ssr/render-space-reader.async-script.ts`。
10. 改后端 API 路由入口：`apps/server/internal/server/router.go`（统一依赖注入与路由注册）。
11. 改服务启动入口与 SSR Worker 生命周期：`apps/server/cmd/server/main.go`。
12. 改 SPA 托管入口（`/login`、`/editor/*`、`/admin/*`）：`apps/server/internal/server/web_spa.go`。
13. 改前后端 API 契约：先看 `apps/web/src/data-access/http/adapter.ts`，再对照 `apps/server/internal/server/handler/*`。
14. 改登录认证方式（LDAP/混合登录）：先读 `docs/LDAP_DIRECT_AUTH_IMPLEMENTATION_PLAN.md`，再看 `apps/server/internal/server/handler/auth.go`、`apps/server/internal/service/auth_service.go`、`apps/web/src/components/AuthPanel.tsx`。

---

## 9. 高频坑：`npm run web:build` 偶发 Node 崩溃

现象（已多次出现）：

1. 在 `vite build` 阶段偶发崩溃，日志包含 `Fatal error ... unreachable code`。
2. 进程可能以 `Trace/breakpoint trap (core dumped)` 结束。
3. 这类问题通常不是业务代码 TS 类型错误，而是 Node/V8 运行时层面的偶发崩溃。

建议处理顺序：

1. 先原命令重试一次：`npm run web:build`（多数场景可恢复）。
2. 若连续失败，拆分执行定位：`npm run build:client -w @plaindoc/web` 与 `npm run build:ssr-worker -w @plaindoc/web`。
3. 检查 Node 版本是否满足要求（当前项目要求 `>=20`）。
4. 若是 CI 任务，建议在前端构建步骤增加一次自动重试，避免偶发崩溃导致流水线误失败。

---

## 10. 关键提醒：后台入口在 `apps/server`

虽然后台页面代码在 `apps/web/src/admin`，但“请求入口和路由挂载”在后端：

1. 服务启动入口：`apps/server/cmd/server/main.go`
2. 全量路由入口：`apps/server/internal/server/router.go`
3. SPA 路由托管：`apps/server/internal/server/web_spa.go`
4. 后台 API 根组：`/api/admin/*`（定义在 `router.go`）

结论：

1. 改后台页面行为通常要同时关注 `apps/web`（页面）与 `apps/server`（API/路由）。
2. 仅看前端目录可能找不到真实入口或权限中间件位置。
