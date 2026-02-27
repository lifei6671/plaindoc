# 前端开发指南（`apps/web`）

**Last Updated**: 2026-02-26  
**适用对象**: 新加入项目的前端工程师、全栈工程师、AI Agent  
**目标**: 快速理解当前前端能力边界，并可直接进入开发与联调状态

---

## 1. 前端定位与现状

`apps/web` 是 PlainDoc 的交互中心，当前已覆盖：

1. 登录/注册与会话恢复。
2. 文档编辑工作台（左树 + 编辑区 + 预览区）。
3. 管理后台（用户、空间、文档、主题、系统配置、审计、个人信息）。
4. 阅读页 SSR Worker（由 Go 子进程拉起 Node Worker 渲染）。
5. 与后端 `JsonResult` 协议对齐的 HTTP 数据访问层。

说明：截至 2026-02-26，前端主流程已可用，当前主要工作集中在阅读页 SSR 后续优化和一致性回归。

---

## 2. 技术栈

核心栈（来自 `apps/web/package.json`）：

1. React `19.1.x`
2. TypeScript `5.9.x`（`strict: true`）
3. Vite `7.1.x`
4. React Router DOM `7.13.x`
5. Tailwind CSS `4.1.x`
6. Radix UI + shadcn/ui 组件封装
7. CodeMirror `6`（Markdown 编辑）
8. React Markdown + remark/rehype（GFM、数学公式、HTML sanitize）
9. Mermaid、KaTeX、react-syntax-highlighter

---

## 3. 已实现功能（按能力域）

### 3.1 账号与路由

1. 支持 `/login`、`/register`、`/editor/*`、`/admin/*`。
2. 支持登录后回跳（`redirect`）。
3. 支持 token 刷新与会话恢复（`/api/auth/refresh` + `/api/auth/me`）。

### 3.2 编辑器与空间协作

1. 空间列表、空间树、节点创建/重命名/移动/删除。
2. 文档编辑与自动保存（HTTP 模式 `800ms` 节流）。
3. 文档版本冲突处理（后端返回冲突时进入前端冲突流程）。
4. 文档可见性、主题切换、修订历史。

### 3.3 预览渲染与样式体系

1. 预览支持 GFM、代码块、公式、内嵌 HTML（sanitize 后）。
2. 支持主题变量与外部样式覆盖。
3. 支持编辑区 <-> 预览区同步滚动（锚点映射）。
4. 支持代码块样式内联与微信导出链路。

### 3.4 阅读页 SSR 协作链路

1. Node Worker 入口：`src/ssr/worker-entry.ts`。
2. SSR 渲染函数：`src/ssr/render-space-reader.tsx`。
3. 共享 Markdown 渲染模块：`src/ssr/markdown-shared.ts`。
4. 开发态已通过 `/r` 与 `/uploads` 代理与后端阅读链路打通。

### 3.5 管理后台

1. 角色菜单：`platform_admin` 与 `space_admin` 差异化展示。
2. 空间管理：含新建空间、封面上传/系统生成、成员管理、分类管理。
3. 用户/文档治理、主题管理、系统配置、审计日志、个人资料维护。
4. 高风险操作通过后端一次性 operation token 防重放。

---

## 4. 目录结构（建议先读）

1. `apps/web/src/App.tsx`：前端主入口与路由分流（编辑器/登录/后台）。
2. `apps/web/src/data-access/`：所有后端交互网关与类型契约。
3. `apps/web/src/editor/`：编辑、预览、sanitize、滚动同步等核心逻辑。
4. `apps/web/src/admin/`：后台入口、菜单与各模块页面。
5. `apps/web/src/ssr/`：阅读页 SSR Worker 渲染实现。
6. `apps/web/src/styles.css`：全局样式与预览样式核心定义。

---

## 5. 本地开发与打包教程

### 5.1 环境要求

1. Node.js `>=20`（CI/Docker 使用 Node 24）。
2. npm（使用 workspace）。

### 5.2 安装依赖

```bash
npm ci
```

### 5.3 本地开发

```bash
# 启动后端（另一个终端）
cd apps/server
go run ./cmd/server

# 回到仓库根目录启动前端
cd /home/lifei6671/src/plaindoc
npm run web:dev
```

默认：

1. 前端 `http://localhost:3001`
2. 后端 `http://localhost:8080`
3. Vite 会代理 `/api`、`/r`、`/uploads` 到后端

### 5.4 前端构建

```bash
npm run web:build
```

等价于：

1. `npm run build -w @plaindoc/web`
2. 内含 `check:dropdown-menu`、`tsc -b`、`build:client`、`build:ssr-worker`

构建产物：

1. `apps/web/dist`（SPA 客户端）
2. `apps/web/dist-ssr`（阅读页 SSR Worker）

### 5.5 发布打包（前端产物）

Release 工作流会自动打包：

1. `plaindoc-web-<tag>.tar.gz`（包含 `dist` + `dist-ssr`）
2. 该包可用于只替换前端资源，不必重编译后端

---

## 6. 常见踩坑指南（高频）

1. 同步滚动失效：改动 `remark/rehype` 或 `pre/code` 渲染时，必须保留锚点属性传递与重建时机。
2. sanitize 误删锚点：`rehype-sanitize` 必须允许 `data-*`，否则滚动映射会漂移或失效。
3. DropdownMenu 阻断页面交互：必须保持 `modal=false`，并执行 `npm run check:dropdown-menu -w @plaindoc/web`。
4. 阅读页或图片 404：开发态未代理 `/r`、`/uploads` 时会出现 `route not found`。
5. 样式改动导致微信导出退化：调整预览相关选择器时要同步更新 `editor/wechat-export.ts` 关键选择器映射。
6. 认证循环退出：前端应按 `JsonResult.code` 判定授权失败并触发统一未授权事件，不要只看 HTTP 状态码。
7. 开发态滚动偶发异常：开发模式禁用 StrictMode 双挂载是已确认策略，不要随意改回。
8. 直接访问 `/editor` 无空间上下文：当前后端已收敛入口，前端需要按空间路由组织跳转。

---

## 7. 上手建议（新成员 / AI Agent）

1. 先读 `src/App.tsx` 与 `src/data-access/http/adapter.ts`，掌握路由与协议。
2. 再读 `src/editor/markdown-shared.ts`、`src/editor/use-scroll-sync.ts`，理解渲染与同步滚动核心。
3. 后台需求优先从 `src/admin/AdminApp.tsx` 和对应 `pages/*` 下手。
4. 提交前至少执行：
   - `npm run check:dropdown-menu -w @plaindoc/web`
   - `npm run web:build`

---

## 8. 关联文档（历史与专项）

1. `docs/ai-handoff-pitfalls.md`
2. `docs/HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`
3. `docs/SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`
4. `docs/ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`
5. `docs/DAILY_PROGRESS_2026-02-22.md`
6. `docs/DAILY_PROGRESS_2026-02-23.md`
