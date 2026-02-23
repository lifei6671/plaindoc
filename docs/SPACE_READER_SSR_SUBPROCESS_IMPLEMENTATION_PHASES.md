# Implementation Phases: 空间阅读页 SSR（Go 拉起 Node 子进程）

**Project Type**: 现有项目新增阅读页 SSR（无独立 Node HTTP 服务）  
**Scope**: 空间阅读页（左侧文档树 + 右侧阅读区）  
**Stack**: Server（Gin + GORM + Go Supervisor）+ Web（React + Vite SSR Bundle）  
**Primary Goal**: 保持单部署入口（Go），由 Go 管理 Node Worker 子进程完成 SSR，且阅读渲染与编辑器预览一致

关联方案文档：`docs/SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`

---

## 零、执行边界（先统一）

1. 不新增独立 Node HTTP 服务；Go 与 Node 只允许 `stdin/stdout` JSONL 协议通信。  
2. Node Worker 不接数据库，不做鉴权；仅渲染。  
3. 渲染一致性以“共享 Markdown 渲染模块”为唯一标准，不接受 Go/TS 双实现分叉。  
4. 首期路由只做阅读页，不改造编辑器现有保存链路。  
5. 首期必须保留降级能力：Worker 不可用时仍能回退 CSR 壳页，不能阻断主服务。

---

## Milestone 1: 协议与配置基线
**Status**: Pending  
**Type**: Infrastructure  
**Estimated**: 1~2 天  
**Files**:
- `apps/server/internal/config/config.go`
- `apps/server/.env.example`
- `apps/server/cmd/server/main.go`
- `docs/SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`（补充协议版本号）

**Tasks**:
- [ ] 在 `Config` 增加 SSR 子进程配置：
  - `SSRWorkerEnabled`
  - `SSRWorkerExec`（默认 `node`）
  - `SSRWorkerEntry`（SSR worker 入口 JS 文件路径）
  - `SSRWorkerCount`
  - `SSRRenderTimeout`
  - `SSRWorkerStartTimeout`
  - `SSRWorkerMaxPayloadBytes`
  - `SSRProtocolVersion`
- [ ] 为配置增加 `Validate` 规则（数量、超时、路径非空、布尔值合法）。  
- [ ] 更新 `apps/server/.env.example`，增加全部 SSR 环境变量样例。  
- [ ] 启动日志打印 SSR 配置摘要（脱敏后）。

**Verification Criteria**:
- [ ] 配置缺失/非法时启动即失败且报错可读。  
- [ ] `SSRWorkerEnabled=false` 时主服务行为与当前版本一致。  

**Exit Criteria**:
- [ ] SSR 基础配置可通过环境变量完整驱动，无硬编码路径。

---

## Milestone 2: Go 子进程监督器与协议实现
**Status**: Pending  
**Type**: Backend Core  
**Estimated**: 2~4 天  
**Files**:
- `apps/server/internal/ssr/protocol/messages.go`（新增）
- `apps/server/internal/ssr/worker/process.go`（新增）
- `apps/server/internal/ssr/worker/stdio_codec.go`（新增）
- `apps/server/internal/ssr/pool/pool.go`（新增）
- `apps/server/internal/ssr/pool/dispatcher.go`（新增）
- `apps/server/internal/ssr/pool/pool_test.go`（新增）
- `apps/server/internal/ssr/worker/process_test.go`（新增）

**Tasks**:
- [ ] 定义 JSONL 消息结构（request/response/error/metrics）。  
- [ ] 实现单 Worker 生命周期管理：
  - 启动、握手、健康检查、超时取消
  - IO 读写循环
  - 崩溃检测与退出码上报
- [ ] 实现 WorkerPool：
  - 固定进程数
  - 负载选择（最小 in-flight）
  - 请求 `id` 路由
  - 超时回收
- [ ] 实现故障恢复：
  - worker 异常退出自动拉起
  - 连续失败熔断窗口
  - 熔断期间降级标记
- [ ] 增加协议版本校验（握手时不匹配直接拒绝）。

**Verification Criteria**:
- [ ] 并发 100 请求下无串线、无泄漏（请求 ID 全匹配）。  
- [ ] 人为 kill worker 后 1 个健康周期内自动恢复。  
- [ ] 超时请求不会卡死 goroutine 或阻塞后续请求。  

**Exit Criteria**:
- [ ] Go 已可稳定调用本地 Node Worker 并得到 HTML 响应。

---

## Milestone 3: Node Worker 与 SSR Bundle 构建
**Status**: Pending  
**Type**: Web Build + Runtime  
**Estimated**: 2~3 天  
**Files**:
- `apps/web/src/ssr/worker-entry.ts`（新增）
- `apps/web/src/ssr/render-space-reader.tsx`（新增）
- `apps/web/src/ssr/protocol.ts`（新增）
- `apps/web/src/ssr/ssr-types.ts`（新增）
- `apps/web/src/ssr/markdown-shared.ts`（新增，复用现有渲染链路）
- `apps/web/package.json`
- `apps/web/vite.config.ts`（或新增 `apps/web/vite.ssr.config.ts`）
- `apps/web/tsconfig.node.json`（纳入 SSR 源码）

**Tasks**:
- [ ] 新增 Node Worker 入口，监听 stdin 并输出 JSONL 响应。  
- [ ] 抽取“阅读页 SSR 渲染函数”，输入 `payload` 输出 `html/head/metrics`。  
- [ ] 抽取共享 Markdown 渲染模块，避免与现有 `editor` 渲染分叉。  
- [ ] 新增构建脚本：
  - `build:client`
  - `build:ssr-worker`
  - `build` 聚合二者
- [ ] 固化 SSR 产物目录（例如 `apps/web/dist-ssr/worker-entry.js`）。

**Verification Criteria**:
- [ ] `node dist-ssr/worker-entry.js` 可独立启动并响应本地 JSONL 请求。  
- [ ] Worker 遇到异常能返回结构化错误，不会输出非协议垃圾到 stdout。  

**Exit Criteria**:
- [ ] 产物构建可用于 Go 子进程直接拉起。

---

## Milestone 4: 阅读页后端路由与数据聚合
**Status**: Pending  
**Type**: API + SSR Handler  
**Estimated**: 2~4 天  
**Files**:
- `apps/server/internal/server/router.go`
- `apps/server/internal/server/handler/reader_page.go`（新增）
- `apps/server/internal/service/reader_page_service.go`（新增）
- `apps/server/internal/service/reader_view_model.go`（新增）
- `apps/server/internal/storage/repository/interfaces.go`（按需扩展）
- `apps/server/internal/storage/repository/gorm_space_repository.go`（按需扩展）
- `apps/server/internal/storage/repository/gorm_document_repository.go`（按需扩展）
- `apps/server/internal/server/reader_page_handler_test.go`（新增）

**Tasks**:
- [ ] 新增阅读路由：`GET /r/:spaceId/:docId`。  
- [ ] 在 handler 中完成：
  - 参数校验
  - 身份识别（匿名/登录）
  - ACL 校验（是否可读）
  - 调用 `reader_page_service` 组装 SSR payload
  - 调用 WorkerPool 渲染
- [ ] 回包策略：
  - 成功：返回完整 HTML
  - 失败：返回 CSR 壳页 + 初始状态（可重试）
- [ ] 明确缓存头与 `Vary`（至少 `Authorization, Cookie`）。

**Verification Criteria**:
- [ ] 有权限用户返回 200 且包含正文 HTML。  
- [ ] 无权限返回 403；不存在返回 404。  
- [ ] Worker 故障时不 500 雪崩，走降级分支。  

**Exit Criteria**:
- [ ] 阅读页 SSR 闭环可用。

---

## Milestone 5: 阅读端 Hydration 与页面组件落地
**Status**: Pending  
**Type**: UI + Integration  
**Estimated**: 2~4 天  
**Files**:
- `apps/web/src/reader/ReaderApp.tsx`（新增）
- `apps/web/src/reader/reader-main.tsx`（新增）
- `apps/web/src/reader/ReaderLayout.tsx`（新增）
- `apps/web/src/reader/ReaderDocumentTree.tsx`（新增，或复用 `WorkspaceTree` 只读模式）
- `apps/web/src/reader/ReaderMarkdownView.tsx`（新增）
- `apps/web/src/editor/markdown-components.tsx`（复用/拆分）
- `apps/web/src/editor/markdown-sanitize.ts`（复用）
- `apps/web/src/styles.css`（补充阅读页样式块）

**Tasks**:
- [ ] 落地阅读页组件结构（左树右文，无编辑工具栏）。  
- [ ] Hydration 入口读取后端注入的初始状态并接管交互。  
- [ ] 文档树点击切换文档（首期可客户端拉 API）。  
- [ ] 确保阅读区 Markdown 样式与编辑器预览一致。  
- [ ] 保持移动端/桌面布局可用，不影响现有编辑器样式。

**Verification Criteria**:
- [ ] SSR 首屏可见，hydrate 后无明显闪烁。  
- [ ] 左树切换文档成功，URL/标题同步。  
- [ ] 不出现 hydration mismatch 警告（允许可控白名单项）。  

**Exit Criteria**:
- [ ] 阅读页可 SSR + 客户端增强完整运行。

---

## Milestone 6: 一致性测试与回归基线
**Status**: Pending  
**Type**: Testing  
**Estimated**: 2~3 天  
**Files**:
- `apps/web/scripts/ssr-markdown-consistency-check.mjs`（新增）
- `apps/server/internal/server/reader_page_handler_test.go`（补充）
- `docs/SPACE_READER_SSR_SUBPROCESS_TESTPLAN.md`（新增）
- `docs/SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`（补充验收样例）

**Tasks**:
- [ ] 建立 Markdown 样例集（表格、脚注、公式、代码块、图片、HTML）。  
- [ ] 实现 SSR vs CSR DOM 差异检查脚本。  
- [ ] 补充手工验收清单（SEO、权限、降级、样式一致性）。  
- [ ] 覆盖关键回归：
  - Worker 崩溃恢复
  - 长文档渲染
  - 大图文档渲染

**Verification Criteria**:
- [ ] 一致性脚本通过率达到目标阈值。  
- [ ] CI 能识别协议破坏与明显渲染回归。  

**Exit Criteria**:
- [ ] 有可重复执行的客观验收工具，不依赖肉眼临时判断。

---

## Milestone 7: 性能、缓存与发布加固
**Status**: Pending  
**Type**: Production Hardening  
**Estimated**: 2~3 天  
**Files**:
- `apps/server/internal/service/reader_page_cache.go`（新增）
- `apps/server/internal/server/handler/reader_page.go`
- `apps/server/internal/logit/*`（必要时扩展指标日志）
- `docs/SPACE_READER_SSR_SUBPROCESS_RELEASE_CHECKLIST.md`（新增）

**Tasks**:
- [ ] 增加 SSR 结果缓存（按 `docVersion + theme + viewerSegment` 维度）。  
- [ ] 增加关键指标日志：
  - render duration
  - queue depth
  - worker restart count
  - fallback count
- [ ] 制定发布与回滚流程：
  - 如何禁用 SSR
  - 如何只回滚 Worker bundle
  - 如何排查协议版本不匹配

**Verification Criteria**:
- [ ] 高并发下 P95 在目标区间内。  
- [ ] Worker 波动时系统仍可对外稳定服务。  

**Exit Criteria**:
- [ ] 具备线上可维护性与可回滚能力。

---

## 协议样例（开发联调用）

### 请求（Go -> Worker）

```json
{"id":"req_01","type":"render","version":"v1","route":"space-reader","deadlineMs":1500,"payload":{"spaceId":"01...","docId":"01...","title":"文档标题","markdown":"# Hello"}}
```

### 响应成功（Worker -> Go）

```json
{"id":"req_01","ok":true,"html":"<div>...</div>","head":{"title":"文档标题"},"metrics":{"renderMs":22}}
```

### 响应失败（Worker -> Go）

```json
{"id":"req_01","ok":false,"error":{"code":"RENDER_FAILED","message":"unexpected token"}}
```

---

## 任务分工建议

1. 后端 A：Milestone 1/2/4/7（Go 配置、进程池、handler、缓存、降级）。  
2. 前端 B：Milestone 3/5（Worker 入口、SSR bundle、阅读页 hydration）。  
3. 联调 C：Milestone 6（样例集、一致性测试、验收清单）。  

---

## 阻塞条件（开始开发前确认）

1. 部署环境可提供 Node 运行时（即使不作为独立服务）。  
2. 发布流程允许 Go 与 Web SSR Bundle 同版本发布。  
3. 已确认阅读页 URL 规范（`/r/:spaceId/:docId` 是否最终定版）。  
4. 已确认首期阅读页权限规则（匿名是否允许访问 member/authenticated 空间）。

---

## 最终验收（上线门槛）

1. 阅读页 SSR 可稳定输出且可被抓取。  
2. Worker 异常时主服务可降级，不出现大面积 500。  
3. 渲染一致性样例通过并有自动化回归。  
4. 发布与回滚清单完备，值班可独立执行故障处置。  

