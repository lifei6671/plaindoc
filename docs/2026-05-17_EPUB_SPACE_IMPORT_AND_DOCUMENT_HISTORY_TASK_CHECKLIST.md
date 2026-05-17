# EPUB 空间导入与文档历史版本执行清单

**文档状态**: To Do  
**创建日期**: 2026-05-17  
**关联方案**: `docs/2026-05-17_EPUB_SPACE_IMPORT_AND_DOCUMENT_HISTORY_TECHNICAL_PLAN.md`  
**适用范围**: `apps/server`、`apps/web`、`docs`  
**目标**: 将 EPUB 空间导入与文档历史版本技术方案拆成可执行、可回写进度、可分阶段 review 的任务清单。完成后支持后台导入 `.epub` 为新空间、EPUB 目录映射为 PlainDoc 目录树、章节转 Markdown、图片本地化、导入任务 SSE 进度，以及编辑器历史版本弹窗、Markdown diff、Office 版本元数据展示和恢复到指定历史版本。

---

## 0. 执行原则

1. 每完成一个 checkbox，必须在本文件中回写 `[x]`。
2. 每个 Phase 完成后停下来做 code review，再进入下一阶段。
3. 依赖变更必须先单独确认，不能直接修改依赖文件：
   - Go 依赖：`github.com/JohannesKaufmann/html-to-markdown/v2`
   - 前端依赖：`@codemirror/merge`
4. `go.sum` 禁止手工编辑，只能由 Go 工具链生成。
5. 后端接口返回继续遵守 `JsonResult`：`{ code, message, requestId, data }`。
6. 前端页面层禁止直接散写业务 `fetch`，统一走 `apps/web/src/data-access/*`。
7. EPUB 导入永远创建新空间，不能覆盖已有空间。
8. EPUB 导入权限按“可创建空间”判断，不能硬编码成平台管理员或空间管理员。
9. 历史版本恢复不能修改、删除或重排旧 revision，只能新增一个恢复后的新版本。
10. Markdown 文档首期支持正文 diff；Office 文档首期只展示文件版本元数据和恢复入口，不做二进制 diff。

---

## 1. 依赖关系与并行批次

### 1.1 必须串行的关口

```text
Phase 0 基线确认与方案锁定
  -> Phase 1 EPUB inspect 契约与前端入口
  -> Phase 2 EPUB 解析器与安全边界
  -> Phase 3 HTML 清洗与 Markdown 转换
  -> Phase 4 EPUB commit 落地与任务进度
  -> Phase 5 历史版本摘要与详情接口
  -> Phase 6 历史版本恢复接口
  -> Phase 7 历史版本弹窗与 diff
  -> Phase 8 测试、文档同步与回归
```

### 1.2 可并行的工作

在 Phase 1 契约稳定后，可分三路推进：

1. EPUB 后端解析与安全测试：Phase 2、Phase 3。
2. EPUB 导入弹窗展示：Phase 1 前端部分可在 mock 数据下推进。
3. 历史版本后端接口与前端弹窗：Phase 5 和 Phase 7 的 UI 骨架可并行，但恢复接口接入必须等 Phase 6 完成。

单轮并行任务最多 5 个，且不得同时修改同一文件区域。

---

## 2. 文件规划

### 2.1 后端新增文件

- [ ] `apps/server/internal/service/admin_space_epub_importer.go`
  - EPUB 包解析、目录预分配、资源收集、章节切分、链接重写和 XHTML 清洗。
- [ ] `apps/server/internal/service/admin_space_epub_importer_test.go`
  - EPUB2、EPUB3、spine fallback、非法包、安全限制、fragment、图片和链接测试。
- [x] `apps/server/internal/service/html_markdown_converter.go`
  - service 层 HTML 到 Markdown 转换组件。
- [x] `apps/server/internal/service/html_markdown_converter_test.go`
  - 图片、代码块、表格、链接、脚注、危险 HTML 降级和性能基准。

### 2.2 后端修改文件

- [ ] `apps/server/internal/server/handler/admin_space_import.go`
  - inspect 错误映射支持 EPUB，保持导入入口不分裂。
- [ ] `apps/server/internal/service/admin_space_import_service.go`
  - staging 记录 package type，commit 根据 `.plaindoc` / `.epub` 分发。
- [ ] `apps/server/internal/storage/repository/interfaces.go`
  - 如现有方法不足，补充分页 revision 摘要和单 revision 查询接口。
- [ ] `apps/server/internal/storage/repository/gorm_workspace_repository.go`
  - 查询 Markdown / Office 版本摘要、详情和创建人展示名。
- [ ] `apps/server/internal/server/handler/workspace.go`
  - 拆分 revision 列表、详情和恢复接口。
- [ ] `apps/server/internal/server/router.go`
  - 注册 revision 详情和恢复路由。
- [x] `apps/server/go.mod`
  - 经确认后加入 `github.com/JohannesKaufmann/html-to-markdown/v2`。
- [x] `apps/server/go.sum`
  - 由 Go 工具链自动更新。

### 2.3 后端测试文件

- [ ] `apps/server/internal/service/admin_space_import_service_test.go`
  - 追加 EPUB inspect / commit / staging / 权限 / SSE 行为测试。
- [ ] `apps/server/internal/server/workspace_handler_test.go`
  - 如仓库已有集中式 handler 测试，追加 revision 列表、详情和恢复接口测试。
- [ ] `apps/server/internal/storage/repository/*_test.go`
  - 按现有测试布局补 revision 摘要分页、详情查询和 Office 版本查询测试。

### 2.4 前端新增文件

- [ ] `apps/web/src/components/DocumentRevisionHistoryDialog.tsx`
  - 历史版本列表、详情加载、diff 渲染、恢复确认、Office 元数据展示和错误态。
- [ ] `apps/web/src/components/DocumentRevisionHistoryDialog.test.tsx`
  - 弹窗加载、空状态、错误态、分页、恢复禁用和确认文案测试。

### 2.5 前端修改文件

- [ ] `apps/web/src/data-access/types.ts`
  - 扩展 EPUB inspect 类型，拆分 revision summary/detail，新增 restore input/result。
- [ ] `apps/web/src/data-access/http/adapter.ts`
  - 兼容 `.epub` inspect/commit，新增 revision detail 和 restore API。
- [ ] `apps/web/src/admin/components/AdminSpaceImportDialog.tsx`
  - 上传 accept 增加 `.epub`，预览展示 EPUB 摘要和 warnings。
- [ ] `apps/web/src/App.tsx`
  - 增加历史版本 icon 按钮，接入弹窗和恢复成功后的编辑器状态刷新。
- [ ] `apps/web/package.json`
  - 经确认后加入 `@codemirror/merge`。
- [ ] `package-lock.json`
  - 由 npm 工具链自动更新。

### 2.6 文档文件

- [ ] `docs/2026-05-17_EPUB_SPACE_IMPORT_AND_DOCUMENT_HISTORY_TECHNICAL_PLAN.md`
  - 实施中如有方案偏离，必须先确认后更新。
- [ ] `docs/2026-05-17_EPUB_SPACE_IMPORT_AND_DOCUMENT_HISTORY_TASK_CHECKLIST.md`
  - 本执行清单，作为推进控制面板。
- [ ] `docs/BACKEND_DEVELOPER_GUIDE.md`
  - Phase 8 同步 EPUB 导入、revision API 和恢复语义。
- [ ] `docs/FRONTEND_DEVELOPER_GUIDE.md`
  - Phase 8 同步 EPUB 导入弹窗、历史版本弹窗和 diff 依赖。
- [ ] `docs/README.md`
  - 如新增专题入口或状态变化，跟随同步。

---

## 3. Phase 0：基线确认与方案锁定

**目标**: 明确现有空间导入、revision、Office 保存和前端编辑器入口的真实状态，避免后续实现偏离现有架构。

### Task 0.1 现有空间导入链路盘点

**文件**

- 阅读：`apps/server/internal/server/handler/admin_space_import.go`
- 阅读：`apps/server/internal/service/admin_space_import_service.go`
- 阅读：`apps/web/src/admin/components/AdminSpaceImportDialog.tsx`
- 阅读：`apps/web/src/admin/space-transfer/AdminSpaceTransferTaskProvider.tsx`

**步骤**

- [x] 确认 inspect staging 的保存位置、TTL、actor 绑定和清理策略。
- [x] 确认 commit 任务创建、SSE 事件、失败清理和右下角浮层恢复逻辑。
- [x] 确认 `.plaindoc` import result 类型是否能扩展 `packageType` 而不破坏现有 UI。
- [x] 记录 EPUB 需要复用的 service helper 和不能复用的 `.plaindoc` 专用逻辑。

**验收**

- [x] 明确 EPUB inspect / commit 可以挂入现有入口。
- [x] 明确 staging TTL 是否需要在本任务补齐。
- [x] 本清单如发现遗漏，已回写新增任务。

### Task 0.2 现有 revision 与 Office 版本链路盘点

**文件**

- 阅读：`apps/server/internal/server/handler/workspace.go`
- 阅读：`apps/server/internal/storage/repository/interfaces.go`
- 阅读：`apps/server/internal/storage/repository/gorm_workspace_repository.go`
- 阅读：`apps/web/src/data-access/types.ts`
- 阅读：`apps/web/src/data-access/http/adapter.ts`
- 阅读：`apps/web/src/App.tsx`

**步骤**

- [x] 确认 `GET /api/docs/:docId/revisions` 当前是否返回正文。
- [x] 确认 Markdown revision 和 Office file revision 的版本字段、baseVersion 和 source 语义。
- [x] 确认 `SaveDocument` 的版本冲突、缓存清理和图片引用同步 helper。
- [x] 确认 Office 保存回调的 source blob、file revision 和阅读 HTML 失效逻辑。
- [x] 确认编辑器当前保存状态、未保存内容判断和 `activeDocument.version` 更新路径。

**验收**

- [x] 明确列表接口拆分策略。
- [x] 明确 Markdown 与 Office 恢复应复用的后端路径。
- [x] 明确前端恢复成功后需要刷新的状态字段。

### Task 0.3 依赖变更确认

**文件**

- 待读：`apps/server/go.mod`
- 待读：`apps/web/package.json`

**步骤**

- [x] 发起 Go 依赖确认：`github.com/JohannesKaufmann/html-to-markdown/v2`。
- [x] 发起前端依赖确认：`@codemirror/merge`。
- [x] 如依赖未被批准，回写替代方案任务（本次已批准，不适用）：
  - Go 侧临时保留 Node 脚本回退或实现有限 Markdown 转换。
  - 前端侧临时展示只读双栏文本或轻量 diff。

**验收**

- [x] 依赖变更已获得明确确认，或清单已改为无新增依赖方案。
- [x] 未在确认前修改 `go.mod`、`go.sum`、`package.json` 或 lockfile。

---

## 4. Phase 1：EPUB inspect 契约与前端入口

**目标**: 先让后端能识别 EPUB、返回预览摘要，前端能选择 `.epub` 并展示可确认的导入信息。

### Task 1.1 前后端导入契约扩展

**文件**

- 修改：`apps/web/src/data-access/types.ts`
- 修改：`apps/web/src/data-access/http/adapter.ts`
- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [x] 新增 `AdminSpaceImportPackageType`，兼容现有 `plaindoc-space` 与新增 `epub`。
- [x] inspect 结果增加 `packageType`。
- [x] summary 增加 EPUB 可选字段：`imageCount`、`maxDepth`。
- [x] 增加 `sourcePublishedAt`，并保留 `.plaindoc` 的 `exportedAt` 语义。
- [x] 保持 `.plaindoc` 现有响应兼容，不要求旧包提供 EPUB 字段。

**验收**

- [x] TypeScript 类型能表达 `.plaindoc` 和 `.epub` 两种导入包。
- [x] 前端仍以 `code == 0` 判断业务成功。
- [x] 旧 `.plaindoc` 导入预览不出现 EPUB 专属空字段噪音。

### Task 1.2 EPUB inspect 后端最小闭环

**文件**

- 修改：`apps/server/internal/server/handler/admin_space_import.go`
- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 新增：`apps/server/internal/service/admin_space_epub_importer.go`
- 新增：`apps/server/internal/service/admin_space_epub_importer_test.go`

**步骤**

- [x] inspect 根据后缀和容器识别 `.epub`。
- [x] 校验 zip 内 `mimetype = application/epub+zip`。
- [x] 校验存在 `META-INF/container.xml`。
- [x] 解析 OPF rootfile 路径。
- [x] 读取 OPF metadata：title、authors、`dc:date`。
- [x] 统计 spine 章节数、图片资源数和目录最大深度。
- [x] 生成临时展示 space ID，commit 时仍由服务端生成真实新空间 ID。
- [x] 错误映射使用导入错误体系，不泄漏底层 zip/XML 细节。

**验收**

- [x] 标准 EPUB3 inspect 返回 `packageType = "epub"`。
- [x] 缺少 mimetype / container / OPF / spine 时返回不可导入或明确错误。
- [x] inspect 不创建真实空间和文档。

### Task 1.3 EPUB 导入弹窗入口

**文件**

- 修改：`apps/web/src/admin/components/AdminSpaceImportDialog.tsx`

**步骤**

- [x] 上传 `accept` 改为 `.plaindoc,.epub`。
- [x] 按 `packageType` 区分预览文案。
- [x] `.epub` 显示书名、作者、出版日期、章节数、图片数、最大目录深度和 warnings。
- [x] `.plaindoc` 继续显示导出时间，不把 EPUB 出版日期误显示成导出时间。
- [x] EPUB 默认空间名取 inspect 返回的书名，并允许用户修改。

**验收**

- [x] 用户能选择 `.epub` 并看到导入预览。
- [x] 用户能修改 EPUB 导入后的空间名、分类和可见性。
- [x] `.plaindoc` 原有导入体验不回退。

---

## 5. Phase 2：EPUB 解析器与安全边界

**目标**: 做稳 EPUB 容器解析、路径清洗、nav/toc/spine 目录模型和安全限制，为 commit 阶段提供可信中间结构。

### Task 2.1 zip entry 与资源限制

**文件**

- 修改：`apps/server/internal/service/admin_space_epub_importer.go`
- 修改：`apps/server/internal/service/admin_space_epub_importer_test.go`

**步骤**

- [x] 使用 `archive/zip` 打开 EPUB。
- [x] 拒绝绝对路径、`..`、空路径和重复 entry。
- [x] 限制 entry 数不超过 2000。
- [x] 限制总解压内容不超过 128MiB。
- [x] 限制单 entry 不超过 32MiB。
- [x] 限制目录深度不超过 16。
- [x] 所有错误使用 `fmt.Errorf("操作描述: %w", err)` 包装。

**验收**

- [x] zip slip、重复 entry、超大 entry、超多 entry 都有单测。
- [x] 不读取超过限制的 entry 内容。
- [x] 不发起任何外部网络请求。

### Task 2.2 OPF / nav.xhtml / toc.ncx 解析

**文件**

- 修改：`apps/server/internal/service/admin_space_epub_importer.go`
- 修改：`apps/server/internal/service/admin_space_epub_importer_test.go`

**步骤**

- [x] 解析 `META-INF/container.xml` 的 rootfile。
- [x] 解析 OPF metadata、manifest 和 spine。
- [x] 优先解析 EPUB3 `nav.xhtml`。
- [x] 其次解析 EPUB2 `toc.ncx`。
- [x] 无 nav/toc 时按 spine 顺序生成扁平目录。
- [x] 对 media-type 不标准但扩展名明确的资源做兜底识别并记录 warning。
- [x] 对非 UTF-8 声明做有限兼容或记录 warning。

**验收**

- [x] EPUB2、EPUB3 和无 nav/toc 样本都有测试。
- [x] 解析结果包含 title、authors、chapters、resources 和 warnings。
- [x] 解析器不依赖新增 EPUB 容器第三方包。

### Task 2.3 目录映射与 fragment 预分配模型

**文件**

- 修改：`apps/server/internal/service/admin_space_epub_importer.go`
- 修改：`apps/server/internal/service/admin_space_epub_importer_test.go`

**步骤**

- [x] 将 `href` 规范化为 `canonicalHref + fragment`。
- [x] 生成稳定 `targetKey`。
- [x] 无 `href` 且有子节点的目录项映射为 folder。
- [x] 叶子目录项默认映射为 doc。
- [x] 有正文且有子项的目录项映射为 folder + `正文` doc。
- [x] 同级标题冲突沿用唯一标题策略。
- [x] fragment 可定位性在预分配阶段完成。
- [x] 多个目录项指向同一 `targetKey` 时创建“参见”占位文档。

**验收**

- [x] 嵌套目录导入模型能区分 folder/doc。
- [x] fragment 可拆分、不可定位降级和重复 target 都有测试。
- [x] 预分配结果能产出 `targetKey -> documentID/readerURL` 映射所需数据。

---

## 6. Phase 3：HTML 清洗与 Markdown 转换

**目标**: 把 EPUB XHTML/HTML 安全转换为 PlainDoc 可编辑 Markdown，并为图片和链接重写留出稳定接口。

### Task 3.1 HTML 清洗与危险协议过滤

**文件**

- 修改：`apps/server/internal/service/admin_space_epub_importer.go`
- 修改：`apps/server/internal/service/admin_space_epub_importer_test.go`

**步骤**

- [x] 使用 `golang.org/x/net/html` 解析 XHTML/HTML。
- [x] 只保留 body 主体。
- [x] 删除 `script`、`style`、`form`、`input`、`button`、`noscript`。
- [x] 移除事件属性。
- [x] 外部 `http/https` 链接保留。
- [x] 内部 EPUB 链接交给 commit 阶段映射表重写。
- [x] `javascript:`、`file:`、本机绝对路径和危险 data URI 降级为纯文本。

**验收**

- [x] 危险标签、事件属性和危险协议都有测试。
- [x] 清洗阶段不会重新放行原始 HTML。
- [x] warning 能定位到 source key 或章节标题。

### Task 3.2 HTML Markdown Converter 封装

**文件**

- 新增：`apps/server/internal/service/html_markdown_converter.go`
- 新增：`apps/server/internal/service/html_markdown_converter_test.go`
- 修改：`apps/server/go.mod`
- 修改：`apps/server/go.sum`

**前置**

- [x] 已获得新增 Go 依赖确认。

**步骤**

- [x] 定义 `HTMLMarkdownConverter` 接口，接口放在消费方或 service 使用边界。
- [x] 定义 `ConvertHTMLMarkdownInput` 和 `ConvertHTMLMarkdownResult`。
- [x] 接入 `github.com/JohannesKaufmann/html-to-markdown/v2`。
- [x] 自定义 `<img>` 输出 `![alt](localURL)`，禁止输出 `data:image/*`。
- [x] 自定义 `<pre><code>`，尽量保留 fenced code block 和 language class。
- [x] 自定义 `<table>`，优先 GFM table，复杂表格降级并记录 warning。
- [x] 自定义 `<a>`，只输出已重写的安全 URL。
- [x] 处理 `<sup>/<sub>`、脚注和 EPUB 常见 `span`。

**验收**

- [x] 转换器单测覆盖标题、段落、列表、表格、代码块、链接和图片。
- [x] 转换器不承担安全清洗职责，调用前后边界清晰。
- [x] `go mod tidy` 后依赖文件由工具链生成。

### Task 3.3 HTML 转 Markdown 性能基准

**文件**

- 修改：`apps/server/internal/service/html_markdown_converter_test.go`

**步骤**

- [x] 增加 20 个章节、50 个章节、100 个章节基准。
- [x] 增加每章 10KiB、100KiB、1MiB HTML 基准。
- [x] 记录串行转换总耗时。
- [x] 记录单章 P95 耗时。
- [x] 记录失败重试或降级行为。

**验收**

- [x] 50 个 100KiB 章节串行转换不超过 10 秒，或已回写优化/回退任务。
- [x] 单章 P95 不超过 300ms，或已回写优化/回退任务。
- [x] 内存峰值在导入任务可接受范围内，或已回写分段转换任务。

---

## 7. Phase 4：EPUB commit 落地与任务进度

**目标**: 完成 EPUB 导入为新空间的真实写入流程，并接入现有导入任务、SSE 和失败清理。

### Task 4.1 EPUB commit 分发与权限

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/server/internal/server/handler/admin_space_import.go`
- 修改：`apps/server/internal/service/admin_space_import_service_test.go`

**步骤**

- [x] staging 记录增加 `PackageType`。
- [x] commit 根据 package type 分发 `.plaindoc` 或 `.epub`。
- [x] EPUB commit 忽略客户端传入的 `spaceId`，由服务端生成新空间 ID。
- [x] EPUB commit 默认空间名取 EPUB title，允许使用请求中的 `spaceName` 覆盖。
- [x] EPUB 导入权限调用“创建空间能力”。
- [x] `.plaindoc` 继续沿用现有导入权限。

**验收**

- [x] 无创建空间权限的用户不能导入 EPUB。
- [x] 有创建空间权限的普通用户可以导入 EPUB。
- [x] EPUB 不可能覆盖已有空间。

### Task 4.2 两段式预分配与内部链接回写

**文件**

- 修改：`apps/server/internal/service/admin_space_epub_importer.go`
- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/server/internal/service/admin_space_epub_importer_test.go`

**步骤**

- [x] 第一段预分配待创建的 `nodeID`、`documentID`、`readerSlug`。
- [x] 建立 `targetKey -> documentID/readerURL` 映射。
- [x] 建立 `canonicalHref -> primary documentID/readerURL` 映射。
- [x] 第二段转换章节 HTML 时重写内部 `<a href>`。
- [x] target 命中时改写为目标 PlainDoc 阅读链接。
- [x] 仅 canonicalHref 命中时改写到主文档并记录 fragment 降级 warning。
- [x] 完全未命中时保留链接文本、移除 href 并记录 warning。

**验收**

- [x] 同文件 fragment、跨文件链接、缺失目标和重复 target 都有测试。
- [x] “参见”占位文档使用固定 Markdown 模板。
- [x] 转换后的 Markdown 不包含 EPUB 内部相对链接。

### Task 4.3 图片本地化与 SVG 安全降级

**文件**

- 修改：`apps/server/internal/service/admin_space_epub_importer.go`
- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/server/internal/service/admin_space_epub_importer_test.go`

**步骤**

- [x] EPUB 内部图片只从 zip entry 读取。
- [x] 支持 `image/png`、`image/jpeg`、`image/gif`、`image/webp`、`image/svg+xml`。
- [x] `data:image/*` 解码前后都执行大小限制。
- [x] 单张图片沿用 20MiB 上限。
- [x] 图片写入复用现有 blob/image hosting 能力。
- [x] Markdown 图片 URL 改写为本地资源 URL。
- [x] `image/svg+xml` 禁止内联。
- [x] SVG 中存在脚本、`foreignObject`、事件属性或外部引用时降级为 alt 文本并记录 warning。
- [x] 单个图片失败不导致整本 EPUB 导入失败。

**验收**

- [x] 相对路径图片和 `data:image/*` 都能本地化。
- [x] 超大图片降级为 alt 文本。
- [x] 危险 SVG 不会进入 Markdown 或可执行渲染路径。

### Task 4.4 空间、目录树、文档和 revision 写入

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/server/internal/service/admin_space_import_service_test.go`

**步骤**

- [x] 创建新空间。
- [x] EPUB `dc:description` 写入空间简介字段，缺失时再使用导入来源兜底描述。
- [x] EPUB3 `cover-image` / EPUB2 `meta name="cover"` 封面写入空间封面资产。
- [x] 按 EPUB 目录创建 folder/doc 节点。
- [x] 将转换后的 Markdown 写入 `documents.content_md`。
- [x] 写入首版 `document_revisions.content_md`。
- [x] 写入作者、导入 warning 或导入来源相关审计信息。
- [x] commit 失败时清理已创建空间、节点、文档和临时资源。
- [x] commit 成功后删除 staging 文件。
- [x] commit 失败后按短期排查窗口保留 staging，并由 TTL 清理。

**验收**

- [x] 标准 EPUB 导入后能在空间中看到目录树和 Markdown 文档。
- [x] 带 OPF 简介和封面的 EPUB 导入后，空间简介与空间封面同步生成。
- [x] 首版 revision 与当前文档内容一致。
- [x] 失败清理不会留下半成品空间。

### Task 4.5 SSE 进度与前端任务中心

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/web/src/admin/space-transfer/AdminSpaceTransferTaskProvider.tsx`
- 修改：`apps/web/src/admin/components/AdminSpaceImportDialog.tsx`

**步骤**

- [x] EPUB commit 接入现有导入任务持久化。
- [x] 进度阶段覆盖解析、创建空间、文档转换/写入、完成和失败。
- [x] SSE completed payload 包含 `newSpaceId`。
- [x] 右下角全局任务浮层能展示 EPUB 导入任务。
- [x] 页面刷新后能恢复进行中的 EPUB 导入任务。
- [x] SSE 连接异常仅标记事件流中断，不再把仍在后台运行的大文件导入任务误标为失败。
- [x] SSE 连接异常后先允许浏览器 `EventSource` 自动重连；若 30 秒内仍未恢复，再主动刷新后端任务快照。任务仍在运行则重新申请 stream token 并重连，已完成则直接触发导入完成流程。
- [x] EPUB 文档写入阶段按“已导入文档数 / 总文档数”计算并发布 `epub_documents` 进度。

**验收**

- [x] EPUB 导入进度不需要新建独立任务中心。
- [x] SSE 断线重连行为与 `.plaindoc` 导入一致。
- [x] 失败状态能展示可读错误和 warnings。
- [x] 大 EPUB 导入完成后阅读页 SSR 响应 payload 可超过 1MiB 请求上限，仍保留独立响应上限兜底。
- [x] 大 EPUB 不会因为图片阶段固定进度导致前端长时间停在同一百分比；每成功写入一个文档都会推进任务进度。

---

## 8. Phase 5：历史版本摘要与详情接口

**目标**: 将现有 revision 列表改为轻量摘要分页，并新增单版本详情接口。

### Task 5.1 前后端 revision 类型拆分

**文件**

- 修改：`apps/web/src/data-access/types.ts`
- 修改：`apps/web/src/data-access/http/adapter.ts`
- 修改：`apps/server/internal/server/handler/workspace.go`

**步骤**

- [x] 新增 `DocumentRevisionSummary`。
- [x] 新增 `DocumentRevisionDetail extends DocumentRevisionSummary`。
- [x] 新增 `format = "markdown" | "docx" | "xlsx"`。
- [x] 新增 `editorUser` 展示模型。
- [x] 列表接口只返回摘要，不返回 `contentMd`。
- [x] 详情接口前端类型和 gateway 支持 Markdown 正文或 Office 文件版本元数据；后端详情查询与路由在 Task 5.2/5.3 落地。

**验收**

- [x] 版本列表响应不包含正文。
- [x] 前端 gateway 能分别请求列表和详情。
- [x] 类型能同时表达 Markdown 和 Office revision。

### Task 5.2 repository 分页摘要和详情查询

**文件**

- 修改：`apps/server/internal/storage/repository/interfaces.go`
- 修改：`apps/server/internal/storage/repository/gorm_workspace_repository.go`
- 修改：`apps/server/internal/storage/repository/*_test.go`

**步骤**

- [x] 增加按 documentID 分页查询 Markdown revision 摘要。
- [x] 增加按 documentID 分页查询 Office file revision 摘要。
- [x] 查询关联用户表，返回创建人 ID 和展示名。
- [x] 增加单个 Markdown revision 详情查询。
- [x] 增加单个 Office file revision 详情查询。
- [x] 查询按版本倒序稳定排序。
- [x] 不存在 revision 返回明确 not found。

**验收**

- [x] 分页不重复、不乱序。
- [x] 创建人缺失时前端可展示“未知用户”或“系统导入”。
- [x] repository 测试覆盖 Markdown 和 Office。

### Task 5.3 revision handler 与权限

**文件**

- 修改：`apps/server/internal/server/handler/workspace.go`
- 修改：`apps/server/internal/server/router.go`
- 修改：`apps/server/internal/server/*workspace*_test.go`

**步骤**

- [x] 保留 `GET /api/docs/:docId/revisions?page=1&pageSize=30`。
- [x] 注册 `GET /api/docs/:docId/revisions/:revisionId`。
- [x] 列表和详情都调用 `visibilityService.GetDocument(ctx, documentID, actorUserID)`。
- [x] 只要能读取当前文档，就能读取该文档历史版本。
- [x] pageSize 做上限保护。
- [x] 错误响应映射符合现有 workspace handler 风格。

**验收**

- [x] 无文档读权限时列表和详情都拒绝。
- [x] 有读权限时能获取摘要和详情。
- [x] 详情接口不能读取其它文档的 revision。

---

## 9. Phase 6：历史版本恢复接口

**目标**: 支持 Markdown、docx、xlsx 恢复到指定历史版本，恢复时新增当前版本和新 revision。

### Task 6.1 恢复接口契约与路由

**文件**

- 修改：`apps/web/src/data-access/types.ts`
- 修改：`apps/web/src/data-access/http/adapter.ts`
- 修改：`apps/server/internal/server/handler/workspace.go`
- 修改：`apps/server/internal/server/router.go`

**步骤**

- [x] 新增 `RestoreDocumentRevisionInput`。
- [x] 新增 `RestoreDocumentRevisionResult`。
- [x] 注册 `POST /api/docs/:docId/revisions/:revisionId/restore`。
- [x] 前端始终传当前 `Document.version` 作为 `baseVersion`。
- [x] 响应沿用保存文档结果语义，返回最新 document 和 restoredFromRevision。

**验收**

- [x] 恢复接口契约前后端一致。
- [x] baseVersion 缺失或非法时返回参数错误。
- [x] 不存在 revision 返回 404。

### Task 6.2 Markdown 恢复实现

**文件**

- 修改：`apps/server/internal/server/handler/workspace.go`
- 修改：`apps/server/internal/storage/repository/gorm_workspace_repository.go`
- 修改：`apps/server/internal/server/*workspace*_test.go`

**步骤**

- [x] 读取目标 Markdown revision。
- [x] 校验当前文档格式与目标 revision 格式一致。
- [x] 校验 `documents.version == baseVersion`。
- [x] 以目标 revision 的 `contentMd` 覆盖当前正文。
- [x] 当前文档版本递增。
- [x] 新增一条 `document_revisions`。
- [x] 新 revision 的 `baseVersion` 为恢复前当前版本。
- [x] `editorUserID` 使用执行恢复的用户。
- [x] `source` 首期仍使用 `remote`。
- [x] 复用保存文档的阅读缓存清理和图片引用同步规则。

**验收**

- [x] 恢复后当前文档内容等于目标 revision 内容。
- [x] 恢复后版本号递增，并新增 revision。
- [x] 旧 revision 不被修改、删除或重排。
- [x] baseVersion 过期时返回版本冲突，并带最新文档。

### Task 6.3 Office 恢复实现

**文件**

- 修改：`apps/server/internal/server/handler/workspace.go`
- 修改：`apps/server/internal/storage/repository/gorm_workspace_repository.go`
- 修改：`apps/server/internal/server/*workspace*_test.go`

**步骤**

- [x] 读取目标 file revision。
- [x] 校验当前文档格式为 `docx` 或 `xlsx`。
- [x] 校验当前文档格式与目标 file revision 格式一致。
- [x] 校验 `documents.version == baseVersion`。
- [x] 确保当前 `contentVersion` 未被其它 Office 写回推进。
- [x] 以目标 file revision 的 `blobID/fileName/mimeType` 覆盖当前 source 文件引用。
- [x] 不重复写入文件 blob。
- [x] 当前文档版本递增。
- [x] 新增一条 `document_file_revisions`。
- [x] 标记或触发阅读 HTML 重新渲染。

**验收**

- [x] Office 恢复后 source blob/fileName/mimeType 等于目标 file revision。
- [x] 恢复后版本号递增，并新增 file revision。
- [x] Office 二进制内容不做 diff。
- [x] 并发写回导致版本冲突时不会覆盖新内容。

### Task 6.4 恢复权限与审计边界

**文件**

- 修改：`apps/server/internal/server/handler/workspace.go`
- 修改：`apps/server/internal/service/*`
- 修改：`apps/server/internal/server/*workspace*_test.go`

**步骤**

- [x] 列表和详情继续使用读权限。
- [x] 恢复操作使用空间写权限，权限边界与 `SaveDocument` / Office 保存回调一致。
- [x] 恢复操作写入必要审计或结构化日志。
- [x] 恢复错误包含 requestID，便于排查。

**验收**

- [x] 无写权限用户不能恢复。
- [x] 有写权限用户能恢复 Markdown 和 Office。
- [x] 权限测试覆盖 reader、collaborator、owner 或对应现有角色。

---

## 10. Phase 7：历史版本弹窗与 diff

**目标**: 在编辑器右侧操作区新增历史版本入口，提供可用的版本列表、详情 diff 和恢复交互。

### Task 7.1 历史版本入口

**文件**

- 修改：`apps/web/src/App.tsx`

**步骤**

- [x] 在 `header-actions` 增加 `History` 图标按钮。
- [x] 按现有按钮风格补 tooltip 和 aria label。
- [x] 仅在存在 activeDocument 时启用。
- [x] 打开 `DocumentRevisionHistoryDialog`。

**验收**

- [x] 入口位置符合：附件、目录、预览模式、主题、历史版本。
- [x] 无文档选中时不会触发无效请求。
- [x] 按钮样式与现有编辑器操作区一致。

### Task 7.2 历史版本弹窗基础状态

**文件**

- 新增：`apps/web/src/components/DocumentRevisionHistoryDialog.tsx`
- 新增：`apps/web/src/components/DocumentRevisionHistoryDialog.test.tsx`

**步骤**

- [x] 弹窗左侧展示版本列表。
- [x] 列表项展示版本号、创建时间和创建人。
- [x] 初始加载 30 条。
- [x] 支持滚动到底部或点击“加载更多”请求下一页。
- [x] 加载中展示 loading。
- [x] 空状态展示“暂无历史版本”。
- [x] 错误态展示错误信息和重试按钮。
- [x] 右侧详情区域独立滚动，长文档不撑破弹窗。

**验收**

- [x] 版本列表分页不重复、不乱序。
- [x] 创建人缺失时展示 fallback 文案。
- [x] 弹窗在窄屏和桌面宽度下无明显文本重叠。

### Task 7.3 Markdown diff 视图

**文件**

- 修改：`apps/web/src/components/DocumentRevisionHistoryDialog.tsx`
- 修改：`apps/web/package.json`
- 修改：`package-lock.json`

**前置**

- [x] 已获得新增前端依赖确认。

**步骤**

- [x] 接入 `@codemirror/merge`。
- [x] 左右对比当前编辑器 `content` 与选中历史版本 `contentMd`。
- [x] diff 视图只读。
- [x] 保持行号、选区和长文本滚动可用。
- [x] 切换版本时取消或忽略过期详情请求。
- [x] 当前文档有未保存内容时仍可查看 diff，但恢复按钮禁用。

**验收**

- [x] Markdown 历史版本能展示差异。
- [x] 未保存内容参与 diff，对比对象为编辑器当前 `content`。
- [x] 大文档切换版本不会造成明显 UI 卡死。

### Task 7.4 Office 历史版本展示

**文件**

- 修改：`apps/web/src/components/DocumentRevisionHistoryDialog.tsx`
- 修改：`apps/web/src/components/DocumentRevisionHistoryDialog.test.tsx`

**步骤**

- [x] Office 文档右侧展示文件名。
- [x] 展示 MIME、版本号、创建时间和创建人。
- [x] 展示“不支持二进制差异”的说明。
- [x] 保留恢复按钮。
- [x] 二次确认文案提示会切换当前 Office 源文件版本。

**验收**

- [x] Office 历史弹窗不尝试渲染文本 diff。
- [x] Office 版本元数据完整可读。
- [x] 恢复入口与 Markdown 行为一致。

### Task 7.5 恢复确认与成功状态刷新

**文件**

- 修改：`apps/web/src/components/DocumentRevisionHistoryDialog.tsx`
- 修改：`apps/web/src/App.tsx`
- 修改：`apps/web/src/components/DocumentRevisionHistoryDialog.test.tsx`

**步骤**

- [x] 点击“恢复到此版本”弹出二次确认。
- [x] 确认文案包含目标版本号、创建时间和创建人。
- [x] 当前文档有未保存内容时禁用恢复按钮，并提示先保存或放弃当前编辑。
- [x] 调用 restore API 时传当前 `activeDocument.version`。
- [x] 恢复成功后刷新版本列表。
- [x] 恢复成功后同步 `content`、`activeDocument.version`、`lastSavedAt` 和保存状态。
- [x] 恢复失败时保留弹窗并展示错误。

**验收**

- [x] 恢复成功后编辑器展示恢复后的内容或 Office 元数据。
- [x] 保存状态不会误显示为未保存。
- [x] 版本冲突错误能提示用户刷新或重新选择。

---

## 11. Phase 8：测试、文档同步与回归

**目标**: 完成后端、前端、样本 EPUB 和文档同步验证，确保任务可以进入最终 review。

### Task 8.1 后端测试矩阵

**文件**

- 修改：`apps/server/internal/service/*_test.go`
- 修改：`apps/server/internal/server/*_test.go`
- 修改：`apps/server/internal/storage/repository/*_test.go`

**步骤**

- [x] EPUB3：`nav.xhtml + content.opf + spine`。
- [x] EPUB2：`toc.ncx + content.opf + spine`。
- [x] 无 nav/toc：按 spine 生成扁平目录。
- [x] 嵌套目录生成 folder/doc 层级。
- [x] 目录项自身有正文且有子项生成 folder + `正文` doc。
- [x] XHTML 转 Markdown 覆盖标题、段落、列表、表格、代码块、链接。
- [x] 内部链接重映射覆盖同文件、跨文件和缺失目标。
- [x] fragment 拆分和无法定位降级。
- [x] 图片本地化覆盖相对路径图片和 `data:image/*`。
- [x] SVG 安全降级。
- [x] 非法 EPUB：缺少 mimetype、container、OPF、spine。
- [x] 安全场景：zip slip、重复 entry、超大 entry、超多 entry、危险协议。
- [x] 权限场景：有创建空间权限可导入，无创建空间权限拒绝。
- [x] 暂存清理：inspect 后放弃 commit，TTL 到期后清理 staging 文件。
- [x] 历史版本列表、详情、恢复、权限、冲突和不存在 revision。

**验收**

- [x] 后端新增逻辑都有针对性单测。
- [x] 关键安全边界都有失败用例。
- [x] 恢复操作证明旧 revision 不被修改。

### Task 8.2 前端测试矩阵

**文件**

- 修改：`apps/web/src/admin/pages/AdminSpacesPage.test.tsx`
- 新增：`apps/web/src/components/DocumentRevisionHistoryDialog.test.tsx`
- 视情况修改：`apps/web/src/App.test.tsx`

**步骤**

- [x] EPUB 导入弹窗支持 `.epub`。
- [x] EPUB 预览展示章节数、目录层级、图片数和 warnings。
- [x] `.plaindoc` 导入展示不回退。
- [x] 历史版本弹窗加载、空状态和错误态。
- [x] 版本切换加载详情。
- [x] 版本列表分页加载下一页。
- [x] Markdown diff 渲染。
- [x] 当前未保存内容禁用恢复按钮。
- [x] 恢复确认文案包含版本号、创建时间和创建人。
- [x] Office 历史弹窗展示文件元数据，不展示文本 diff。
- [x] 恢复成功后刷新编辑器状态。

**验收**

- [x] 前端交互关键路径有测试覆盖。
- [x] 历史版本弹窗不会绕过 data-access 层直接 fetch。
- [x] DropdownMenu 相关改动不绕过现有封装。

### Task 8.3 手工 EPUB 样本验证

**文件**

- 新增：`docs/epub-import-sample-validation.md`

**当前状态**

- Blocked：当前仓库、`/tmp` 和本地 `src` 工作区未找到可用 `.epub` 样本；为避免引入版权不明确文件，需等待用户提供样本或明确确认临时下载公版/测试 EPUB 后继续。

**步骤**

- [ ] 收集至少 1 本 EPUB2 样本。
- [ ] 收集至少 1 本 EPUB3 样本。
- [ ] 收集至少 1 本 Calibre 转换样本。
- [ ] 收集至少 1 本非标准 `media-type` 但扩展名明确的样本。
- [ ] 记录 inspect 摘要、warnings、导入耗时和导入后目录树。
- [ ] 记录 Markdown 转换质量和图片本地化结果。
- [ ] 如 20 本主流 EPUB 样本中解析失败超过 3 本，回写第三方 EPUB 解析库评估任务。

**验收**

- [ ] 样本验证结果可追溯。
- [ ] 解析失败或转换质量问题已记录为后续任务。
- [ ] 未把测试样本提交到仓库，除非明确确认。

### Task 8.4 标准验证命令

**命令**

```bash
go build ./...
go test -timeout 60s ./...
go test -race -timeout 60s ./...
golangci-lint run ./...
gofmt -w .
npm run web:build
npm run check:dropdown-menu -w @plaindoc/web
```

如果本地 `apps/web` 构建遇到 Node/V8 optimizer 崩溃，使用项目已知 workaround：

```bash
node --no-opt $(command -v npm) run build
```

**步骤**

- [x] 执行 `gofmt -w .`。
- [x] 执行 `go build ./...`。
- [x] 执行 `go test -timeout 60s ./...`。
- [ ] 执行 `go test -race -timeout 60s ./...`。
  - Blocked：`internal/service` 包在 60s 超时；诊断性 `go test -race -timeout 180s ./internal/service` 通过，未发现 race 报告，需要后续优化 race 模式耗时或调整 CI timeout。
- [ ] 执行 `golangci-lint run ./...`。
  - Blocked：本次 EPUB/Markdown 引入的 `errcheck`、未使用 NCX 类型和无效 XML tag 已修复；复跑后仍有既存 unused/gosimple/ineffassign/staticcheck 问题，需要单独 lint 清理任务处理。
- [x] 执行 `npm run web:build`。
- [x] 执行 `npm run check:dropdown-menu -w @plaindoc/web`。
- [x] 如使用 workaround，记录原因和实际命令。

**后续修复任务**

- [x] EPUB 导出文件名改为以空间名命名，并清理路径非法字符；空间名为空时回退到 `spaceID`。
- [x] EPUB 导出渲染阶段按“已渲染章节数 / 总章节数”发布 `epub_documents` 进度，避免大文件导出长期停在 55%。
- [x] EPUB 导出使用空间封面生成标准 EPUB cover，写入 `cover.xhtml`、`meta name="cover"` 和 `cover-image` manifest。
- [x] 导入/导出 SSE heartbeat 调整为 5s，低于常见 10s 代理 idle timeout，避免大文件任务无业务事件时 EventSource 反复重连。
- [x] 导入/导出 SSE 跳过全局 `REQUEST_TIMEOUT=10s` 业务超时，并清除 `HTTP_WRITE_TIMEOUT` 写截止时间，避免长连接被固定超时切断。
- [ ] 优化 `internal/service` 在 race 模式下的测试耗时，目标是 `go test -race -timeout 60s ./...` 不再因 60s 门限失败。
- [ ] 单独清理 `golangci-lint run ./...` 既存问题：unused、gosimple、ineffassign、staticcheck deprecated API 和 empty branch。

**验收**

- [x] 所有必跑命令通过，或失败原因已回写并拆成后续修复任务。
- [x] 没有未解释的格式化、lint 或测试失败。

### Task 8.5 文档同步

**文件**

- 修改：`docs/2026-05-17_EPUB_SPACE_IMPORT_AND_DOCUMENT_HISTORY_TECHNICAL_PLAN.md`
- 修改：`docs/2026-05-17_EPUB_SPACE_IMPORT_AND_DOCUMENT_HISTORY_TASK_CHECKLIST.md`
- 修改：`docs/BACKEND_DEVELOPER_GUIDE.md`
- 修改：`docs/FRONTEND_DEVELOPER_GUIDE.md`
- 修改：`docs/README.md`

**步骤**

- [x] 技术方案状态从 Draft 更新为实际状态。
- [x] 后端指南补 EPUB 导入、staging、权限、安全限制和 revision restore。
- [x] 前端指南补 EPUB 导入弹窗、历史版本弹窗和 diff 依赖。
- [x] README 专题导航补充或更新本文档入口。
- [x] 本清单回写所有完成项和验证记录。

**验收**

- [x] 文档与最终实现一致。
- [x] 技术方案、执行清单和开发者指南不存在明显冲突。
- [x] 后续可直接从本清单继续未完成任务。

---

## 12. 阶段 Review 记录

### Phase 0 Review

- 状态：部分完成
- 结论：空间导入、任务中心、staging、SSE、revision、Office 保存和前端保存状态链路已完成基线盘点；EPUB inspect 可以挂入现有导入入口，历史版本列表需要拆成摘要和详情。
- 遗留问题：依赖变更确认尚未执行，进入 Phase 3 前需确认 `github.com/JohannesKaufmann/html-to-markdown/v2`，进入 Phase 7 前需确认 `@codemirror/merge`。

### Phase 1 Review

- 状态：已完成
- 结论：已支持 `.epub` inspect 最小闭环，前端导入弹窗支持 `.plaindoc,.epub`，EPUB 预览展示书名、作者、出版日期、章节数、图片数、目录层级和 warnings。
- 遗留问题：当前仅完成 inspect 预览，commit 仍需在 Phase 4 按 EPUB package type 分发实现。

### Phase 2 Review

- 状态：已完成
- 结论：已完成 zip entry 安全边界、资源限制、EPUB3 nav、EPUB2 toc.ncx、无 nav/toc spine 回退、media-type 扩展名兜底、非 UTF-8 XML 声明 warning，以及目录映射与 fragment 预分配模型。
- 遗留问题：尚未进入 Phase 3，HTML 清洗与 Markdown 转换依赖变更仍需单独确认。

### Phase 3 Review

- 状态：已完成
- 结论：已完成 EPUB HTML 清洗、危险协议降级、HTML 到 Markdown 转换器封装、图片/链接/table/code 等 PlainDoc 自定义转换规则，以及 20/50/100 章批量性能基准；service 包、后端全量测试和构建均已验证。
- 遗留问题：前端 diff 依赖 `@codemirror/merge` 尚未确认，需在 Phase 7 前单独确认或回写替代方案。

### Phase 4 Review

- 状态：已完成
- 结论：已完成 EPUB commit 分发、创建新空间、EPUB3 nav / EPUB2 toc.ncx / spine fallback 目录写入、Markdown 文档与首版 revision 写入、图片本地化、内部链接回写、staging 成功删除与失败保留、失败回滚，以及现有全局导入任务中心/SSE 进度接入。阶段 review 发现慢客户端缓冲可能丢失终态事件的问题，已补回归测试并修复为终态事件优先送达。
- 遗留问题：fragment 当前完成预分配、定位校验和链接降级，尚未做正文片段级切分；样本验证与真实 EPUB 转换质量评估留到 Phase 8。

### Phase 5 Review

- 状态：已完成
- 结论：Task 5.1 已完成 revision 前端类型拆分和 HTTP gateway 拆分；Task 5.2 已完成 repository 层 Markdown / Office 摘要分页、详情查询、创建人关联、版本倒序稳定排序和 not found 语义；Task 5.3 已完成 revisions 列表分页参数、pageSize 上限、详情路由、读权限复用和跨文档 revision 防护。后端 revision 列表响应已改为轻量摘要，新增 `format`、`fileName`、`mimeType` 和 `editorUser`，不再返回 `contentMd`。
- 遗留问题：历史版本恢复接口已在 Phase 6 完成，前端弹窗与 diff 交互从 Phase 7 继续实现。

### Phase 6 Review

- 状态：已完成
- 结论：Task 6.1 已完成恢复接口前后端契约、HTTP gateway、后端路由、`baseVersion` 校验和 revision not found 语义；Task 6.2 已完成 Markdown 历史版本恢复，恢复时校验写权限与当前版本，以目标 revision 正文覆盖当前文档，递增版本并新增一条 `document_revisions`，同时复用阅读缓存清理与图片引用同步规则。Task 6.3 已完成 Office 历史版本恢复，恢复时校验 `documents.version` 与 `contentVersion`，复用目标 file revision 的 `blobID/fileName/mimeType` 覆盖当前 source 引用，不重复写 blob，递增版本并新增 `document_file_revisions`，并将阅读渲染状态标记为 pending。Task 6.4 已完成恢复权限与可观测性边界，列表/详情继续复用读权限，恢复接口复用空间写权限，reader 被拒绝、collaborator 可恢复、owner Markdown/Office 恢复均已覆盖，错误响应保留 requestID，成功恢复写结构化日志。
- 遗留问题：Phase 6 恢复接口后端任务已完成；前端历史版本弹窗与 diff 接入从 Phase 7 继续推进。

### Phase 7 Review

- 状态：已完成
- 结论：Task 7.1 已完成历史版本入口，顶栏操作区按“附件、目录、预览模式、主题、历史版本”的顺序展示；入口使用 34px 图标按钮、tooltip 和 `aria-label`，仅在存在当前文档时启用。Task 7.2 已完成弹窗基础状态，左侧版本列表通过 `DataGateway.document.listRevisions(docId, { page, pageSize: 30 })` 初始加载 30 条，支持滚动到底部或点击“加载更多”请求下一页，并在前端按 revision ID 去重追加；加载、空、错误、重试和创建人 fallback 均已覆盖，右侧详情区域独立滚动。Task 7.3 已完成 `@codemirror/merge` 接入，Markdown 历史版本会加载详情接口的 `contentMd`，与当前编辑器 `content` 做只读左右 diff；切换版本时通过请求序号忽略过期详情响应，未保存内容仍参与 diff。Task 7.4 已完成 Office 历史版本元数据展示，选中 docx/xlsx revision 时加载详情接口，展示文件名、MIME、版本号、创建时间、创建人和“不支持二进制差异”说明，不创建文本 diff，并保留恢复入口与 Office 源文件切换提示。Task 7.5 已完成恢复二次确认和成功状态刷新，恢复请求传当前文档版本，成功后刷新版本列表并同步编辑器内容、版本、保存时间和保存状态；Office 恢复额外触发 ONLYOFFICE 编辑配置重载，避免继续使用恢复前的源文件配置。失败时弹窗保持打开，版本冲突提示用户刷新或重新选择。
- 遗留问题：无；进入 Phase 8 回归验证与文档同步。

### Phase 8 Review

- 状态：未开始
- 结论：
- 遗留问题：

---

## 13. 验证记录

> 每次执行验证命令后，在这里追加记录，包含日期、命令、结果和失败原因。

| 日期 | 命令 | 结果 | 备注 |
| ---- | ---- | ---- | ---- |
| 2026-05-17 | 创建执行清单 | 通过 | 仅文档拆解，未运行代码验证 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_Inspect_(AcceptsEPUBPreview\|RejectsInvalidEPUBPreview\|RejectsEmptyAndUnsupportedUpload)' -count=1` | 通过 | 验证 EPUB inspect 成功路径、非法 EPUB 和原 `.plaindoc` 上传校验 |
| 2026-05-17 | `npm run web:build` | 失败 | 命中本地 Node/V8 `Fatal error ... unreachable code` 已知问题 |
| 2026-05-17 | `node --no-opt $(command -v npm) run web:build` | 通过 | 按项目 workaround 完成前端类型检查、客户端构建和 SSR worker 构建 |
| 2026-05-17 | `git diff --check` | 通过 | 未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | 覆盖 service 包现有导入导出测试和新增 EPUB inspect 测试 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestCollectAdminSpaceEPUBEntries' -count=1` | 通过 | 验证 EPUB zip entry 路径、重复、数量、单 entry、总解压量和目录深度限制 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_Inspect_(AcceptsEPUB2TOCPreview\|FallsBackToFlatSpineWithoutNavOrTOC\|WarnsForNonStandardMediaTypeFallback\|WarnsForNonUTF8XMLDeclaration)' -count=1` | 通过 | 验证 EPUB2 toc、无目录回退、media-type 兜底 warning 和非 UTF-8 XML warning |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 2.1/2.2 后端 service 回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 2.1/2.2 未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestPlanAdminSpaceEPUBImportTree\|TestNormalizeAdminSpaceEPUBHref' -count=1` | 通过 | 验证目录映射、href 规范化、fragment 降级、重复 target 参见文档和 target 映射 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 2 完整 service 回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 2 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestPlanAdminSpaceEPUBImportTree\|TestNormalizeAdminSpaceEPUBHref' -count=1` | 通过 | 补充 EPUB 目录规划中文注释后的 targeted 回归 |
| 2026-05-17 | `git diff --check` | 通过 | 补充中文注释与验证记录后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestSanitizeAdminSpaceEPUBChapterHTML' -count=1` | 失败后通过 | TDD 红绿验证：先确认 HTML 清洗函数缺失失败，再实现危险标签、事件属性和危险协议清洗 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestSanitizeAdminSpaceEPUBChapterHTML\|TestPlanAdminSpaceEPUBImportTree\|TestNormalizeAdminSpaceEPUBHref\|TestCollectAdminSpaceEPUBEntries\|TestAdminSpaceImportService_Inspect_(AcceptsEPUB2TOCPreview\|FallsBackToFlatSpineWithoutNavOrTOC\|WarnsForNonStandardMediaTypeFallback\|WarnsForNonUTF8XMLDeclaration\|AcceptsEPUBPreview\|RejectsInvalidEPUBPreview\|RejectsEmptyAndUnsupportedUpload)' -count=1` | 通过 | Phase 3.1 与既有 EPUB inspect / 目录规划回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 3.1 完成后 service 包完整回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 3.1 完成后未发现 whitespace error |
| 2026-05-17 | `go get github.com/JohannesKaufmann/html-to-markdown/v2` | 通过 | 根据“继续下一步”确认加入 Go HTML 转 Markdown 依赖，依赖文件由 Go 工具链更新 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestHTMLMarkdownConverter' -count=1` | 失败后通过 | TDD 红绿验证：先确认转换器接口缺失失败，再实现标题、段落、列表、表格、代码块、链接、图片和危险 URL 降级 |
| 2026-05-17 | `go mod tidy` | 通过 | 整理 `html-to-markdown/v2` 及其传递依赖 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 3.2 完成后 service 包完整回归 |
| 2026-05-17 | `go test -timeout 60s ./...` | 通过 | Phase 3.2 引入 Go 依赖后的后端全量回归 |
| 2026-05-17 | `go build ./...` | 通过 | Phase 3.2 引入 Go 依赖后的后端全量编译检查 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 3.2 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 240s ./internal/service -run '^$' -bench 'BenchmarkHTMLMarkdownConverter_SerialChapters' -benchtime=1x -count=1` | 通过 | Phase 3.3 性能基准：50 章 × 100KiB 串行约 196ms，单章 P95 约 4ms，heap 采样约 27MiB；100 章 × 1MiB 串行约 3893ms，单章 P95 约 41ms |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 3.3 完成后 service 包完整回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 3.3 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestHTMLMarkdownConverter_DegradesUnsafeURLs\|TestHTMLMarkdownConverter_ConvertsPlainDocEditableMarkdown' -count=1` | 失败后通过 | Phase 3 review 修复：外部图片 URL 不再进入 Markdown，未本地化图片统一降级为 alt 文本 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_(Commit_QueuesEPUBWithCreateSpaceSemantics\|Commit_UsesCustomEPUBSpaceName\|Inspect_RejectsEPUBWithoutCreateSpaceCapability)' -count=1` | 失败后通过 | TDD 红绿验证：EPUB commit 记录 package type、忽略客户端 spaceId、默认空间名取 EPUB title、权限走创建空间能力 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_(Commit_QueuesEPUBWithCreateSpaceSemantics\|Commit_UsesCustomEPUBSpaceName\|Inspect_RejectsEPUBWithoutCreateSpaceCapability\|Commit_RejectsOtherActorsStaging\|Commit_RejectsExpiredStaging\|StreamToken_BindsToJobAndActor\|Commit_FailsJobWhenImportCapabilityRevoked)' -count=1` | 通过 | Phase 4.1 与既有 commit / stream token / 权限回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 4.1 完成后 service 包完整回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 4.1 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestRewriteAdminSpaceEPUBInternalLinks\|TestPlanAdminSpaceEPUBImportTree\|TestBuildAdminSpaceEPUBReferenceMarkdown' -count=1` | 失败后通过 | TDD 红绿验证：预分配暴露 canonical target，内部链接 exact 命中、canonical 降级、缺失目标降级和参见模板 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 4.2 完成后 service 包完整回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 4.2 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestLocalizeAdminSpaceEPUBChapterImages' -count=1` | 失败后通过 | TDD 红绿验证：相对路径图片、data image、超大图片、危险 SVG 和单图写入失败降级 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestLocalizeAdminSpaceEPUBChapterImages\|TestAdminSpaceImportService_LocalizeEPUBImagesUsesImportedBlobStorage\|TestAdminSpaceEPUBImageContentTypeSupportsAllowedTypes\|TestParseAdminSpaceEPUBDataImageAssetRejectsOversizedBeforeDecode' -count=1` | 通过 | Phase 4.3 图片本地化、类型支持、20MiB 上限和 blob/image hosting 适配回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 4.3 完成后 service 包完整回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 4.3 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_RestoreEPUBPackage' -count=1` | 失败后通过 | TDD 红绿验证：EPUB commit 写入新空间、EPUB3 nav / EPUB2 toc.ncx 目录树、Markdown 文档、首版 revision、成功删除 staging、失败回滚空间并保留 staging |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 4.4 完成后 service 包完整回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 4.4 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_RunEPUBJobPublishesProgressAndCompletedNewSpaceID' -count=1` | 失败后通过 | TDD 红绿验证：EPUB worker 发布 running、解析、创建空间、章节转换、图片本地化、完成阶段，并在 completed 事件和持久化任务中写入 `newSpaceId` |
| 2026-05-17 | `npm run test:run -- AdminSpaceTransferTaskProvider.test.tsx` | 通过 | 验证全局任务中心可恢复/展示导入任务，completed 事件仅携带 `newSpaceId` 时仍能触发导入完成流程 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 4.5 完成后 service 包完整回归 |
| 2026-05-17 | `npm run build` | 通过 | Phase 4.5 完成后前端类型检查、DropdownMenu 规则、客户端构建和 SSR worker 构建通过 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 4.5 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportStore_PublishKeepsTerminalEventWhenSubscriberBufferIsFull' -count=1` | 失败后通过 | Phase 4 review 修复：慢客户端塞满导入事件缓冲时，`completed/failed` 终态事件仍优先送达 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_RunEPUBJobPublishesProgressAndCompletedNewSpaceID\|TestAdminSpaceImportStore_PublishKeepsTerminalEventWhenSubscriberBufferIsFull\|TestAdminSpaceImportService_RestoreEPUBPackage' -count=1` | 通过 | Phase 4 review 后端重点回归：EPUB 写库、SSE 进度、终态事件保留 |
| 2026-05-17 | `npm run test:run -- AdminSpaceTransferTaskProvider.test.tsx` | 通过 | Phase 4 review 前端任务中心回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 4 review 后 service 包完整回归 |
| 2026-05-17 | `node --no-opt $(command -v npm) run test:run -- adapter.test.ts` | 失败 | TDD 红灯：`gateway.document.getRevisionDetail is not a function`，确认前端详情 gateway 缺失 |
| 2026-05-17 | `go test -timeout 60s ./internal/server -run TestRouter_ListDocumentRevisionsReturnsSummariesWithoutContent -count=1` | 失败后通过 | TDD 红绿验证：先确认版本列表泄露 `contentMd`，再改为摘要响应并补 `format/editorUser` |
| 2026-05-17 | `npm run test:run -- adapter.test.ts` | 通过 | Phase 5.1 前端 HTTP adapter 回归：列表和详情 gateway 分别请求 |
| 2026-05-17 | `go test -timeout 60s ./internal/server ./internal/storage/repository -count=1` | 通过 | Phase 5.1 后端 handler 与 workspace repository 回归 |
| 2026-05-17 | `npm run build` | 通过 | Phase 5.1 前端类型检查、DropdownMenu 规则、客户端构建和 SSR worker 构建通过 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 5.1 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/storage/repository -run 'TestGormWorkspaceRepository_(ListRevisionSummariesPaginatesMarkdownAndOffice\|GetRevisionDetailByIDReturnsMarkdownAndOffice)' -count=1` | 失败后通过 | TDD 红绿验证：新增 repository 分页摘要和详情查询，覆盖 Markdown / Office / 跨文档 not found |
| 2026-05-17 | `go test -timeout 60s ./internal/storage/repository -count=1` | 通过 | Phase 5.2 repository 包完整回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/server -run TestRouter_ListDocumentRevisionsReturnsSummariesWithoutContent -count=1` | 通过 | Phase 5.2 兼容上一阶段 revisions 列表摘要响应 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -count=1` | 通过 | Phase 5.2 新增 WorkspaceRepository 接口方法后的 service 包编译与行为回归 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 5.2 完成后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/server -run 'TestRouter_(ListDocumentRevisionsPaginatesAndRejectsNoReadPermission\|GetDocumentRevisionDetailReturnsMarkdownAndOffice\|GetDocumentRevisionDetailRejectsNoPermissionAndCrossDocumentRevision)' -count=1` | 失败后通过 | TDD 红绿验证：新增 revisions 分页列表、详情路由、Markdown 正文、Office 文件元数据、无权限拒绝和跨文档 revision 防护 |
| 2026-05-17 | `go test -timeout 60s ./internal/server ./internal/storage/repository -count=1` | 通过 | Phase 5.3 handler / router / repository 回归 |
| 2026-05-17 | `npm run test:run -- adapter.test.ts` | 通过 | Phase 5.3 前端 HTTP adapter 回归 |
| 2026-05-17 | `npm run build` | 通过 | Phase 5.3 前端类型检查、DropdownMenu 规则、客户端构建和 SSR worker 构建通过 |
| 2026-05-17 | `git diff --check` | 通过 | Phase 5.3 完成后未发现 whitespace error |
| 2026-05-17 | `npm run test:run -- DocumentRevisionHistoryDialog.test.tsx` | 失败后通过 | Task 7.4 TDD 红绿验证：先确认 Office 元数据区域缺失，再实现详情加载、文件名/MIME/版本/创建人展示和不创建文本 diff |
| 2026-05-17 | `npm run check:dropdown-menu` | 通过 | Task 7.4 前端规则回归，未发现 DropdownMenu 违规用法 |
| 2026-05-17 | `npm run build` | 失败 | SSR worker 阶段命中本地 Node/V8 `Fatal error ... unreachable code` 已知问题；类型检查和 client build 已完成 |
| 2026-05-17 | `node --no-opt $(command -v npm) run build` | 通过 | Task 7.4 前端类型检查、DropdownMenu 规则、客户端构建和 SSR worker 构建通过 |
| 2026-05-17 | `git diff --check` | 通过 | Task 7.4 完成后未发现 whitespace error |
| 2026-05-17 | `npm run test:run -- DocumentRevisionHistoryDialog.test.tsx` | 失败后通过 | Task 7.5 TDD 红绿验证：先确认恢复按钮仍禁用、确认框缺失，再实现二次确认、restore API 调用、成功刷新列表和版本冲突提示 |
| 2026-05-17 | `npm run check:dropdown-menu` | 通过 | Task 7.5 前端规则回归，未发现 DropdownMenu 违规用法 |
| 2026-05-17 | `npm run build` | 通过 | Task 7.5 前端类型检查、DropdownMenu 规则、客户端构建和 SSR worker 构建通过 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'Test(AdminSpaceImportService_(Inspect_(AcceptsEPUBPreview\|AcceptsEPUB2TOCPreview\|FallsBackToFlatSpineWithoutNavOrTOC\|WarnsForNonStandardMediaTypeFallback\|WarnsForNonUTF8XMLDeclaration\|RejectsInvalidEPUBPreview\|RejectsEPUBWithoutCreateSpaceCapability)\|Commit_(QueuesEPUBWithCreateSpaceSemantics\|UsesCustomEPUBSpaceName\|RejectsExpiredStaging\|FailsJobWhenImportCapabilityRevoked)\|RestoreEPUBPackage(CreatesSpaceTreeDocumentsAndRevisions\|UsesEPUB2TOCTree\|CleansSpaceOnCreateNodeFailureAndKeepsStaging)\|RunEPUBJobPublishesProgressAndCompletedNewSpaceID\|LocalizeEPUBImagesUsesImportedBlobStorage)\|CollectAdminSpaceEPUBEntries_RejectsUnsafeEntries\|CollectAdminSpaceEPUBEntries_RejectsTooManyEntries\|PlanAdminSpaceEPUBImportTree_BuildsFoldersDocsFragmentsAndReferences\|NormalizeAdminSpaceEPUBHref\|RewriteAdminSpaceEPUBInternalLinks_(RewritesAndDegradesTargets\|ConvertedMarkdownHasNoEPUBRelativeLinks)\|LocalizeAdminSpaceEPUBChapterImages_(LocalizesRelativeAndDataImages\|DegradesOversizedDangerousAndFailedImages)\|SanitizeAdminSpaceEPUBChapterHTML_RemovesDangerousHTML\|HTMLMarkdownConverter_(ConvertsPlainDocEditableMarkdown\|DegradesUnsafeURLs\|DegradesComplexTables)\|ParseAdminSpaceEPUBDataImageAssetRejectsOversizedBeforeDecode\|AdminSpaceEPUBImageContentTypeSupportsAllowedTypes\|AdminSpaceImportService_CleanupExpiredTransfersDeletesExpiredStagingFile)$' -count=1` | 通过 | Task 8.1 后端 EPUB 矩阵：EPUB2/3、spine fallback、目录规划、HTML/Markdown、内部链接、图片本地化、安全降级、权限和 staging 清理 |
| 2026-05-17 | `go test -timeout 60s ./internal/server ./internal/storage/repository -run 'Test(Router_(ListDocumentRevisionsReturnsSummariesWithoutContent\|ListDocumentRevisionsPaginatesAndRejectsNoReadPermission\|GetDocumentRevisionDetailReturnsMarkdownAndOffice\|GetDocumentRevisionDetailRejectsNoPermissionAndCrossDocumentRevision\|RestoreDocumentRevisionContractValidatesBaseVersionAndNotFound\|RestoreMarkdownDocumentRevisionCreatesNewVersion\|RestoreMarkdownDocumentRevisionRejectsStaleBaseVersion\|RestoreOfficeDocumentRevisionCreatesNewFileRevision\|RestoreOfficeDocumentRevisionRejectsAdvancedContentVersion\|RestoreDocumentRevisionUsesWritePermissionAndRequestID)\|GormWorkspaceRepository_(ListRevisionSummariesPaginatesMarkdownAndOffice\|GetRevisionDetailByIDReturnsMarkdownAndOffice))$' -count=1` | 通过 | Task 8.1 后端 revision 矩阵：列表、详情、恢复、权限、冲突、不存在 revision 与 repository 查询 |
| 2026-05-17 | `go test -timeout 60s ./internal/service ./internal/server ./internal/storage/repository -count=1` | 通过 | Task 8.1 相关后端包完整回归 |
| 2026-05-17 | `npm run test:run -- AdminSpacesPage.test.tsx` | 失败后通过 | Task 8.2 TDD 红绿验证：先确认导入弹窗旧断言仍排除 EPUB，再补充 `.epub` accept、EPUB 预览统计、warnings 和 `.plaindoc` 不展示 EPUB 专属统计 |
| 2026-05-17 | `npm run test:run -- DocumentRevisionHistoryDialog.test.tsx` | 通过 | Task 8.2 历史版本弹窗矩阵：加载、空态、错误态、分页、详情切换、Markdown diff、未保存禁用恢复、确认恢复、Office 元数据和恢复成功状态同步 |
| 2026-05-17 | `npm run test:run -- adapter.test.ts` | 通过 | Task 8.2 data-access 回归：版本列表、详情和恢复均通过 gateway 层，不直接 fetch |
| 2026-05-17 | `npm run test:run -- AdminSpaceTransferTaskProvider.test.tsx` | 通过 | Task 8.2 前端任务中心回归：导入任务恢复、进度和完成事件路径保持可用 |
| 2026-05-17 | `npm run check:dropdown-menu -w @plaindoc/web` | 通过 | Task 8.2 前端规则回归，未发现 DropdownMenu 违规用法 |
| 2026-05-17 | `rg --files \| rg '\.epub$'` / `find /tmp -maxdepth 3 -type f -name '*.epub'` / `find /home/lifei6671/src -maxdepth 4 -type f -name '*.epub'` | 阻塞 | Task 8.3 样本查找未发现可用 EPUB，已新增 `docs/epub-import-sample-validation.md` 记录待补样本矩阵 |
| 2026-05-17 | `gofmt -w .` | 通过 | Task 8.4 在 `apps/server` 执行 Go 格式化 |
| 2026-05-17 | `go build ./...` | 通过 | Task 8.4 在 `apps/server` 执行后端全量编译检查 |
| 2026-05-17 | `go test -timeout 60s ./...` | 通过 | Task 8.4 在 `apps/server` 执行后端全量单测回归 |
| 2026-05-17 | `go test -race -timeout 60s ./...` | 失败 | Task 8.4 race 门禁在 `internal/service` 包 60s 超时，输出未见 race 报告 |
| 2026-05-17 | `go test -race -timeout 180s ./internal/service` | 通过 | 诊断性复跑：`internal/service` race 模式 110.743s 通过，说明 60s 失败主要是耗时门限问题 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'Test(HTMLMarkdownConverter\|AdminSpaceImportService_(Inspect_(AcceptsEPUBPreview\|AcceptsEPUB2TOCPreview\|FallsBackToFlatSpineWithoutNavOrTOC)\|RestoreEPUBPackage)\|CollectAdminSpaceEPUBEntries\|PlanAdminSpaceEPUBImportTree\|SanitizeAdminSpaceEPUBChapterHTML)' -count=1` | 通过 | 修复本次 EPUB/Markdown lint 问题后的 targeted service 回归 |
| 2026-05-17 | `golangci-lint run ./...` | 失败 | Task 8.4 lint 门禁：本次 EPUB/Markdown 新增 lint 已清理，仍剩既存 unused/gosimple/ineffassign/staticcheck 问题 |
| 2026-05-17 | `npm run web:build` | 失败 | Task 8.4 前端构建首次执行命中本地 Node/V8 `Fatal error ... unreachable code` 已知问题，DropdownMenu 检查已先通过 |
| 2026-05-17 | `node --no-opt $(command -v npm) run build` | 通过 | Task 8.4 workaround 构建通过：完成 DropdownMenu 检查、TypeScript 编译、客户端构建和 SSR worker 构建 |
| 2026-05-17 | `npm run check:dropdown-menu -w @plaindoc/web` | 通过 | Task 8.4 单独执行前端规则门禁，未发现 DropdownMenu 违规用法 |
| 2026-05-17 | 文档同步 | 通过 | Task 8.5 更新技术方案状态、后端指南、前端指南、README 导航、执行清单和 EPUB 样本验证记录 |
| 2026-05-17 | `git diff --check` | 通过 | Task 8.5 文档同步和空白修复后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run TestAdminSpaceImportService_Inspect_AcceptsEPUBPreview -count=1` / `npm run test:run -- AdminSpacesPage.test.tsx` | 失败后通过 | 修复后台 EPUB inspect 空 warnings 被序列化为 null 导致导入弹窗读取 `warnings.length` 崩溃的问题 |
| 2026-05-17 | `go test -timeout 60s ./internal/service ./internal/server -run 'Test(AdminSpaceImport\|Router_.*SpaceImport\|AdminSpaceImportService_Inspect_AcceptsEPUBPreview)' -count=1` | 通过 | EPUB 导入 inspect 响应 warnings 数组语义和相关 handler/service 回归 |
| 2026-05-17 | `node --no-opt $(command -v npm) run build` | 通过 | EPUB 导入弹窗 warnings 兜底修复后，前端类型检查、客户端构建和 SSR worker 构建通过 |
| 2026-05-17 | `git diff --check` | 通过 | EPUB 导入 warnings 修复后未发现 whitespace error |
| 2026-05-17 | `go test -timeout 60s ./internal/storage/repository -run 'TestGormWorkspaceRepository_ListRevisionSummariesBoundsSourceQueries' -count=1` | 失败后通过 | Review 修复：历史版本摘要分页的 Markdown / Office 来源查询都下推 `LIMIT offset+limit`，避免长历史文档每页全量加载 |
| 2026-05-17 | `npm run test:run -- onlyoffice-edit-config.test.ts` | 失败后通过 | Review 修复：Office 历史版本恢复后必须触发 ONLYOFFICE 编辑配置重载，Markdown 恢复不触发 |
| 2026-05-17 | `go test -timeout 60s ./internal/storage/repository ./internal/server -run 'TestGormWorkspaceRepository_(ListRevisionSummaries\|GetRevisionDetail\|SaveOfficeDocument)\|TestRouter_.*Revision' -count=1` | 通过 | Review 修复后端历史版本 repository / handler / 恢复回归 |
| 2026-05-17 | `npm run test:run -- DocumentRevisionHistoryDialog.test.tsx onlyoffice-edit-config.test.ts` | 通过 | Review 修复前端历史版本弹窗和 Office 配置刷新规则回归 |
| 2026-05-17 | `node --no-opt $(command -v npm) run build` | 失败 | 本轮构建仍命中本地 Node/V8 `Fatal error ... unreachable code`，崩溃发生在 npm 子进程；改用直接脚本逐项验证 |
| 2026-05-17 | `node ./scripts/check-dropdown-menu-modal.mjs` | 通过 | 直接执行 DropdownMenu 规则检查，未发现 `modal=true` 或绕过封装 |
| 2026-05-17 | `node --no-opt ./node_modules/typescript/bin/tsc -b apps/web/tsconfig.json` | 通过 | 直接执行前端 TypeScript project build，规避 npm 子进程 V8 崩溃 |
| 2026-05-17 | `node --no-opt ../../node_modules/vite/bin/vite.js build`（cwd: `apps/web`） | 通过 | 直接执行 client build，规避 npm 子进程 V8 崩溃 |
| 2026-05-17 | `node --no-opt ../../node_modules/vite/bin/vite.js build --config vite.ssr.config.ts`（cwd: `apps/web`） | 通过 | 直接执行 SSR worker build，规避 npm 子进程 V8 崩溃 |
| 2026-05-17 | `npm run test:run -- AdminSpaceTransferTaskProvider.test.tsx` | 失败后通过 | 大 EPUB 导入 SSE 连接异常不再把前端任务中心状态误标为失败，保留已有进度并关闭失效订阅 |
| 2026-05-17 | `go test -timeout 60s ./internal/config ./internal/ssr/worker -count=1` | 失败后通过 | 新增 `SSR_WORKER_MAX_RESPONSE_BYTES=16777216` 默认值和 SSR codec 大响应读取回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/config ./internal/ssr/... -count=1` | 通过 | SSR worker 请求/响应 payload 限制拆分后回归 |
| 2026-05-17 | `node --no-opt $(command -v npm) run web:build` | 失败 | 本地 Node/V8 `Fatal error ... unreachable code` 仍会在 npm 子进程触发，已改用直接脚本逐项验证 |
| 2026-05-17 | `node ./scripts/check-dropdown-menu-modal.mjs` / `node --no-opt ./node_modules/typescript/bin/tsc -b apps/web/tsconfig.json` / `git diff --check` | 通过 | 前端规则、TypeScript project build 和 whitespace 检查通过 |
| 2026-05-17 | `node --no-opt ../../node_modules/vite/bin/vite.js build`（cwd: `apps/web`） | 通过 | 直接执行 client build，规避 npm 子进程 V8 崩溃 |
| 2026-05-17 | `node --no-opt ../../node_modules/vite/bin/vite.js build --config vite.ssr.config.ts`（cwd: `apps/web`） | 通过 | 直接执行 SSR worker build，规避 npm 子进程 V8 崩溃 |
| 2026-05-17 | `go test -timeout 60s ./internal/config -count=1` | 失败后通过 | SSR worker 请求/响应上限支持 `1MB`、`16 MiB` 等带单位写法，未知单位会拒绝启动 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run TestAdminSpaceImportService_RunEPUBJobPublishesProgressAndCompletedNewSpaceID -count=1` | 失败后通过 | EPUB 导入进度改为按已写入文档数推进，2 个文档样本按 1/2、2/2 推进到 62% 和 90% |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_(RunEPUBJobPublishesProgressAndCompletedNewSpaceID\|RestoreEPUBPackageCreatesSpaceTreeDocumentsAndRevisions\|RestoreEPUBPackageUsesEPUB2TOCTree)' -count=1` | 通过 | EPUB job 进度、EPUB3 写库和 EPUB2 toc 写库回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceImportService_RestoreEPUBPackage(UsesCoverAndDescription\|CreatesSpaceTreeDocumentsAndRevisions)\|TestAdminSpaceImportService_RunEPUBJobPublishesProgressAndCompletedNewSpaceID' -count=1` | 通过 | EPUB OPF `description` 写入空间简介，EPUB3 `cover-image` 写入空间封面资产，同时回归 EPUB 写库与进度 |
| 2026-05-17 | `npm run test:run -- AdminSpaceTransferTaskProvider.test.tsx` | 失败后通过 | 大 EPUB SSE 断线后主动刷新任务快照：运行中任务会重连，已完成任务会直接触发导入完成回调 |
| 2026-05-17 | `npm run test:run -- adapter.test.ts` | 失败后通过 | SSE `EventSource.onerror` 先等待浏览器自动重连，短暂中断不会立刻触发任务快照刷新和 stream token 续签 |
| 2026-05-17 | `npm run test:run -- AdminSpaceTransferTaskProvider.test.tsx` | 通过 | SSE 自动重连窗口加入后，全局任务中心导入/导出恢复逻辑保持可用 |
| 2026-05-17 | `go test -timeout 60s ./internal/ssr/worker -run TestBuildSSRWorkerCommandArgs -count=1` | 失败后通过 | SSR Worker 使用 Node 启动时自动注入 `--no-opt`，避免大空间导出 SSR 渲染触发 Node/V8 优化器原生崩溃 |
| 2026-05-17 | `go test -timeout 60s ./internal/ssr/... -count=1` | 通过 | SSR worker / pool / protocol 包回归 |
| 2026-05-17 | `node --no-opt -e "console.log('node-no-opt-ok')"` | 通过 | 当前本地 Node 支持 `--no-opt` 启动参数 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run TestAdminSpaceExportService_WritesEPUBPackageWithMarkdownAndOfficeHTML -count=1` | 失败后通过 | EPUB 导出文件名改为使用清理后的空间名，覆盖空间名中包含路径非法字符的场景 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run TestAdminSpaceExportService_EPUBPublishProgressByRenderedDocuments -count=1` | 失败后通过 | EPUB 导出渲染阶段按章节数发布 `epub_documents` 进度，2 个章节从 55% 后推进到 90% |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceExportService_(EPUBPublishProgressByRenderedDocuments\|WritesEPUBPackageWithMarkdownAndOfficeHTML\|WritesEPUBWithNestedDocumentTree)' -count=1` | 通过 | EPUB 导出章节进度、文件名、Markdown/Office 渲染和嵌套目录回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run TestAdminSpaceExportService_WritesEPUBPackageWithMarkdownAndOfficeHTML -count=1` | 失败后通过 | Review 修复：EPUB 导出写入空间封面，覆盖 `cover.xhtml`、`meta name="cover"`、`cover-image` manifest 和封面图片 payload |
| 2026-05-17 | `go test -timeout 60s ./internal/service -run 'TestAdminSpaceExportService_(WritesEPUBPackageWithMarkdownAndOfficeHTML\|EPUBPublishProgressByRenderedDocuments\|WritesEPUBWithNestedDocumentTree)' -count=1` | 通过 | EPUB 导出封面、章节进度、Markdown/Office 渲染和嵌套目录回归 |
| 2026-05-17 | `go test -timeout 60s ./internal/server/handler -run TestAdminSpaceTransferSSEHeartbeatIntervalStaysBelowCommonProxyIdleTimeout -count=1` | 失败后通过 | SSE heartbeat 间隔固定为 5s，确保低于常见 10s 代理 idle timeout |
| 2026-05-17 | `go test -timeout 60s ./internal/server/handler -count=1` | 通过 | 后端 handler 包回归，导入/导出共用 SSE handler 保持可用 |
| 2026-05-17 | `go test -timeout 60s ./internal/server/middleware -run 'TestTimeout(SkipsServerSentEventRequests\|KeepsDeadlineForRegularRequests)' -count=1` | 失败后通过 | SSE 请求不再继承 `REQUEST_TIMEOUT` deadline，普通请求仍保留业务超时 |
| 2026-05-17 | `go test -timeout 60s ./internal/server/middleware ./internal/server/handler -count=1` | 通过 | middleware 和 handler 回归：SSE timeout 例外、heartbeat 和 handler 行为保持可用 |
| 2026-05-17 | `go test -timeout 60s ./internal/server/middleware ./internal/server/handler` | 失败后通过 | Review 修复：只有带 `Accept: text/event-stream` 的真实 SSE 请求跳过 `REQUEST_TIMEOUT`；SSE 写截止时间清理失败会返回错误并记录 warning |
| 2026-05-17 | `npm run test -- onlyoffice-edit-config.test.ts DocumentRevisionHistoryDialog.test.tsx` | 通过 | Review 修复：Office dirty 状态映射为历史恢复未保存保护，Markdown diff 弹窗回归保持可用 |
