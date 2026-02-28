# 前端统一开发文档（`apps/web`）

**Last Updated**: 2026-02-28  
**适用对象**: 前端工程师、全栈工程师、AI Agent  
**目标**: 用一份文档覆盖前端开发、联调、构建、SSR、发布与排障；并把每个关键配置项对应到“来源文件 + 消费代码 + 运行影响”。

---

## 1. 当前状态（结论先看）

截至 2026-02-28，前端主链路已可用：

1. 认证链路：登录、注册、会话恢复、回跳。
2. 编辑工作台：空间树 + Markdown 编辑 + 预览 + 自动保存 + 冲突处理。
3. 阅读页 SSR：Go 调 Node Worker，前端负责 Worker 渲染实现。
4. 管理后台：用户、空间、文档、主题、系统配置、审计、个人资料页面。
5. 空间治理增强：后台新建空间、分类管理、封面上传/系统生成、成员管理。
6. 目录树拖拽：同级重排 + 跨父级移动。

当前重点：

1. 阅读页 SSR 一致性自动化与性能加固。
2. 预览链路稳定性（sanitize、滚动同步、样式注入、DropdownMenu 约束）持续防回归。

---

## 2. 技术栈与工程边界

核心栈：

1. React 19 + TypeScript 5.9（`strict`）
2. Vite 7
3. React Router DOM 7
4. CodeMirror 6
5. React Markdown + remark/rehype（含 sanitize）
6. Radix UI + shadcn/ui
7. Tailwind CSS 4

必须遵守的工程边界：

1. 后端业务 API 调用禁止页面层直连 `fetch`；统一走 `apps/web/src/data-access/*`。
2. 成功/失败优先依据 `JsonResult.code`，不是仅看 HTTP 状态码。
3. `DropdownMenu` 默认必须 `modal=false`，且不允许绕过封装直接使用 Radix 原始包。
4. 以下预览根选择器是稳定契约，不可随意改名：
   - `#plaindoc-preview-pane`
   - `.plaindoc-preview-pane`
   - `#plaindoc-preview-body`
   - `.plaindoc-preview-body`

### 2.1 强制代码规范（前端）

1. TypeScript 保持 `strict: true`；禁止新增 `any` 绕过类型系统。确需兜底时使用 `unknown + type guard`，并写明原因。
2. API 协议字段改动必须同时更新：
   - `apps/web/src/data-access/types.ts`
   - `apps/web/src/data-access/http/adapter.ts`
   - 对应页面消费逻辑
3. 禁止新增 `window.alert/window.confirm/window.prompt`；统一使用项目内对话框封装（`ConfirmDialog` / prompt 封装）。历史遗留调用应逐步替换。
4. 复杂分支、时序逻辑和兼容性处理必须补充简洁中文注释，注释说明“目的/边界”，不解释语法。
5. 菜单、抽屉等局部交互状态优先下沉到子组件，避免放在 `App.tsx` 根组件造成大范围重渲染。
6. 允许直接 `fetch` 的场景仅限：
   - 非业务 API 的外部资源探测/下载（如图片探测）
   - `multipart/form-data` 上传等 `data-access` 不便承载的链路
   - 阅读页 SSR 异步脚本内部的同源 HTML 抓取
   以上例外必须有注释说明原因与边界。

### 2.2 强制代码规范（预览与阅读 SSR）

1. Markdown 渲染插件顺序必须保持：`rehype-raw -> rehype-sanitize -> rehype-katex`。
2. 修改 `rehype-sanitize` 时必须同时验证三件事：锚点属性保留、XSS 拦截、数学公式渲染。
3. 改动 `pre/code` 渲染器时必须确认锚点属性仍能透传到预览 DOM。
4. 任何会影响预览高度的改动（主题切换、外部样式、图片加载）都必须触发滚动映射重建。
5. 不可删除滚动重算关键机制：图片 `load/error`、`ResizeObserver`、`MutationObserver`。
6. 预览样式改动时必须同步更新 `apps/web/src/editor/wechat-export.ts` 中的 `WECHAT_CRITICAL_SELECTORS_BY_TYPE`，且不得移除 `inlineWechatCriticalStyles` 兜底链路。
7. 阅读页异步脚本必须依赖 `data-reader-*` hook，禁止依赖视觉 class 作为交互契约。
8. 阅读页不得暴露编辑态专用锚点属性（如 `data-source-line`、`data-anchor-index`）。

---

## 3. 目录与事实来源（Source of Truth）

优先阅读文件：

1. `apps/web/src/App.tsx`（路由解析、认证流程、编辑器主流程）
2. `apps/web/src/main.tsx`（样式注入、StrictMode 策略）
3. `apps/web/src/data-access/types.ts`（前后端契约类型）
4. `apps/web/src/data-access/http/adapter.ts`（请求、鉴权、冲突、错误模型）
5. `apps/web/vite.config.ts`（客户端构建与开发代理）
6. `apps/web/vite.ssr.config.ts`（SSR Worker 构建配置）
7. `apps/web/src/ssr/worker-entry.ts`（Worker 协议入口）
8. `apps/web/src/ssr/render-space-reader.tsx`（阅读页 SSR HTML 生成）
9. `apps/web/src/ssr/render-space-reader.async-script.ts`（阅读页异步增强脚本）
10. `apps/web/scripts/check-dropdown-menu-modal.mjs`（DropdownMenu 规范门禁）

目录职责：

1. `src/data-access/`：API 契约、请求网关、会话与错误策略。
2. `src/editor/`：编辑器、预览渲染、sanitize、滚动同步。
3. `src/admin/`：后台入口、角色菜单、治理页面。
4. `src/ssr/`：阅读页 Worker 协议、SSR 渲染与异步增强。
5. `src/components/ui/`：UI 基础组件封装与行为约束。

---

## 4. 配置系统（来源、优先级、消费点）

### 4.1 前端环境变量来源与优先级

来源文件：`apps/web/.env.example`（注释已给出规则）。

约定：

1. 浏览器端仅暴露 `VITE_*` 变量。
2. `.env` 变更后需重启 Vite dev server。
3. 不要把密钥放在前端环境变量（会打包进客户端）。

### 4.2 前端配置矩阵（含消费位置）

| 配置项 | 默认值 | 来源文件 | 主要消费位置 | 作用与影响 |
| --- | --- | --- | --- | --- |
| `VITE_API_BASE_URL` | `/api` | `.env*` | `apps/web/src/data-access/index.ts` | 决定前端 API 基础前缀；若填绝对地址则不依赖 Vite 反代。 |
| `VITE_DEV_PROXY_TARGET` | `http://localhost:8080` | `.env*` | `apps/web/vite.config.ts` | 仅开发态生效，决定 `/api` `/r` `/uploads` 与 fallback 路由代理到哪个后端地址。 |
| `import.meta.env.DEV` | Vite 注入 | 构建时注入 | `apps/web/src/main.tsx` | 开发态关闭 StrictMode 双挂载，减少编辑器滚动容器抖动。 |
| `PACKAGE_VERSION` | `3.2.1`（define） | `vite.config.ts` / `vite.ssr.config.ts` | 构建常量 | 供 `mathjax-full` 浏览器构建读取，避免走 Node `require` 分支。 |

### 4.3 SSR Worker 运行时环境（前后端交界）

这些变量由后端进程注入给 Node Worker，前端代码只负责读取：

| 配置项 | 读取位置 | 说明 |
| --- | --- | --- |
| `SSR_PROTOCOL_VERSION` | `apps/web/src/ssr/worker-entry.ts` | Worker 握手和 render 请求协议版本，默认 `v1`。 |
| `SSR_WORKER_ENTRY` | 后端 `main.go` 校验与启动 | 必须指向已构建的 `worker-entry.js` 文件（不是目录）。 |
| `SSR_WORKER_EXEC` | 后端 `main.go` 校验与启动 | Node 可执行命令（如 `node`）。 |

注：`SSR_WORKER_ENTRY` 文件存在性、类型校验发生在后端启动阶段，见 `apps/server/cmd/server/main.go` 的 `validateSSRWorkerRuntime`。

---

## 5. 路由与请求归属（开发态 vs 生产态）

| 路径 | 开发态（`vite dev`） | 生产态 | 关键来源文件 |
| --- | --- | --- | --- |
| `/login` `/register` `/editor/*` `/admin/*` | Vite + React App | 后端 `web_spa.go` 回 `dist/index.html` | `apps/web/src/App.tsx`, `apps/server/internal/server/web_spa.go` |
| `/api/*` | Vite 代理到后端 | 后端 API | `apps/web/vite.config.ts`, `apps/server/internal/server/router.go` |
| `/r/*` | Vite 代理到后端阅读 SSR | 后端阅读页 SSR（Go -> Worker） | `apps/web/vite.config.ts`, `apps/server/internal/server/router.go` |
| `/uploads/*` | Vite 代理到后端 | 后端静态/图床回源 | `apps/web/vite.config.ts`, `apps/server/internal/server/router.go` |
| 其他非 Web 路由 | fallback 代理到后端 | 后端路由 | `apps/web/vite.config.ts` 的 fallback 正则 |

说明：

1. `vite.config.ts` 中 fallback 正则明确“仅 Web 路由由 Vite 处理，其余路径统一转发后端”。
2. 生产态中，后端通过 `WEB_DIST_DIR` 解析 SPA 产物并注册 `/login` `/register` `/editor/*` `/admin/*`。

---

## 6. 数据契约与网关（必须读）

### 6.1 契约真值来源

`apps/web/src/data-access/types.ts` 是前端契约真值来源，覆盖：

1. 基础实体：`User`、`Space`、`TreeNode`、`Document`、`DocumentRevision`。
2. 认证协议：`AuthLoginInput`、`AuthSession`、`AuthLoginOptions`。
3. 后台协议：`AdminUser`、`AdminSpace`、`AdminDocument`、`AdminTheme`、`AdminSystemConfig`、`AdminAudit*`。
4. 错误模型：`ConflictError`（文档/配置版本冲突）。

### 6.2 HTTP Adapter 行为模型

实现文件：`apps/web/src/data-access/http/adapter.ts`。

关键规则：

1. `request()` 统一组装 headers、鉴权、`credentials`、`JsonResult` 解析。
2. 若 `403 + code=1001`，会尝试 `refreshSession()`，成功后自动重放一次原请求。
3. 仍未恢复会话时，派发 `AUTH_UNAUTHORIZED_EVENT` 并清理本地 token。
4. 若 `code=4002/4010` 或 HTTP `409` 且有 `latestDocument`，抛 `ConflictError`。
5. `response.ok` 但 `JsonResult.code != 0` 同样视为业务失败。

### 6.3 认证失效事件链路

1. 事件定义：`apps/web/src/data-access/auth-events.ts`（`plaindoc.auth.unauthorized`）。
2. 事件触发：`adapter.ts` 在未授权场景派发。
3. 事件消费：`apps/web/src/App.tsx` 监听后执行登录态清理与跳转。

### 6.4 管理后台高风险写操作令牌

前端逻辑在 `adapter.ts`：

1. 先调用 `POST /admin/operation-tokens` 申请一次性 token。
2. 再在目标写请求头里注入 `X-Admin-Operation-Token`。
3. 后端中间件对 token 做操作者、操作类型、目标绑定校验。

### 6.5 认证风控（验证码 + 封禁）前端契约

契约文件与字段来源：

1. 类型定义：`apps/web/src/data-access/types.ts`
   - `AuthLoginInput`：新增 `captchaId`、`captchaAnswer`
   - `AuthRegisterInput`：注册入参新增 `captchaId`、`captchaAnswer`
   - `AuthCaptchaChallenge`：`captchaId`、`captchaImageDataUrl`、`level`、`expiresInSeconds`
   - `level` 语义为验证码位数（例如 `4/5/6`），用于提示“输入几位数字”
2. 网关实现：`apps/web/src/data-access/http/adapter.ts`
   - `HttpRequestError` 新增 `data` 字段，透传后端 `JsonResult.data`
3. 业务编排：`apps/web/src/App.tsx`
   - 解析风控错误码与 `data`
   - 维护 `authChallenge` 状态并分发给登录/注册与后台登录组件

错误码约定（前端必须识别）：

1. `1008`：`CAPTCHA_REQUIRED`
2. `1009`：`CAPTCHA_INVALID`
3. `1010`：`AUTH_TEMPORARILY_LOCKED`

页面组件消费点：

1. `apps/web/src/components/AuthPanel.tsx`
   - 登录/注册共用验证码展示与提交
2. `apps/web/src/components/AdminAuthPanel.tsx`
   - 后台登录同样支持验证码会话
3. `apps/web/src/admin/AdminApp.tsx`
   - 透传 `authChallenge` 到后台登录面板

状态流约束：

1. 收到 `1008/1009` 时：
   - 展示后端返回的 `captchaImageDataUrl`
   - 下一次提交必须附带 `captchaId + captchaAnswer`
2. 收到 `1010` 时：
   - 清空验证码会话
   - 展示封禁提示（优先使用 `retryAfterSeconds`/`lockedUntil`）
3. 登录成功、登出或会话切换时必须清空 `authChallenge`，防止复用旧会话。

---

## 7. 编辑器预览链路与硬约束

### 7.1 sanitize 与 Markdown 渲染

关键文件：

1. `apps/web/src/editor/markdown-sanitize.ts`
2. `apps/web/src/editor/markdown-shared.ts`
3. `apps/web/src/editor/constants.ts`

约束要点：

1. `rehype-raw` 开启后必须配合 sanitize 白名单。
2. `data*` 属性必须保留，否则滚动映射/锚点功能会失效。
3. `code` 节点需保留 `math-inline` / `math-display`，否则公式渲染回归。
4. 链接/资源协议白名单只放行安全协议（阻断 `javascript:` 等）。

### 7.2 滚动同步依赖项

关键实现：`apps/web/src/editor/use-scroll-sync.ts`。

稳定契约：

1. 锚点选择器来自 `BLOCK_ANCHOR_SELECTOR`。
2. 预览正文容器选择器来自 `PREVIEW_BODY_SELECTOR`。
3. 预览主题/自定义样式变更后要重建映射，否则出现漂移。

### 7.3 预览样式注入

`apps/web/src/main.tsx` 使用 `?inline` + `<style>` 注入：

1. KaTeX 样式
2. 应用样式
3. Google Sans Code 字体样式

目的：避免依赖外部 `<link>`，降低构建路径与 SSR 样式不一致风险。

---

## 8. 阅读页 SSR（前端侧）深度说明

### 8.1 SSR Worker 构建配置逐项含义

配置文件：`apps/web/vite.ssr.config.ts`。

| 配置项 | 当前值 | 含义 | 影响 |
| --- | --- | --- | --- |
| `build.ssr` | `src/ssr/worker-entry.ts` | Worker 入口 | 生成可被后端启动的 SSR Worker 程序。 |
| `build.outDir` | `dist-ssr` | 输出目录 | 后端通常读取 `apps/web/dist-ssr/worker-entry.js`。 |
| `build.assetsDir` | `web-assets` | 资源目录前缀 | 与客户端构建对齐，避免 `/assets` 路由冲突。 |
| `build.ssrEmitAssets` | `true` | SSR 构建也落盘静态资源 | 保证字体/KaTeX 等资源可用。 |
| `build.emptyOutDir` | `true` | 构建前清理目录 | 避免历史 hash 文件污染发布。 |
| `build.target` | `node20` | Worker 运行时目标 | 与部署 Node 版本保持一致。 |
| `build.minify` | `false` | 禁止压缩 | 便于排障与堆栈定位。 |
| `build.sourcemap` | `true` | 生成 sourcemap | 便于线上错误追踪。 |
| `ssr.noExternal` | `true` | 强制内联依赖 | 降低外部依赖分发复杂度。 |
| `rollupOptions.output.entryFileNames` | `worker-entry.js` | 固定入口文件名 | 简化后端 `SSR_WORKER_ENTRY` 配置。 |

### 8.2 构建产物与存在性检查

默认产物：

1. Worker 入口：`apps/web/dist-ssr/worker-entry.js`
2. Worker 资源：`apps/web/dist-ssr/web-assets/*`

根目录 `Makefile` 已提供前置检查：

1. `make server-dev-ssr` 会先检查 `dist-ssr/worker-entry.js` 是否存在。
2. 后端启动时再次校验 `SSR_WORKER_ENTRY` 必须存在且是文件。

### 8.3 Worker 协议与渲染流程

关键文件：

1. `apps/web/src/ssr/protocol.ts`（握手 + render 消息结构）
2. `apps/web/src/ssr/worker-entry.ts`（stdin JSONL -> render -> stdout JSONL）
3. `apps/web/src/ssr/render-space-reader.tsx`（渲染完整 HTML）

流程：

1. 后端发送 `handshake`，校验 `version`。
2. 后端发送 `render`（含 `id`、`route`、`payload`）。
3. Worker 返回 `{ ok: true, html, head, metrics }` 或错误对象。

### 8.4 阅读页 DOM Hook 契约（不能随意改）

`render-space-reader.tsx` 与 `render-space-reader.async-script.ts` 共同依赖：

1. `data-reader-doc-link`
2. `data-reader-doc-id`
3. `data-reader-hook='tree-*'`
4. `data-reader-hook='article-shell'`
5. `data-reader-hook='progress'`
6. `data-reader-hook='main'`
7. `data-reader-hook='sidebar'`
8. `data-reader-active`
9. `data-reader-label-active`
10. `data-reader-hook='outline'` / `outline-link`

规则：异步增强脚本应依赖 `data-reader-*`，而不是视觉 class。

---

## 9. 管理后台前端规则

关键文件：

1. `apps/web/src/admin/AdminApp.tsx`
2. `apps/web/src/admin/routes.ts`
3. `apps/web/src/components/ui/dropdown-menu.tsx`
4. `apps/web/scripts/check-dropdown-menu-modal.mjs`

规则：

1. 菜单由管理员角色动态生成：`platform_admin` 与 `space_admin` 权限不同。
2. `DropdownMenu` 默认 `modal=false`，禁止写 `modal=true`。
3. 门禁脚本会扫描源码：
   - 阻止绕过封装直接 `import @radix-ui/react-dropdown-menu`
   - 阻止 `<DropdownMenu modal={true}>` 与等价写法

---

## 10. 开发、构建、发布（推荐命令）

优先使用根目录 `Makefile`：

1. `make install`：安装依赖。
2. `make web-dev`：启动前端开发服务。
3. `make server-dev`：启动后端（默认关闭 SSR Worker）。
4. `make web-build`：构建 `dist + dist-ssr`。
5. `make web-build-ssr`：仅构建 `dist-ssr`。
6. `make server-dev-ssr`：检查 Worker 产物后启动后端 SSR。

等价 npm 命令：

1. `npm run web:dev`
2. `npm run web:build`
3. `npm run web:build-ssr`

前端最小门禁：

1. `npm run check:dropdown-menu -w @plaindoc/web`
2. `npm run web:build`

高风险改动附加验收（按需执行）：

1. 涉及后台权限/治理：`cd apps/server && go test ./internal/server -run TestRouter_Admin -count=1`
2. 涉及迁移与数据层：`cd apps/server && go test ./internal/storage/... -count=1`
3. 涉及滚动同步/sanitize/主题样式：
   - 手工验证长文档双向滚动（含长图）
   - 主题切换后再次滚动无漂移
   - 含内嵌 HTML 的 Markdown 正常渲染且恶意协议被拦截

---

## 11. 高频问题与排障路径

1. `make server-dev-ssr` 报 `SSR_WORKER_ENTRY` 相关错误：
   - 先执行 `make web-build-ssr`
   - 确认 `apps/web/dist-ssr/worker-entry.js` 存在且是文件
2. 开发态 `/r` 或 `/uploads` 404：
   - 检查 `VITE_DEV_PROXY_TARGET`
   - 检查 `vite.config.ts` 代理配置是否被改动
3. 登录态频繁丢失：
   - 检查 `adapter.ts` 的 `403 + code=1001` 刷新与事件处理链路
4. 文档保存冲突处理失效：
   - 检查 `ConflictError` 分支（`4002/4010` 与 HTTP 409）
5. 阅读页目录激活/折叠异常：
   - 检查 `data-reader-*` hooks 是否被修改
6. 预览滚动同步漂移：
   - 检查 sanitize 是否误删 `data*`
   - 检查主题/样式变更后是否触发映射重建
7. 菜单弹出后页面“像被锁住”：
   - 检查是否出现 `DropdownMenu modal=true`

---

## 12. 快速定位（按任务）

1. 路由解析与认证回跳：`apps/web/src/App.tsx`
2. 会话鉴权与重试：`apps/web/src/data-access/http/adapter.ts`
3. 契约类型定义：`apps/web/src/data-access/types.ts`
4. 预览 sanitize 与 HTML 白名单：`apps/web/src/editor/markdown-sanitize.ts`
5. 滚动同步：`apps/web/src/editor/use-scroll-sync.ts`
6. 后台角色菜单与路由：`apps/web/src/admin/AdminApp.tsx`
7. SSR Worker 协议入口：`apps/web/src/ssr/worker-entry.ts`
8. 阅读页 SSR HTML：`apps/web/src/ssr/render-space-reader.tsx`
9. 阅读页异步增强：`apps/web/src/ssr/render-space-reader.async-script.ts`
10. DropdownMenu 规范门禁：`apps/web/scripts/check-dropdown-menu-modal.mjs`

---

## 13. 文档维护约定

1. 本文件是前端唯一口径文档。
2. 涉及配置、契约、SSR、路由、门禁规则变更时，必须同步更新本文件。
3. 新增说明必须附“来源文件 + 消费位置”，避免再次出现口径漂移。
