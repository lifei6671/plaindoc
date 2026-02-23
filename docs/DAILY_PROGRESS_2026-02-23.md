# 2026-02-23 开发同步（空间阅读 SSR + 编辑器联动优化）

## 1) 今日新增功能

### 1.1 空间阅读页 SSR（Go 拉起 Node 子进程）

- 新增阅读路由：
  - `GET /r/:spaceId`（自动解析并跳转空间首篇可读文档）
  - `GET /r/:spaceId/:docId`（空间文档阅读页）
- Go 侧新增 SSR 子进程能力：
  - 配置项：`SSR_WORKER_*`（启停、入口、进程数、超时、协议版本、payload 上限）
  - 进程池：Worker 启动、握手、渲染分发、异常重启（不可用时重启重试）
  - 协议：`stdin/stdout` JSONL（`handshake` + `render`）
- Web 侧新增 SSR Worker：
  - `apps/web/src/ssr/worker-entry.ts`
  - `apps/web/src/ssr/render-space-reader.tsx`
  - `apps/web/src/ssr/markdown-shared.ts`
  - `apps/web/vite.ssr.config.ts`
- SSR 页面结构落地：
  - 左侧文档树（层级、激活态、可折叠）
  - 右侧文档阅读区（Markdown SSR 输出）
  - 标题下元信息（空间、版本、最后编辑时间）

### 1.2 阅读页体验与一致性

- 阅读页文档树视觉样式与编辑态树结构对齐（含缩进层级、激活态）。
- 阅读页内容样式与编辑器预览对齐：
  - SSR 页面注入 `styles.css` 内联样式，补齐 `#plaindoc-preview-body` 规则。
- 阅读页去除编辑同步专用锚点属性：
  - 关闭 SSR 链路的 `remarkBlockAnchorPlugin`，不再输出 `data-source-line`/`data-anchor-index` 等属性。
- 标题下新增“友好时间”：
  - 例如 `最后编辑：3天前（2026-02-20 09:30）`
  - 与空间/版本元数据合并为一行展示。

### 1.3 路由、代理与错误处理补齐

- 首页和分类页空间卡片跳转由编辑页改为阅读页：`/r/:spaceId`。
- 开发环境 Vite 代理补齐：
  - `/r` 代理到后端（阅读 SSR）
  - `/uploads` 代理到后端（图片静态访问）
- 阅读页错误处理补齐：
  - 空间/文档不存在：友好错误页
  - 无权限：友好错误页
  - 需要登录：重定向登录页并保留回跳参数
  - SSR 失败：降级基础阅读页（非 500 阻断）

### 1.4 编辑器相关优化（与阅读链路联动）

- 自动保存节流策略回归一致：HTTP 模式恢复为 `800ms`，降低冲突风暴概率。
- 图片上传链路与阅读渲染打通：
  - 插入后使用可访问 URL（去掉 `/api` 前缀）
  - 阅读页可直接命中后端静态目录
  - 上传鉴权遵循角色约束（reader 不可上传）
- 中间件日志新增可读耗时字段：
  - `latency_human`（如 `33.455µs`）

---

## 2) 已完成里程碑（按阅读 SSR 方案）

对应文档：`docs/SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`

### 2.1 已完成

- Milestone 1（协议与配置基线）：完成
  - `Config` 与 `.env.example` 已补齐 SSR 配置
  - 启动日志输出 SSR 摘要
- Milestone 3（Node Worker 与 SSR 构建）：完成
  - Worker 入口、SSR 渲染函数、构建脚本、产物目录均已落地
- Milestone 4（阅读路由与数据聚合）：完成
  - 路由、Handler、Service、权限校验、错误分流、降级链路已打通

### 2.2 核心完成（与原计划略有差异）

- Milestone 2（Go 子进程监督器）：核心功能已可用
  - 已实现：握手、渲染、进程池、不可用重启重试
  - 未完全按原清单落地：最小 in-flight 选择、完整测试矩阵
- Milestone 5（阅读端页面形态）：核心页面已落地
  - 已实现：左树右文、可折叠树、样式一致性
  - 本期未引入独立 Reader hydration 模块，先以 SSR 可用性和一致性优先

### 2.3 待后续阶段

- Milestone 6：一致性自动化测试与回归脚本
- Milestone 7：缓存、指标、发布回滚加固

---

## 3) 今日踩坑指南（问题 / 根因 / 处理 / 建议）

### 3.1 阅读页样式与编辑预览不一致

- 问题：阅读区字体、标题、表格、代码块样式明显“退化”。
- 根因：SSR 输出只注入了局部样式，未带 `styles.css` 中的预览规则。
- 处理：在 `render-space-reader.tsx` 内联注入 `../styles.css?inline`。
- 建议：阅读页样式统一以 `#plaindoc-preview-body` 为单一源，避免多套 CSS 分叉。

### 3.2 `/uploads` 图片链接在 3001 端口无法访问

- 问题：Markdown 图片 URL 可生成，但浏览器请求返回 `route not found`。
- 根因：Vite 仅代理 `/api`，未代理 `/uploads` 与 `/r`。
- 处理：补充 `vite.config.ts` 代理规则：
  - `/r` -> backend
  - `/uploads` -> backend
- 建议：本地开发路由统一走 `3001`，并集中在 Vite 代理维护白名单。

### 3.3 文档存在但阅读页报 `document not found`

- 问题：按 `docId` 访问返回不存在，实际数据在库中。
- 根因：历史链路可能传入 `node_id`，且大小写/映射存在差异。
- 处理：服务层增加 `document_id/node_id` 双通道解析与 canonical 重定向。
- 建议：外部链接统一使用 canonical `document_id`。

### 3.4 SQLite 时间字段扫描异常

- 问题：`updated_at` 扫描时报 `unsupported Scan ... string into *time.Time`。
- 根因：SQLite 驱动返回字符串，结构体直接用 `time.Time` 扫描会失败。
- 处理：先按字符串接收，再用多 layout 解析为 RFC3339 输出。
- 建议：SQLite 兼容场景下，时间字段尽量先走字符串归一化。

### 3.5 配置写了 `.env` 但启动不生效

- 问题：`apps/server/.env` 与 `apps/server/cmd/server/.env` 均不生效。
- 根因：启动 cwd 不固定，默认只读单路径会漏文件。
- 处理：`main.go` 增加 `loadDotEnvCandidates()`，按候选路径顺序加载，且保留系统环境变量优先级。
- 建议：本地启动统一检查 `server starting` 日志中的配置摘要确认是否生效。

### 3.6 阅读 DOM 暴露编辑同步锚点属性

- 问题：阅读页面出现大量 `data-source-line` 等调试/同步属性。
- 根因：阅读链路复用了编辑链路的 block anchor 插件。
- 处理：为共享 remark 插件增加开关，阅读 SSR 关闭锚点注入。
- 建议：共享渲染链路必须支持“编辑态能力开关”，避免阅读面泄露编辑内部实现。

### 3.7 阅读页异步脚本与 DOM 选择器耦合风险

- 问题：阅读页右侧异步加载脚本如果依赖 class 选择器，后续样式重构容易导致点击、折叠、激活态、Tooltip 定位失效。
- 根因：class 同时承担“视觉语义 + 交互语义”，改样式时容易误伤交互逻辑。
- 处理：统一切换为 `data-reader-*` 钩子契约，脚本只依赖 hook，不依赖视觉 class。
- 建议：
  - 后续迭代中，阅读页可以改 class，但必须保留并校验 hook。
  - 详细约束见：`docs/SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md` 的“0.6 阅读页 DOM Hook 契约”。

---

## 4) 涉及文件（本日重点）

- 后端：
  - `apps/server/internal/config/config.go`
  - `apps/server/cmd/server/main.go`
  - `apps/server/internal/ssr/*`
  - `apps/server/internal/server/router.go`
  - `apps/server/internal/server/handler/reader_page.go`
  - `apps/server/internal/service/reader_page_service.go`
  - `apps/server/internal/service/reader_view_model.go`
  - `apps/server/internal/server/view/templates/partials/space_cover_card.tmpl`
- 前端：
  - `apps/web/src/ssr/*`
  - `apps/web/vite.ssr.config.ts`
  - `apps/web/vite.config.ts`
  - `apps/web/src/editor/markdown-shared.ts`
  - `apps/web/package.json`
  - `apps/web/src/ssr/render-space-reader.tsx`
