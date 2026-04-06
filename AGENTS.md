# PlainDoc AGENTS 指南

> 目标：让新加入的 AI Agent 在 5-10 分钟内建立正确心智模型，避免改动跑偏。
>  
> 适用范围：本仓库根目录（`apps/server` + `apps/web` + `docs`）。

---

## 1. 项目一句话

PlainDoc 是一个面向中小团队的文档平台：Go 单体后端 + React 前端编辑器 + 双 SSR（Go 模板 SSR + Node Worker 阅读 SSR），覆盖编辑、阅读、分享与后台治理。

---

## 2. 当前状态（基于仓库与 docs 实况）

已稳定可用：

1. 认证（local + LDAP）、JWT/refresh 会话轮换、验证码风控与临时封禁。
2. 工作区协作（空间/目录树/文档/修订/可见性/权限）。
3. 管理后台（用户、空间、文档、主题、系统配置、审计、operation token）。
4. 阅读页 SSR（Go 调 Node Worker）与首页/分类页 Go 模板 SSR。
5. 文档分享、文档模板、邮箱找回密码、阅读页 SEO slug、SSR 移动端适配。
6. ONLYOFFICE 一等文档能力（Word/Excel 编辑、阅读、分享；不进 sitemap/检索）。

仍在推进：

1. 阅读页 SSR 一致性自动化与性能收口（M6/M7）。
2. LDAP Phase 6（灰度、监控、回滚演练）。
3. ONLYOFFICE 阅读页本地 HTML 渲染（`In Progress`）。
4. 文档导入能力（`Draft`）。

---

## 3. 技术栈与架构

- 后端：Go 1.26 + Gin + GORM + SQLite/MySQL/PostgreSQL
- 前端：React 19 + TypeScript + Vite 7 + CodeMirror 6
- SSR：
  - 首页/分类：Go `html/template`
  - 阅读页：Go 进程管理 Node SSR Worker（非独立 Node 服务）

核心响应协议：

- API 统一返回 `JsonResult`：`{ code, message, requestId, data }`
- 业务成功判断以 `code == 0` 为准，不只看 HTTP 状态码

---

## 4. 仓库地图（先看这些）

后端主入口：

1. `apps/server/cmd/server/main.go`（启动、配置、SSR Worker、迁移）
2. `apps/server/internal/server/router.go`（唯一路由注册与依赖注入入口）
3. `apps/server/internal/service/*`（业务规则）
4. `apps/server/internal/storage/*`（模型、迁移、仓储）
5. `apps/server/internal/ssr/*`（Worker 协议/进程池）

前端主入口：

1. `apps/web/src/App.tsx`（路由、认证、编辑器主流程）
2. `apps/web/src/data-access/types.ts`（前后端契约真值）
3. `apps/web/src/data-access/http/adapter.ts`（统一请求与错误模型）
4. `apps/web/src/ssr/worker-entry.ts`（SSR Worker 协议入口）
5. `apps/web/src/ssr/render-space-reader.tsx`（阅读页 SSR）

---

## 5. 文档真值来源（Source of Truth）

先读：

1. `docs/README.md`（文档导航）
2. `docs/BACKEND_DEVELOPER_GUIDE.md`（后端唯一主文档）
3. `docs/FRONTEND_DEVELOPER_GUIDE.md`（前端唯一主文档）

专题文档状态（高频）：

- 已完成：`DOCUMENT_SHARE_TECHNICAL_PLAN.md`
- 已完成：`DOCUMENT_TEMPLATE_TECHNICAL_PLAN.md`
- 已完成：`EMAIL_PASSWORD_RESET_TECHNICAL_PLAN.md`
- 已完成：`READER_DOCUMENT_IDENTIFIER_SEO_TECHNICAL_PLAN.md`
- 已完成：`READER_SINGLE_PAGE_EXPORT_TECHNICAL_PLAN.md`
- 已完成：`SSR_MOBILE_ADAPTATION_TECHNICAL_PLAN.md`
- 已完成：`ONLYOFFICE_INTEGRATION_TECHNICAL_PLAN.md`
- 进行中：`ONLYOFFICE_READER_HTML_RENDERING_TECHNICAL_PLAN.md`
- 草案：`DOCUMENT_IMPORT_TECHNICAL_PLAN.md`

---

## 6. 本地开发与验证（按项目约定）

优先使用根目录 `Makefile`：

1. `make install`
2. `make web-dev`
3. `make server-dev`
4. `make web-build`
5. `make web-build-ssr`
6. `make server-dev-ssr`
7. `make test-server`
8. `make build`

必要直跑命令：

- 后端：`cd apps/server && go test ./... -count=1`
- 前端构建门禁：`npm run web:build`
- 前端规则门禁：`npm run check:dropdown-menu -w @plaindoc/web`

环境变量样例：

- 后端：`apps/server/.env.example`
- 前端：`apps/web/.env.example`

---

## 7. 关键业务模型（改动前必须理解）

1. 目录树结构由 `nodes.type` 表示：`folder/doc`。
2. 文档格式由文档字段表示：`markdown/docx/xlsx`（不要把格式语义塞回 `NodeType`）。
3. 权限模型：
   - 协作端：`owner > collaborator > reader`
   - 管理端：`platform_admin` 与 `space_admin` 严格隔离
   - 管理后台壳页：所有已登录用户可进入，`/admin/me` 与 `/admin/profile` 只要求登录态；普通成员若属于某个空间，可看到空间管理但只保留“编辑文档”入口；分享中心对所有登录用户开放，但普通用户只显示“我的分享”，且只能修改/取消自己创建的分享；左下角用户徽章需随当前登录用户角色动态展示。
   - 个人资料自助更新：普通登录用户可在 `/admin/profile` 修改昵称、头像和密码；后台审计仅对 `profile` / `profile_password` 这类自助场景放行，其他后台审计仍要求管理员身份。
4. 高风险后台写操作必须走 operation token，并写审计日志。

---

## 8. AI 改动硬约束（高频踩坑）

1. 不要在页面层直接写业务 `fetch`，统一走 `apps/web/src/data-access/*`。
2. 改 API 契约时，必须同步更新：
   - `apps/web/src/data-access/types.ts`
   - `apps/web/src/data-access/http/adapter.ts`
   - 对应消费页面
3. 阅读/预览链路改动必须保留关键契约：
   - Markdown 插件顺序：`rehype-raw -> rehype-sanitize -> rehype-katex`
   - 阅读页 `data-reader-*` hooks 不可随意改
4. 管理端 `DropdownMenu` 默认 `modal=false`，禁止绕过封装直接引入 Radix 原始包。
5. 管理后台菜单与右侧操作区必须按“角色 + 能力视图”生成，不能只按 `platform_admin` / `space_admin` 角色硬编码；普通用户进入后台时，至少应能看到个人信息页和分享中心的“我的分享”视图，并且只能对自己创建的分享执行改码、延期、设永久、取消等操作。
6. 涉及数据库结构改动时，必须同步三套迁移：
   - `apps/server/internal/storage/migrations/sqlite`
   - `apps/server/internal/storage/migrations/mysql`
   - `apps/server/internal/storage/migrations/postgres`
   - PostgreSQL 迁移脚本可以使用 `DO $$...$$` 这类 dollar-quoted block；当前迁移执行器已支持该语法，不要为了拆句器兼容性刻意回避。
7. 后端分层不能混用：
   - `handler` 做参数校验与响应映射
   - `service` 做业务与权限
   - `repository` 做数据访问与事务
8. 若修改 SSR Worker 相关逻辑，先确认 `apps/web/dist-ssr/worker-entry.js` 存在，避免本地联调误判。
9. `repository` 层构造 GORM 查询时，优先使用 `models` 结构体或其 `TableName()` 派生表名；复合条件优先拆成多个 `Where(...)`，`EXISTS/NOT EXISTS` 优先使用子查询对象传入，避免硬编码表名和长字符串 `AND` 链接。

---

## 9. 任务到文件的快速映射

- 认证/登录/刷新：`apps/server/internal/server/handler/auth.go` + `apps/web/src/components/AuthPanel.tsx`
- 权限与后台：`apps/server/internal/service/admin_*` + `apps/web/src/admin/*`
- 阅读页 SSR：`apps/server/internal/service/reader_page_service.go` + `apps/web/src/ssr/*`
- ONLYOFFICE：`apps/server/internal/server/handler/onlyoffice*.go` + `apps/web/src/components/OnlyOfficeEditorPane.tsx`
- 分享：`apps/server/internal/server/handler/workspace_document_share.go` + `apps/web` 阅读页/分享页相关组件
- 模板：`apps/server/internal/server/handler/document_template.go` + `apps/web/src/admin/pages/AdminDocumentTemplatesPage.tsx`
- 搜索：`apps/server/internal/search/*` + `apps/server/internal/service/search_*`

---

## 10. 给 AI 的执行建议

1. 先判定改动属于哪条链路：认证、编辑器、阅读 SSR、后台治理、搜索、ONLYOFFICE。
2. 再定位“契约真值文件”（types/config/router），最后落到实现细节。
3. 每次改动后至少执行最小回归：`go test ./... -count=1`（server）+ `npm run web:build`（web）。
4. 涉及契约、配置、SSR、路由变更时，同步更新 `BACKEND_DEVELOPER_GUIDE.md` 或 `FRONTEND_DEVELOPER_GUIDE.md`，避免文档漂移。
