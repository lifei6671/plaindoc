# Technical Proposal: 空间与文档阅读页 SSR（Go 拉起 Node 子进程）

**Project Type**: 在现有 `apps/server` + `apps/web` 架构上新增阅读页 SSR 能力  
**Deployment Constraint**: 不新增独立 Node HTTP 服务；由 Go 进程拉起并管理 Node 子进程  
**Scope**: 空间阅读页（左侧文档树 + 右侧文档阅读区），无编辑器顶部操作工具栏  
**Primary Goal**: 在不改变当前部署拓扑的前提下，提供可爬取 SSR 首屏，同时保证阅读渲染效果与编辑器预览一致

---

## 一、背景与目标

### 1.1 目标

1. 新增“空间阅读页”SSR：
   - 页面结构与编辑器主体相似：左侧文档树、右侧阅读区。
   - 不包含编辑器顶部语法工具栏与编辑相关操作。
2. 维持单入口部署：不额外部署 Node Web 服务，不引入 Go->Node HTTP 内网调用链路。
3. 保证渲染一致性：阅读页 SSR 输出与当前前端 `react-markdown` 渲染风格保持一致，避免“同文档不同页面视觉差异”。

### 1.2 非目标（首期）

1. 不改造为 Next.js/Nuxt 等全站同构框架。
2. 不在首期覆盖编辑态“每次输入都后端渲染”。
3. 不做多机分布式 SSR Worker 调度（首期限定单实例内进程池）。

---

## 二、总体架构

### 2.1 架构总览

1. 浏览器请求阅读页路由（例如 `/r/:spaceId/:docId`）进入 Go（Gin）服务。
2. Go 完成鉴权、权限校验、数据聚合（空间、文档树、当前文档、主题配置）。
3. Go 通过 `stdin/stdout` 向 Node SSR Worker 发送渲染请求（JSONL 协议）。
4. Node Worker 使用 React SSR（`renderToString`）生成 HTML 片段并返回。
5. Go 将 SSR 结果填入页面壳（含初始状态、静态资源链接），返回最终 HTML。

### 2.2 核心原则

1. **Go 负责业务与数据，Node 只负责视图渲染**。  
2. **Node Worker 无数据库权限、无业务鉴权逻辑**。  
3. **Node 子进程常驻，不按请求冷启动**。  
4. **Go 持有进程生命周期与容错控制权**（超时、重启、熔断、降级）。

---

## 三、子进程通信方案（非 HTTP）

### 3.1 通信介质

1. Go `os/exec` 启动 Node Worker：
   - `stdin`：Go -> Worker 请求流
   - `stdout`：Worker -> Go 响应流
   - `stderr`：Worker 结构化日志
2. 协议采用 JSON Lines（每行一个完整 JSON 消息），便于流式解码与排障。

### 3.2 消息模型（建议）

1. 请求消息：
   - `id`: string（请求唯一 ID）
   - `type`: `"render"`
   - `route`: string（如 `space-reader`）
   - `payload`: object（渲染所需数据）
   - `deadlineMs`: number（Worker 内部软超时）
2. 响应消息：
   - `id`: string（与请求对应）
   - `ok`: boolean
   - `html`: string（`ok=true` 时）
   - `head`: object（title/meta/canonical 等）
   - `error`: object（`ok=false` 时，含 code/message/stack 简化信息）
   - `metrics`: object（renderMs、payloadBytes 等）

### 3.3 并发与顺序

1. 单 Worker 内支持多请求并发处理，但返回顺序不保证。
2. Go 侧通过 `id -> chan` 映射做响应路由。
3. Go 可维护 N 个 Worker（建议 2~4）形成本地池，按最小负载派发。

### 3.4 超时与容错

1. 请求超时：Go `context deadline` 到期即取消等待，返回降级页面。
2. Worker 卡死：Go 发现超时/读写异常后 `kill` 并自动拉起替代进程。
3. 半开熔断：短时间连续失败超过阈值，暂时切换 CSR 壳页，后台探活恢复。

---

## 四、渲染一致性设计（重点）

### 4.1 单渲染逻辑源

1. 将当前 Markdown 渲染链路抽为可复用模块（同一份 TS 代码）：
   - `remark` 插件列表与顺序
   - `rehype` 插件列表与顺序
   - sanitize 白名单策略
   - 代码高亮语言注册与 alias
   - 主题变量与代码块样式映射
2. CSR（现有阅读/预览）与 SSR（Node Worker）共同依赖该模块，避免双实现漂移。

### 4.2 与当前项目对齐项

1. 保持与现有 `react-markdown` 组件体系一致（标题结构、TOC、代码块、数学公式、脚注等）。
2. 保持现有预览主题体系一致（主题变量、代码高亮主题、自定义样式注入）。
3. 对 Mermaid 等纯客户端增强内容，SSR 首屏输出稳定占位结构，客户端再增强渲染，避免 hydration mismatch。

### 4.3 一致性验收机制

1. 同一 Markdown 输入，SSR 与 CSR 产物做 DOM 快照对比（忽略随机 ID/时间戳）。
2. 关键样例集覆盖：
   - 表格、任务列表、脚注、数学公式、代码块、多级标题、引用、图片、HTML 片段。
3. 引入视觉回归（截图对比）确保样式层一致。

---

## 五、阅读页产品与路由方案

### 5.1 路由建议

1. 阅读页路由：`GET /r/:spaceId/:docId`
2. 可选兼容路由：`GET /spaces/:spaceId/docs/:docId`
3. 编辑页与阅读页明确分离，避免工具栏与编辑交互泄漏到阅读页。

### 5.2 页面结构

1. 左栏：文档树（可折叠、可展开、可选中文档高亮）。
2. 右栏：Markdown 阅读区（只读）。
3. 顶部：不展示编辑语法工具栏；保留必要导航（返回空间/返回管理）可单独定义。

### 5.3 首屏与交互策略

1. SSR 返回完整首屏 HTML（含已选中文档内容）。
2. 客户端 hydration 后接管树切换与局部导航。
3. 文档切换可首期走客户端拉取 + CSR 渲染；后续可升级为局部 SSR 片段返回。

---

## 六、Go 侧实现设计

### 6.1 模块划分（建议）

1. `apps/server/internal/ssr/protocol`：消息结构、编码解码。
2. `apps/server/internal/ssr/worker`：单 Worker 生命周期与 IO 循环。
3. `apps/server/internal/ssr/pool`：Worker 池、负载均衡、健康检查。
4. `apps/server/internal/service/reader_page_service.go`：数据聚合（空间树 + 文档 + 权限）。
5. `apps/server/internal/server/handler/reader_page.go`：阅读页 HTTP Handler。

### 6.2 渲染流程（Go Handler）

1. 解析路由参数并校验。
2. 鉴权与 ACL 判定（匿名/登录态均支持）。
3. 查询并组装 `ReaderPageViewModel`。
4. 查询 SSR 缓存（命中则直接返回）。
5. 未命中则发请求至 Worker 池渲染。
6. 成功后写缓存并返回；失败则按降级策略返回 CSR 壳页。

### 6.3 缓存策略（建议）

1. Key 维度：
   - `spaceId`
   - `docId`
   - `docVersion`
   - `themeId`
   - `viewerSegment`（anonymous/authenticated/member）
2. 公开空间可较长缓存；私有空间建议短缓存或不缓存。
3. 文档更新后主动失效相关 key（按 `docId` 前缀清理）。

---

## 七、Node Worker 侧实现设计

### 7.1 Worker 职责

1. 启动时加载 SSR Bundle（只做一次）。
2. 监听 `stdin` 消息，执行 `render(request)`。
3. 输出 `stdout` 响应消息。
4. 输出结构化错误与性能日志到 `stderr`。

### 7.2 Worker 输入数据约束

1. 仅接收渲染必需数据，不传敏感字段（如 token 原文）。
2. `payload` 体积限制（例如 1MB），超限直接拒绝。
3. 所有字符串字段做长度上限防护，避免恶意大报文占用内存。

### 7.3 渲染实现

1. React `renderToString`（或 `renderToPipeableStream`，首期可先 `renderToString`）。
2. 调用共享 Markdown 渲染模块生成阅读区内容。
3. 返回：
   - `html`（页面主体或可嵌入片段）
   - `head`（title/meta）
   - `metrics`（渲染耗时）

---

## 八、构建与发布方案

### 8.1 前端构建产物

1. 新增 SSR 构建脚本（例如）：
   - Client Bundle：现有 `vite build`
   - SSR Bundle：`vite build --ssr src/ssr/worker-entry.tsx`
2. 输出目录建议：
   - `apps/web/dist`（客户端）
   - `apps/web/dist-ssr`（Worker 渲染逻辑）

### 8.2 后端启动检查

1. Go 启动时检查：
   - Node 可执行文件是否存在
   - `dist-ssr` 产物是否存在
2. 若缺失：
   - 明确告警日志
   - 可配置“阻止启动”或“降级为 CSR”

### 8.3 部署约束

1. 单体部署包内包含：
   - Go 二进制
   - Web 静态资源
   - SSR Bundle
   - Node 运行时（或系统预装 Node）
2. 版本一致性要求：Go 与 SSR Bundle 必须同版本发布，避免协议不匹配。

---

## 九、安全与稳定性

### 9.1 安全

1. Worker 不直接访问数据库和外部网络（按运行环境限制）。
2. 严格限制消息大小、并发数、最大渲染耗时。
3. SSR 输出继续经过统一 sanitize 规则（与前端一致）。

### 9.2 稳定性

1. Worker 进程崩溃自动拉起。
2. 周期性健康检查（ping/pong）。
3. 内存阈值守护（超阈值重启 Worker）。
4. 错误熔断与快速降级，保证主业务可用。

---

## 十、监控与排障

### 10.1 指标

1. `ssr_render_requests_total`  
2. `ssr_render_failures_total`  
3. `ssr_render_duration_ms`（P50/P95/P99）  
4. `ssr_worker_restarts_total`  
5. `ssr_queue_depth`  

### 10.2 日志

1. Go 日志带 `requestId`、`workerId`、`renderId`。
2. Worker 日志透传 `renderId`，便于全链路关联。
3. 失败时记录“协议错误/超时/渲染异常/降级类型”。

---

## 十一、分阶段落地计划

## Phase 1: 协议与进程池基础

1. 完成 Go `WorkerPool` 与 JSONL 协议通信。
2. 完成 Node Worker demo（返回固定 HTML）。
3. 打通超时、重启、降级分支。

**验收**：
1. Worker 异常退出可自动恢复。
2. 压测下无请求串线（`id` 映射正确）。

## Phase 2: 阅读页 SSR 最小闭环

1. 新增阅读页路由与 Handler。
2. 接入真实空间树 + 文档内容数据。
3. SSR 输出基础布局（左树右文）。

**验收**：
1. 关闭 JS 时可看到完整阅读内容。
2. 无权限请求返回正确状态码（403/404）。

## Phase 3: 渲染一致性收口

1. 抽取并复用 Markdown 渲染共享模块。
2. 补齐主题、代码高亮、脚注、数学公式一致性。
3. 完成快照测试与视觉回归测试。

**验收**：
1. 样例集 SSR 与 CSR DOM 差异在允许范围内。
2. 无关键样式回归。

## Phase 4: 性能与发布加固

1. 上线缓存策略与主动失效机制。
2. 增加核心指标与告警。
3. 完成灰度发布与回滚预案。

**验收**：
1. 首屏 TTFB 与渲染成功率满足目标。
2. Worker 异常不会导致全站不可用。

---

## 十二、风险与应对

1. 风险：Worker 内存增长导致不稳定。  
   应对：进程池隔离 + 内存阈值重启 + 请求超时。  
2. 风险：SSR/CSR 渲染链路分叉。  
   应对：强制共享渲染模块 + 快照测试门禁。  
3. 风险：协议演进导致 Go/Node 版本不兼容。  
   应对：协议版本字段 + 启动时握手校验。  
4. 风险：高峰期渲染排队增加延迟。  
   应对：缓存 + Worker 池扩容 + 降级策略。  

---

## 十三、验收标准（最终）

1. 不引入独立 Node HTTP 服务，Go 可独立拉起并管理 Node Worker。  
2. 阅读页 SSR 首屏可被搜索引擎抓取，且无 JS 依赖。  
3. 阅读页渲染效果与编辑器预览核心样例保持一致。  
4. Worker 异常情况下服务可自动恢复或快速降级。  
5. 具备可观测性（指标、日志、告警）与可回滚方案。  

