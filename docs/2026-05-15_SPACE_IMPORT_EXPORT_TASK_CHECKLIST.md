# 空间导入导出执行清单

**文档状态**: Draft  
**创建日期**: 2026-05-15  
**关联方案**: `docs/2026-05-15_SPACE_EXPORT_TECHNICAL_PLAN.md`  
**适用范围**: `apps/server`、`apps/web`、`docs`  
**目标**: 将空间导入导出技术方案拆成可执行、可回写进度、可分阶段 code review 的任务清单。完成后支持可回灌 zip 空间交换包导出、zip 导入新空间、EPUB 阅读包导出、SSE 进度推送、短期下载链接和后台审计。

---

## 0. 执行原则

1. 每完成一个 checkbox，必须在本文件中回写 `[x]`。
2. 每个 Phase 完成后停下来做 code review，再进入下一阶段。
3. 涉及依赖变更时先确认；本清单中 `github.com/go-shiori/go-epub` 属于新增 Go 依赖。
4. `go.sum` 禁止手工编辑，只能由 Go 工具链生成。
5. 后端接口返回继续遵守 `JsonResult`：`{ code, message, requestId, data }`。
6. 前端页面层禁止直接散写业务 `fetch`，统一走 `apps/web/src/data-access/*`。
7. 数据库结构第一期不新增表；导入导出任务使用进程内任务表和私有临时文件目录。
8. zip 空间交换包是唯一可导入格式；EPUB 是阅读包，不参与导入。

---

## 1. 依赖关系与并行批次

### 1.1 必须串行的关口

```text
Phase 1 协议与骨架
  → Phase 2 任务 Store / SSE / Token
  → Phase 3 Zip 导出最小闭环
  → Phase 4 附件与 Office source
  → Phase 6 导入解析
  → Phase 7 导入落地
  → Phase 8 下载、审计、清理
  → Phase 9 测试与文档同步
```

### 1.2 可并行的工作

在 Phase 1 和 Phase 2 稳定后，可分三路推进：

1. 后端 zip 导出链路：Phase 3、Phase 4。
2. 前端导入导出浮层：Phase 6 中前端部分可先做 mock 接入。
3. EPUB 阅读包导出：Phase 5 可在 zip 导出数据读取能力稳定后并行推进。

单轮并行任务最多 5 个，且不得同时修改同一文件区域。

---

## 2. 文件规划

### 2.1 后端新增文件

- [x] `apps/server/internal/service/admin_space_export_service.go`
  - 空间导出服务：导出任务创建、zip/EPUB 生成、下载链接、SSE 事件。
- [x] `apps/server/internal/service/admin_space_import_service.go`
  - 空间导入服务：zip staging、inspect、commit、导入任务执行、ID 映射。
- [x] `apps/server/internal/service/admin_space_export_manifest.go`
  - 空间交换包 manifest/tree 类型、校验、路径清洗、hash 计算。
- [x] `apps/server/internal/server/handler/admin_space_export.go`
  - 后台导出 handler：创建导出任务、SSE、下载。
- [x] `apps/server/internal/server/handler/admin_space_import.go`
  - 后台导入 handler：inspect、commit、SSE。
- [x] `apps/server/internal/server/response/error_admin_space_export.go`
  - 空间导出错误码和响应定义。
- [x] `apps/server/internal/server/response/error_admin_space_import.go`
  - 空间导入错误码和响应定义。

### 2.2 后端修改文件

- [x] `apps/server/internal/server/router.go`
  - 注册导入导出 API。
- [ ] `apps/server/internal/server/handler/workspace.go`
  - 如需复用 Office source blob 私有 helper，先抽公共方法，避免复制大段逻辑。
- [ ] `apps/server/internal/storage/repository/interfaces.go`
  - 若现有仓储接口缺失导入导出所需方法，补最小接口。
- [ ] `apps/server/internal/storage/repository/*`
  - 补齐最小 GORM 查询或写入实现。
- [ ] `apps/server/go.mod`
  - 经确认后加入 `github.com/go-shiori/go-epub`。
- [ ] `apps/server/go.sum`
  - 由 Go 工具链自动更新。

### 2.3 后端测试文件

- [x] `apps/server/internal/service/admin_space_export_service_test.go`
- [x] `apps/server/internal/service/admin_space_import_service_test.go`
- [ ] `apps/server/internal/server/admin_space_export_handler_test.go`
- [ ] `apps/server/internal/server/admin_space_import_handler_test.go`

如仓库现有测试集中放在 `admin_handler_test.go`，可追加到现有文件；但新增文件更容易控制篇幅。

### 2.4 前端新增文件

- [ ] `apps/web/src/admin/components/AdminSpaceExportDialog.tsx`
- [ ] `apps/web/src/admin/components/AdminSpaceImportDialog.tsx`
- [ ] `apps/web/src/admin/components/AdminSpaceTransferProgress.tsx`
  - 可选：导入导出共用进度条和状态展示。

### 2.5 前端修改文件

- [x] `apps/web/src/data-access/types.ts`
  - 新增导入导出类型和 `AdminGateway` 方法。
- [x] `apps/web/src/data-access/http/adapter.ts`
  - 新增 API 调用、SSE 订阅封装、下载 URL 处理。
- [ ] `apps/web/src/admin/pages/AdminSpacesPage.tsx`
  - 增加导出菜单项、导入按钮、弹层状态。
- [ ] `apps/web/src/components/ui/*`
  - 如现有组件不足，优先复用，不新增无必要 UI 基础组件。

### 2.6 前端测试文件

- [ ] `apps/web/src/admin/pages/AdminSpacesPage.test.tsx`
- [ ] `apps/web/src/admin/components/AdminSpaceExportDialog.test.tsx`
- [ ] `apps/web/src/admin/components/AdminSpaceImportDialog.test.tsx`

### 2.7 文档文件

- [ ] `docs/2026-05-15_SPACE_EXPORT_TECHNICAL_PLAN.md`
  - 实现中如有方案偏离，必须先确认后更新。
- [ ] `docs/2026-05-15_SPACE_IMPORT_EXPORT_TASK_CHECKLIST.md`
  - 本执行清单，作为推进控制面板。
- [ ] `docs/BACKEND_DEVELOPER_GUIDE.md`
  - Phase 9 同步导入导出后端说明。
- [ ] `docs/FRONTEND_DEVELOPER_GUIDE.md`
  - Phase 9 同步导入导出前端说明。
- [ ] `docs/README.md`
  - 已加入专题文档入口；如文件名调整需同步更新。

---

## 3. Phase 1：协议与骨架

**目标**: 先打通导入导出 API 契约、类型枚举、路由骨架和权限边界，不写真实文件生成逻辑。

### Task 1.1 后端类型与错误码

**文件**

- 修改：`apps/server/internal/server/response/*`
- 创建：`apps/server/internal/service/admin_space_export_manifest.go`
- 创建：`apps/server/internal/service/admin_space_export_service.go`
- 创建：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [x] 定义导出格式枚举：`markdown_zip`、`source_zip`、`epub`。
- [x] 定义 zip 空间交换包常量：`packageType = "plaindoc-space"`、`version = 1`。
- [x] 定义导出请求结构：`format`、`includeAttachments`、`includeOfficeSources`。
- [x] 定义导出启动响应：`jobId`、`streamUrl`。
- [x] 定义导入 inspect 响应：`importId`、`packageVersion`、`packageType`、`importable`、`space`、`summary`、`warnings`。
- [x] 定义导入 commit 请求：`spaceName`、`spaceId`、`categoryId`、`visibility`。
- [x] 定义导入 commit 响应：`jobId`、`streamUrl`。
- [x] 新增错误语义：
  - `AdminSpaceExportErrSpaceIDRequired`
  - `AdminSpaceExportErrRequestBody`
  - `AdminSpaceExportErrFormatUnsupported`
  - `AdminSpaceExportErrJobNotFound`
  - `AdminSpaceExportErrJobTokenInvalid`
  - `AdminSpaceExportErrJobRunningLimit`
  - `AdminSpaceExportErrFileNotReady`
  - `AdminSpaceExportErrFileExpired`
  - `AdminSpaceExportErrDownloadForbidden`
  - `AdminSpaceImportErrFileRequired`
  - `AdminSpaceImportErrZipInvalid`
  - `AdminSpaceImportErrManifestMissing`
  - `AdminSpaceImportErrTreeMissing`
  - `AdminSpaceImportErrPackageUnsupported`
  - `AdminSpaceImportErrPackageNotImportable`
  - `AdminSpaceImportErrStagingNotFound`
  - `AdminSpaceImportErrStagingExpired`
  - `AdminSpaceImportErrJobRunningLimit`
  - `AdminSpaceImportErrJobTokenInvalid`
  - `AdminSpaceImportErrCommitForbidden`

**验收**

- [x] 后端能编译到类型层面。
- [x] 错误命名和现有 response 包风格一致。
- [x] 没有新增数据库迁移。

### Task 1.2 后端路由与 handler 骨架

**文件**

- 修改：`apps/server/internal/server/router.go`
- 创建：`apps/server/internal/server/handler/admin_space_export.go`
- 创建：`apps/server/internal/server/handler/admin_space_import.go`

**步骤**

- [x] 注册 `POST /api/admin/spaces/:spaceId/exports`。
- [x] 注册 `GET /api/admin/spaces/:spaceId/exports/:jobId/events`。
- [x] 注册 `GET /api/admin/space-exports/:jobId/download`。
- [x] 注册 `POST /api/admin/space-imports/inspect`。
- [x] 注册 `POST /api/admin/space-imports/:importId/commit`。
- [x] 注册 `GET /api/admin/space-imports/:jobId/events`。
- [x] 导出创建接口使用 `RequireAdminSession` 取得 actor，再由 service 调用 `CanExportSpace(actorUserID, spaceID)`。
- [x] 导入 inspect 和 commit 使用 `RequireAdminSession` 取得 actor，再由 service 调用 `CanImportSpace(actorUserID)`。
- [x] handler 骨架返回明确的“未实现”或调用 service stub，不能 panic。

**验收**

- [x] 路由命名和技术方案一致。
- [x] EventSource URL 不要求 Authorization header。
- [x] 导入 commit 只允许具备导入能力的 actor：有空间管理权限，或普通用户当前允许创建空间。

### Task 1.3 前端契约类型

**文件**

- 修改：`apps/web/src/data-access/types.ts`

**步骤**

- [x] 新增 `AdminSpaceExportFormat = "markdown_zip" | "source_zip" | "epub"`。
- [x] 新增 `AdminSpaceExportStartInput`。
- [x] 新增 `AdminSpaceExportStartResult`。
- [x] 新增 `AdminSpaceTransferEvent` 或导入导出共用事件类型。
- [x] 新增 `AdminSpaceImportInspectResult`。
- [x] 新增 `AdminSpaceImportCommitInput`。
- [x] 新增 `AdminSpaceImportStartResult`。
- [x] 扩展 `AdminGateway`：
  - `startSpaceExport`
  - `subscribeSpaceExport`
  - `inspectSpaceImport`
  - `commitSpaceImport`
  - `subscribeSpaceImport`

**验收**

- [x] TypeScript 类型无循环引用。
- [x] EPUB 被标记为导出格式，但不是导入包格式。
- [x] 页面层后续可完全依赖 `AdminGateway`，不需要直接拼 URL。

### Task 1.4 HTTP adapter 骨架

**文件**

- 修改：`apps/web/src/data-access/http/adapter.ts`

**步骤**

- [x] 实现 `startSpaceExport` 请求骨架。
- [x] 实现 `inspectSpaceImport` multipart 上传骨架。
- [x] 实现 `commitSpaceImport` 请求骨架。
- [x] 实现 `subscribeSpaceExport` 的 EventSource 封装。
- [x] 实现 `subscribeSpaceImport` 的 EventSource 封装。
- [x] 订阅对象返回 `close()`。
- [x] SSE 回调至少区分 `progress`、`completed`、`failed`。

**验收**

- [x] adapter 内部统一拼接 `baseUrl`。
- [x] 页面层不用读取 localStorage token。
- [x] SSE 连接关闭时不会遗留 listener。

**Phase 1 验证命令**

```bash
npm run web:build
cd apps/server && go test ./... -count=1
```

**Phase 1 Review Gate**

- [ ] code review 确认 API 契约稳定。
- [ ] code review 确认权限边界无误。
- [x] 更新本清单 Phase 1 checkbox。

---

## 4. Phase 2：任务 Store、Token 与 SSE

**目标**: 建立导入导出后台任务生命周期，打通 queued/running/completed/failed 事件推送。

### Task 2.1 导出任务 Store

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`

**步骤**

- [x] 定义 `AdminSpaceExportJob`。
- [x] 定义 `AdminSpaceExportStatus`：`queued`、`running`、`completed`、`failed`。
- [x] 实现进程内 `AdminSpaceExportJobStore`。
- [x] Store 内部使用 `sync.Mutex` 保护任务 map。
- [x] 支持同一 `actorUserID + spaceID` running 限制。
- [x] 支持全局 running 导出任务上限 2。
- [x] 支持订阅者注册、注销、广播事件。
- [x] 导出 worker 从 queued 进入 running 前再次调用 `CanExportSpace(actorUserID, spaceID)`。

**验收**

- [ ] 并发读写无 data race。
- [x] 重复创建 running 任务返回明确错误。
- [x] 订阅者关闭后不阻塞广播。

### Task 2.2 导入 staging 与任务 Store

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [x] 定义 `AdminSpaceImportStaging`。
- [x] 定义 `AdminSpaceImportJob`。
- [x] 实现 staging map，绑定 `importId + actorUserId`。
- [x] 实现导入任务 Store。
- [x] 支持同一 actor 同时最多 1 个 running 导入任务。
- [x] 支持 staging 过期时间。
- [x] 支持导入任务事件广播。

**验收**

- [x] actor 不能提交别人的 `importId`。
- [x] staging 过期后不能 commit。
- [x] 导入任务 Store 不依赖数据库表。

### Task 2.3 短期 Token

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [x] 实现导出 `streamToken`。
- [x] 实现导出 `downloadToken`。
- [x] 实现导入 `importStreamToken`。
- [x] token 绑定 actor、space/import/job、过期时间。
- [x] token 只保存哈希或不可逆摘要，响应里只返回一次明文 token。
- [x] token 不在日志中打印。
- [x] token 校验失败返回专用错误。

**验收**

- [x] token 不能跨用户复用。
- [x] token 不能跨 job 复用。
- [x] 过期 token 被拒绝。
- [x] download token 单次使用，下载成功或过期后不可再次使用。

### Task 2.4 SSE handler

**文件**

- 修改：`apps/server/internal/server/handler/admin_space_export.go`
- 修改：`apps/server/internal/server/handler/admin_space_import.go`

**步骤**

- [x] 设置 `Content-Type: text/event-stream`。
- [x] 设置 `Cache-Control: no-cache`。
- [x] 设置 `X-Accel-Buffering: no`。
- [x] 首次连接推送当前任务快照。
- [x] 任务 completed 或 failed 后关闭连接。
- [x] 客户端断开时注销订阅者。
- [x] 心跳策略明确：第一期可每 15 秒发送 comment heartbeat。

**验收**

- [x] EventSource 能收到 `progress`。
- [x] completed 后连接自然结束。
- [x] 客户端断开不取消后端任务。

**Phase 2 验证命令**

```bash
cd apps/server && go test ./... -count=1
cd apps/server && go test -race -timeout 120s ./...
```

**Phase 2 当前验证记录（2026-05-15）**

- [x] `go test ./internal/service/admin_space_export_service.go ./internal/service/admin_space_import_service.go ./internal/service/admin_space_export_manifest.go ./internal/service/admin_access_service.go ./internal/service/admin_space_export_service_test.go ./internal/service/admin_space_import_service_test.go -count=1`
- [x] `npm run build -w @plaindoc/web`
- [ ] `go test ./... -count=1`：当前 Windows 环境缺少 `gcc`，`runtime/cgo` 无法编译，未完成全量验证。
- [ ] `go test -race -timeout 120s ./...`：同样受缺少 `gcc` 阻塞，未完成 race 验证。

**Phase 2 Review Gate**

- [ ] code review 确认任务 Store 无并发风险。
- [ ] code review 确认 token 不泄露长期凭据。
- [x] 更新本清单 Phase 2 checkbox。

---

## 5. Phase 3：可回灌 Zip 导出

**目标**: 先完成一个不含附件和 Office source 的最小可回灌 zip：空间元数据、tree、Markdown 文档、manifest。

### Task 3.1 manifest 与 tree 类型

**文件**

- 修改：`apps/server/internal/service/admin_space_export_manifest.go`

**步骤**

- [x] 定义 `AdminSpaceExportManifest`。
- [x] 定义 `AdminSpaceExportTree`。
- [x] 定义 `AdminSpaceExportDocumentEntry`。
- [x] 定义 `AdminSpaceExportSummary`。
- [x] 实现 manifest JSON marshal。
- [x] 实现 tree JSON marshal。
- [x] 增加 `packageType = "plaindoc-space"`。
- [x] 增加 `version = 1`。

**验收**

- [x] manifest 包含 `importable=true`。
- [x] manifest 中每个文档包含 `documentId`、`nodeId`、`parentNodeId`、`title`、`format`、`sort`、`visibility`、`path`。
- [x] tree 保留原始父子关系。

### Task 3.2 路径清洗与重名处理

**文件**

- 修改：`apps/server/internal/service/admin_space_export_manifest.go`
- 测试：`apps/server/internal/service/admin_space_export_service_test.go`

**步骤**

- [x] 实现 zip entry 名称清洗。
- [x] 禁止绝对路径。
- [x] 禁止 `..`。
- [x] 替换 Windows 非法字符。
- [x] 去除控制字符。
- [x] 空标题使用安全兜底名。
- [x] 同目录重名追加 ` (1)`、` (2)`。

**验收**

- [x] `../evil.md` 被拒绝或清洗为安全路径。
- [x] `C:\evil.md` 不会成为 zip entry。
- [x] 两个同名文档不会覆盖。

### Task 3.3 读取空间与目录树

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 可能修改：`apps/server/internal/storage/repository/interfaces.go`

**步骤**

- [x] 读取空间基础信息。
- [x] 确认空间未删除。
- [x] 读取 `WorkspaceRepository.ListTreeNodesBySpaceID`。
- [x] 构造内存树。
- [x] 按 sort 和 title 生成稳定顺序。
- [x] 统计 folder 和 doc 数量。

**验收**

- [x] 空空间也能导出 manifest/tree。
- [x] 删除空间不能导出。
- [x] tree 顺序稳定。

### Task 3.4 写入 Markdown 文档

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`

**步骤**

- [x] 遍历 doc 节点。
- [x] 对 `format=markdown` 的文档读取 `ContentMD`。
- [x] 写入 `documents/.../*.md`。
- [x] 计算内容 sha256。
- [x] 将文档 path 和 hash 写入 manifest。
- [x] 对 `docx/xlsx` 先只写 manifest source 占位，不写入 source 文件。

**验收**

- [x] Markdown 内容与数据库一致。
- [x] 文档标题和路径符合目录层级。
- [x] manifest 引用的 Markdown path 在 zip 中存在。

### Task 3.5 zip 原子写入

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`

**步骤**

- [x] 创建私有导出目录：`data/exports/admin-space` 或服务配置决定的目录。
- [x] 写入 `{jobId}.part`。
- [x] zip writer 正常关闭后 rename 为 `{jobId}.zip`。
- [x] 失败时删除 `.part`。
- [x] completed 事件写入 `fileName`、`sizeBytes`。

**验收**

- [x] 不会暴露半成品 zip。
- [x] 失败后没有残留 `.part`。
- [x] completed 事件能拿到 zip 大小。

**Phase 3 验证命令**

```bash
cd apps/server && go test ./... -count=1
```

**Phase 3 当前验证记录（2026-05-15）**

- [x] `go test ./internal/service/admin_space_export_service.go ./internal/service/admin_space_import_service.go ./internal/service/admin_space_export_manifest.go ./internal/service/admin_access_service.go ./internal/service/admin_space_export_service_test.go ./internal/service/admin_space_import_service_test.go -count=1`
- [ ] `go test ./... -count=1`：当前 Windows 环境缺少 `gcc`，`runtime/cgo` 无法编译，未完成全量验证。

**Phase 3 Review Gate**

- [ ] code review 确认 zip 结构可被导入解析。
- [ ] code review 确认路径安全。
- [x] 更新本清单 Phase 3 checkbox。

---

## 6. Phase 4：附件与 Office Source 导出

**目标**: 让 zip 空间交换包真正满足“原样导入”，包含普通附件和 `docx/xlsx` source blob。

### Task 4.1 附件导出

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 可能修改：`apps/server/internal/storage/repository/interfaces.go`

**步骤**

- [ ] 对每个文档调用 `DocumentAttachmentRepository.ListByDocumentID`。
- [ ] 跳过已删除附件。
- [ ] 读取附件 blob。
- [ ] 将附件写入 `attachments/{oldDocumentId}/filename.ext`。
- [ ] 附件路径写入 manifest。
- [ ] 附件数量写入 summary。

**验收**

- [ ] 附件文件存在于 zip。
- [ ] manifest 的附件路径和 zip entry 一致。
- [ ] 附件缺失时任务 failed，不生成 importable zip。

### Task 4.2 Office source 导出

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`

**步骤**

- [ ] 对 `docx/xlsx` 文档读取 `SourceBlobID`。
- [ ] 调用 `DocumentAttachmentRepository.GetBlobByBlobID`。
- [ ] 按 provider 读取物理文件或对象内容。
- [ ] 写入 `sources/{oldDocumentId}/source-file-name.docx` 或 `.xlsx`。
- [ ] source 路径写入 manifest。
- [ ] Office source 数量写入 summary。

**验收**

- [ ] `docx/xlsx` 文档都能在 zip 中找到 source 文件。
- [ ] 缺 source 的 Office 文档导致 zip `importable=false` 或任务失败；第一期按任务失败处理。
- [ ] source 文件名经过清洗。

### Task 4.3 完整性校验

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 测试：`apps/server/internal/service/admin_space_export_service_test.go`

**步骤**

- [ ] zip 关闭前校验 manifest 引用文件都已写入。
- [ ] 校验 `importable=true` 时附件/source 无缺失。
- [ ] 校验 summary 统计与实际写入数量一致。
- [ ] 导出完成前写入最终 manifest。

**验收**

- [ ] 破坏任意 manifest 引用时测试失败。
- [ ] summary 与实际 entry 数一致。
- [ ] 完整 zip 能通过 Phase 6 inspect。

**Phase 4 验证命令**

```bash
cd apps/server && go test ./... -count=1
```

**Phase 4 Review Gate**

- [ ] code review 确认附件和 source 文件读取没有越权路径风险。
- [ ] code review 确认 importable zip 所需内容完整。
- [ ] 更新本清单 Phase 4 checkbox。

---

## 7. Phase 5：EPUB 阅读包导出

**目标**: 在导出格式中支持 `epub`，复用现有 Markdown / Office HTML 渲染链路，生成阅读包，不参与导入。

### Task 5.1 依赖确认与引入

**文件**

- 修改：`apps/server/go.mod`
- 修改：`apps/server/go.sum`

**步骤**

- [ ] 发起依赖变更确认：新增 `github.com/go-shiori/go-epub`。
- [ ] 获得确认后，在 `apps/server` 下执行 Go 工具链添加依赖。
- [ ] 不手工编辑 `go.sum`。
- [ ] 记录实际引入版本。

**验收**

- [ ] `apps/server/go.mod` 出现 `github.com/go-shiori/go-epub`。
- [ ] `go mod tidy` 后无异常依赖漂移。
- [ ] 文档中记录的推荐库与实际依赖一致。

### Task 5.2 EPUB 章节生成

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 可选创建：`apps/server/internal/service/admin_space_epub_exporter.go`

**步骤**

- [ ] 实现空间标题页 XHTML。
- [ ] 实现 Markdown 文档 HTML/XHTML 转换。
- [ ] 调用现有 Markdown 渲染能力或提取共用渲染函数。
- [ ] 每个文档节点生成一个章节。
- [ ] 章节标题来自目录树节点标题。
- [ ] 章节顺序和目录树一致。

**验收**

- [ ] EPUB 包含标题页。
- [ ] Markdown 文档在 EPUB 中可读。
- [ ] 章节顺序稳定。

### Task 5.3 Office HTML 复用

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 复用：`apps/server/internal/service/office_html_render_service.go`

**步骤**

- [ ] 对 `docx/xlsx` 读取 source blob。
- [ ] 调用 `OfficeHTMLRenderService.RenderImportHTML`。
- [ ] 不新增高保真 Office 转换链路。
- [ ] 将返回 HTML 清洗为 EPUB XHTML。
- [ ] `xlsx` 宽表格按可读性降级处理。

**验收**

- [ ] `docx` 章节来自 Mammoth HTML。
- [ ] `xlsx` 章节来自 excelize table HTML。
- [ ] Office 宏、图表等复杂内容不作为高保真目标。

### Task 5.4 图片资源本地化

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 可选创建：`apps/server/internal/service/admin_space_epub_exporter.go`

**步骤**

- [ ] 扫描章节 HTML 中的 `<img src>`。
- [ ] 对站内图片、Office 渲染图片、附件图片尝试读取内容。
- [ ] 通过 `go-epub` 添加图片资源。
- [ ] 将 HTML 中 src 改写为 EPUB 内部相对路径。
- [ ] 无法读取的图片用 alt 文本或占位段落降级。

**验收**

- [ ] EPUB 不依赖需要鉴权的站内图片 URL。
- [ ] 图片路径全部指向 EPUB 内部资源。
- [ ] 图片缺失不会导致整个 EPUB 崩溃，除非该错误被定义为 fatal。

### Task 5.5 EPUB 打包与下载

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`

**步骤**

- [ ] 使用 `go-epub` 创建 EPUB。
- [ ] 添加基础 CSS。
- [ ] 添加章节。
- [ ] 添加图片。
- [ ] 写出 `.epub` 文件。
- [ ] completed 事件返回 `.epub` 文件名。
- [ ] 下载接口复用导出下载逻辑。

**验收**

- [ ] EPUB 文件能被常见阅读器打开。
- [ ] `format=epub` 不被导入 inspect 接受。
- [ ] `completed.fileName` 后缀为 `.epub`。

**Phase 5 验证命令**

```bash
cd apps/server && go test ./... -count=1
```

如本地装有 EPUBCheck：

```bash
epubcheck path/to/generated.epub
```

**Phase 5 Review Gate**

- [ ] code review 确认没有自造 EPUB 底层打包轮子。
- [ ] code review 确认 Office 只复用纯 HTML 渲染。
- [ ] 更新本清单 Phase 5 checkbox。

---

## 8. Phase 6：导入解析与预览

**目标**: 支持上传 zip 空间交换包，解析 manifest/tree，返回导入预览；不写入业务空间数据。

### Task 6.1 上传 staging

**文件**

- 修改：`apps/server/internal/server/handler/admin_space_import.go`
- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] 接收 multipart 字段 `file`。
- [ ] 上传 staging 前调用 `CanImportSpace(actorUserID)`。
- [ ] 只允许 `.zip` 或 zip content type。
- [ ] 限制上传大小。
- [ ] 写入 `data/imports/admin-space/{importId}.zip`。
- [ ] staging 绑定 `actorUserID`。
- [ ] 设置 staging 过期时间。

**验收**

- [ ] 空文件被拒绝。
- [ ] 非 zip 被拒绝。
- [ ] staging 文件不在公开目录。

### Task 6.2 manifest/tree 解析

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/server/internal/service/admin_space_export_manifest.go`

**步骤**

- [ ] 打开 zip。
- [ ] 查找 `manifest.json`。
- [ ] 查找 `tree.json`。
- [ ] 校验 `packageType == "plaindoc-space"`。
- [ ] 校验 `version == 1`。
- [ ] 校验 `importable == true`。
- [ ] 校验 manifest 文档引用的文件存在。
- [ ] 校验 tree 中 parent/children 关系无环。
- [ ] 校验 EPUB 不会被识别为空间交换包。

**验收**

- [ ] 缺 manifest 返回 `AdminSpaceImportErrManifestMissing`。
- [ ] 缺 tree 返回 `AdminSpaceImportErrTreeMissing`。
- [ ] EPUB 返回 package unsupported 或 zip invalid。

### Task 6.3 导入预览响应

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/server/internal/server/handler/admin_space_import.go`

**步骤**

- [ ] 返回原空间名称。
- [ ] 返回原空间 ID。
- [ ] 返回导出时间。
- [ ] 返回 folder/doc/attachment/source 统计。
- [ ] 返回 warnings。
- [ ] `importable=false` 时保留预览，但禁止 commit。

**验收**

- [ ] 前端可以展示完整预览。
- [ ] warnings 不包含敏感路径。
- [ ] importId 不泄露本机 staging 路径。

### Task 6.4 前端导入按钮与预览浮层

**文件**

- 修改：`apps/web/src/admin/pages/AdminSpacesPage.tsx`
- 创建：`apps/web/src/admin/components/AdminSpaceImportDialog.tsx`
- 修改：`apps/web/src/data-access/http/adapter.ts`

**步骤**

- [ ] 在空间管理上方工具区增加“导入空间”按钮。
- [ ] 打开 `AdminSpaceImportDialog`。
- [ ] 文件选择只接受 `.zip`。
- [ ] 选择文件后调用 `inspectSpaceImport`。
- [ ] 展示空间名称、导出时间、文档数、附件数、Office 源文件数。
- [ ] 展示 warnings。
- [ ] `importable=false` 时禁用确认导入。
- [ ] 允许用户输入新空间名称。

**验收**

- [ ] 导入入口位置清晰。
- [ ] 不能选择 EPUB 执行导入。
- [ ] 预览态不触发真实导入。

**Phase 6 验证命令**

```bash
npm run web:build
cd apps/server && go test ./... -count=1
```

**Phase 6 Review Gate**

- [ ] code review 确认 inspect 不写业务数据。
- [ ] code review 确认 zip 解析安全。
- [ ] 更新本清单 Phase 6 checkbox。

---

## 9. Phase 7：导入落地

**目标**: 将 inspect 通过的 zip 空间交换包导入为一个新空间，并恢复目录、文档、附件和 Office source。

### Task 7.1 创建新空间

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] commit 调用 `CanImportSpace(actorUserID)`，确认 actor 仍具备导入能力。
- [ ] 导入 worker 开始实际创建新空间前再次调用 `CanImportSpace(actorUserID)`。
- [ ] 校验 staging 存在且归属当前 actor。
- [ ] 校验 `importable=true`。
- [ ] 使用请求中的 `spaceName/spaceId/categoryId/visibility`。
- [ ] 调用现有空间创建逻辑。
- [ ] 建立 `oldSpaceId -> newSpaceId` 映射。

**验收**

- [ ] 导入永远创建新空间。
- [ ] 不复用原始 spaceId。
- [ ] 创建失败时任务 failed，staging 保留到过期。

### Task 7.2 恢复目录树

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] 对 tree 做拓扑排序。
- [ ] 先创建 folder 节点。
- [ ] 再创建 doc 节点。
- [ ] 建立 `oldNodeId -> newNodeId`。
- [ ] 保留 title、sort、type。
- [ ] parent 使用映射后的新 parent ID。

**验收**

- [ ] 目录层级与源空间一致。
- [ ] sort 顺序一致。
- [ ] 缺 parent 或环形 parent 关系无法导入。

### Task 7.3 恢复 Markdown 文档

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] 读取 manifest 中 Markdown 文档 path。
- [ ] 从 zip 读取 Markdown 内容。
- [ ] 校验 sha256。
- [ ] 创建 document。
- [ ] 创建初始 revision。
- [ ] 建立 `oldDocumentId -> newDocumentId`。
- [ ] 保留文档 visibility。

**验收**

- [ ] Markdown 内容与导出包一致。
- [ ] revision 创建成功。
- [ ] hash 不匹配时导入失败。

### Task 7.4 恢复 Office 文档

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] 对 `docx/xlsx` 读取 manifest source path。
- [ ] 从 zip 读取 source 文件。
- [ ] 复用现有 Office source blob 落地逻辑。
- [ ] 创建对应格式 document。
- [ ] 创建 file revision。
- [ ] 触发或排队 Office HTML 渲染。

**验收**

- [ ] 新空间中 Office 文档可打开。
- [ ] source blob 存在。
- [ ] Office HTML 渲染失败不影响导入完成，但需记录 warning 或后台日志。

### Task 7.5 恢复附件

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] 根据 manifest 附件列表读取 zip entry。
- [ ] 为新文档创建新 blob。
- [ ] 创建附件记录。
- [ ] 建立 `oldAttachmentId -> newAttachmentId`。
- [ ] 保留文件名、mime、size。

**验收**

- [ ] 附件数量与 manifest 一致。
- [ ] 附件属于新文档。
- [ ] 缺附件文件时导入失败。

### Task 7.6 失败回滚

**文件**

- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] 导入失败时尝试删除或软删除新空间。
- [ ] 清理已创建但未引用的临时 blob。
- [ ] 如果无法完整回滚，将新空间标记 deleted。
- [ ] failed 事件包含失败 stage。
- [ ] 写 warn 日志。

**验收**

- [ ] 失败后不会留下可见半成品空间。
- [ ] failed 事件前端可读。
- [ ] 日志不包含 token 或本机敏感路径。

**Phase 7 验证命令**

```bash
cd apps/server && go test ./... -count=1
```

**Phase 7 Review Gate**

- [ ] code review 确认导入 ID 映射正确。
- [ ] code review 确认失败回滚策略可接受。
- [ ] 更新本清单 Phase 7 checkbox。

---

## 10. Phase 8：下载、审计与清理

**目标**: 完善下载、审计、过期清理和前端完成态体验。

### Task 8.1 下载接口

**文件**

- 修改：`apps/server/internal/server/handler/admin_space_export.go`
- 修改：`apps/server/internal/service/admin_space_export_service.go`

**步骤**

- [ ] 实现 `GET /api/admin/space-exports/:jobId/download`。
- [ ] 校验 download token。
- [ ] 校验 job completed。
- [ ] 校验文件存在且未过期。
- [ ] 设置 `Content-Disposition: attachment`。
- [ ] 设置 `Cache-Control: no-store`。
- [ ] 设置 `Referrer-Policy: no-referrer`。
- [ ] 下载成功后将 download token 标记为已使用。
- [ ] 支持 zip 和 epub 两种扩展名。

**验收**

- [ ] 未完成 job 不能下载。
- [ ] 过期 token 不能下载。
- [ ] 已使用 token 不能重复下载。
- [ ] 文件名正确。

### Task 8.2 后台审计日志

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 修改：`apps/server/internal/service/admin_space_import_service.go`

**步骤**

- [ ] 导出创建记录 `space.export`。
- [ ] 导出完成记录 `space.export` success。
- [ ] 导出失败记录 `space.export` failed。
- [ ] 导入 commit 记录 `space.import`。
- [ ] 导入完成记录新空间 ID。
- [ ] 导入失败记录失败阶段。
- [ ] metadata 不写 token。
- [ ] metadata 记录 actor 当时命中的能力类型，例如 `space_manage` 或 `space_create`，不记录敏感配置值。

**验收**

- [ ] 审计记录能在后台审计页查到。
- [ ] 有空间管理权限的 actor 导出只记录其可管理空间。
- [ ] 导入审计 targetId 是新空间 ID。

### Task 8.3 过期清理

**文件**

- 修改：`apps/server/internal/service/admin_space_export_service.go`
- 修改：`apps/server/internal/service/admin_space_import_service.go`
- 修改：`apps/server/cmd/server/main.go` 或 `router.go` 初始化位置

**步骤**

- [ ] 清理过期导出 job。
- [ ] 删除过期导出 zip/epub 文件。
- [ ] 清理过期 import staging。
- [ ] 删除过期 staging zip。
- [ ] 清理循环有 context 退出路径。

**验收**

- [ ] 服务关闭时清理 goroutine 可退出。
- [ ] 清理任务不删除未过期文件。
- [ ] 清理错误写 warn 日志，不中断服务。

### Task 8.4 前端完成态

**文件**

- 修改：`apps/web/src/admin/components/AdminSpaceExportDialog.tsx`
- 修改：`apps/web/src/admin/components/AdminSpaceImportDialog.tsx`
- 修改：`apps/web/src/admin/pages/AdminSpacesPage.tsx`

**步骤**

- [ ] 导出 completed 后自动触发下载。
- [ ] 自动下载失败时保留手动下载按钮。
- [ ] EPUB completed 使用 `.epub` 文件名。
- [ ] 导入 completed 展示“打开编辑器”和“打开阅读页”。
- [ ] 导入 completed 后刷新空间列表。
- [ ] 关闭弹层时关闭 SSE。

**验收**

- [ ] 下载不会重复触发多次。
- [ ] 导入完成后列表出现新空间。
- [ ] 前端没有内存泄露警告。

**Phase 8 验证命令**

```bash
npm run web:build
cd apps/server && go test ./... -count=1
```

**Phase 8 Review Gate**

- [ ] code review 确认下载 token 和文件路径安全。
- [ ] code review 确认审计信息完整但不泄露敏感信息。
- [ ] 更新本清单 Phase 8 checkbox。

---

## 11. Phase 9：测试、构建与文档同步

**目标**: 补齐回归测试、执行项目验证命令，并同步主开发文档。

### Task 9.1 后端测试补齐

**文件**

- 修改或创建：`apps/server/internal/service/admin_space_export_service_test.go`
- 修改或创建：`apps/server/internal/service/admin_space_import_service_test.go`
- 修改或创建：`apps/server/internal/server/admin_space_export_handler_test.go`
- 修改或创建：`apps/server/internal/server/admin_space_import_handler_test.go`

**测试清单**

- [ ] 无目标空间管理权限的用户无法创建导出任务。
- [ ] `space_admin` 无 scope 无法导出空间。
- [ ] `platform_admin` 可导出空间。
- [ ] 普通用户具备创建空间能力时，也只能导出自己拥有或明确可管理的空间。
- [ ] 普通用户不能导出自己无管理权的空间。
- [ ] 不支持的导出格式返回错误。
- [ ] running 限流生效。
- [ ] zip entry 路径清洗防 zip slip。
- [ ] zip 包包含 manifest/tree/文档/附件/source。
- [ ] EPUB 包含 `mimetype`、`container.xml`、`content.opf`、`nav.xhtml`。
- [ ] EPUB 中 `docx/xlsx` 复用 Office HTML。
- [ ] inspect 非 PlainDoc zip 返回错误。
- [ ] inspect 缺 manifest/tree 返回错误。
- [ ] 有空间管理权限的用户可 inspect/commit 导入任务。
- [ ] 普通用户具备创建空间能力时可 inspect/commit 导入任务。
- [ ] 普通用户不具备创建空间能力时，inspect/commit 被拒绝。
- [ ] commit 时如果 actor 导入能力已被撤销，任务被拒绝。
- [ ] 导入后创建新空间，不复用原始 ID。
- [ ] 导入后目录树、文档、附件数量一致。
- [ ] 导入失败后不留下可见半成品空间。

### Task 9.2 前端测试补齐

**文件**

- 修改：`apps/web/src/admin/pages/AdminSpacesPage.test.tsx`
- 创建：`apps/web/src/admin/components/AdminSpaceExportDialog.test.tsx`
- 创建：`apps/web/src/admin/components/AdminSpaceImportDialog.test.tsx`

**测试清单**

- [ ] 空间操作菜单显示“导出空间”。
- [ ] 点击导出打开浮层。
- [ ] 导出格式可选择 zip / EPUB。
- [ ] 点击开始后调用 `startSpaceExport`。
- [ ] running 事件更新进度。
- [ ] completed 事件触发下载。
- [ ] failed 事件展示错误。
- [ ] 空间管理上方显示“导入空间”。
- [ ] 选择 zip 后调用 `inspectSpaceImport`。
- [ ] `importable=false` 禁用确认导入。
- [ ] commit 后订阅导入 SSE。
- [ ] 导入 completed 展示新空间入口。
- [ ] 关闭浮层会 close SSE。

### Task 9.3 文档同步

**文件**

- 修改：`docs/BACKEND_DEVELOPER_GUIDE.md`
- 修改：`docs/FRONTEND_DEVELOPER_GUIDE.md`
- 修改：`docs/2026-05-15_SPACE_EXPORT_TECHNICAL_PLAN.md`
- 修改：`docs/2026-05-15_SPACE_IMPORT_EXPORT_TASK_CHECKLIST.md`

**步骤**

- [ ] 后端文档补充导入导出 API、权限、SSE、临时文件目录。
- [ ] 前端文档补充 AdminGateway 方法、空间管理入口、SSE 订阅约束。
- [ ] 技术方案如有实现偏离，先确认后回写。
- [ ] 本清单所有已完成项回写 `[x]`。

### Task 9.4 最终验证

**命令**

```bash
cd apps/server && go test ./... -count=1
npm run web:build
npm run check:dropdown-menu -w @plaindoc/web
cd apps/server && go test -race -timeout 120s ./...
```

**验收**

- [ ] 后端测试通过。
- [ ] 前端构建通过。
- [ ] dropdown-menu 规则通过。
- [ ] race 测试通过，或记录明确失败原因和处理方案。
- [ ] 手工验证：导出 zip。
- [ ] 手工验证：导入 zip 到新空间。
- [ ] 手工验证：导出 EPUB。
- [ ] 手工验证：SSE 进度正常。

**Phase 9 Review Gate**

- [ ] 最终 code review。
- [ ] 文档状态可从 Draft 更新为 In Progress 或 Completed。
- [ ] 准备提交或继续下一轮实现。
