# 空间导入导出功能技术方案

**文档状态**: Draft  
**创建日期**: 2026-05-15  
**适用范围**: `apps/server`、`apps/web`、`docs`  
**目标**: 在管理后台空间管理中增加空间导出与导入能力。导出侧从空间操作菜单发起，后端异步生成可回灌的 zip 空间交换包，或生成用于离线阅读分发的 EPUB 文件，并通过 SSE 推送导出进度与下载链接；导入侧在空间管理上方提供“导入空间”按钮，选择 zip 后先解析元数据，再将导出包原样导入为一个新空间。

---

## 1. 方案结论

采用“空间交换包 + 后台任务 + SSE 进度订阅 + 短期文件链接”的实现方式。

导出核心流程如下：

```text
管理后台空间列表
  → 操作菜单点击“导出空间”
  → 打开导出浮层
  → 选择导出格式
  → 点击 zip 图标开始导出
  → POST 创建导出任务
  → 前端订阅 SSE 进度
  → 后端异步生成 zip
  → SSE 推送 completed + downloadUrl
  → 前端自动触发下载
```

本方案默认不让 `POST` 请求直接返回文件，也不让导出动作占用一个长时间 HTTP 请求。导出任务由后端异步执行，前端只负责创建任务、展示进度和拉起下载。

导入核心流程如下：

```text
管理后台空间列表上方
  → 点击“导入空间”
  → 选择 zip 空间交换包
  → POST 上传并解析 manifest/tree 元数据
  → 前端展示导入预览
  → 用户确认导入到新空间
  → POST 提交导入任务
  → 前端订阅 SSE 进度
  → 后端创建新空间并恢复目录、文档、附件和 Office 源文件
  → SSE 推送 completed + 新空间信息
```

本方案要求系统导出的 zip 包必须能被同版本或兼容版本的 PlainDoc 原样导入。导入时不复用原空间 ID、节点 ID、文档 ID、附件 ID，而是在新空间中生成新 ID，并通过导入过程中的 `oldID -> newID` 映射恢复引用关系。

---

## 2. 目标与非目标

### 2.1 目标

1. 在管理后台 `空间管理 -> 操作` 子菜单增加“导出空间”入口。
2. 点击入口后打开导出浮层，浮层初始态显示 zip 压缩包图标。
3. 点击 zip 图标后切换到导出进度视图。
4. 后端收到导出请求后，根据导出格式执行导出动作。
5. 导出进度通过 SSE 推送给前端。
6. 导出完成后，后端通过 SSE 推送下载链接。
7. 前端收到下载链接后自动拉起下载。
8. 导出文件支持 zip 空间交换包和 EPUB 阅读包。
9. 权限按能力判断，不按单一管理员角色判断：有目标空间管理权限的用户可导出该空间；具备创建空间能力的用户可导入 zip 创建新空间。
10. EPUB 导出复用现有 `docx/xlsx -> 纯 HTML` 阅读渲染链路，不追求 Office 高保真还原。
11. 在空间管理列表上方增加“导入空间”按钮，支持选择 zip 空间交换包。
12. 导入前必须解析 `manifest.json` 和 `tree.json`，展示空间名称、文档数、附件数、Office 源文件数和导出版本。
13. 用户确认后，系统将 zip 包导入为一个新空间，并原样恢复目录树、文档内容、附件和 Office 源文件。
14. 导入过程同样通过 SSE 推送进度，完成后返回新空间 ID 和可跳转入口。

### 2.2 非目标

本期不做以下能力：

1. 多空间批量导出。
2. 导出任务历史页面。
3. 导出任务手动取消。
4. 跨进程持久队列和分布式任务调度。
5. PDF、HTML 站点包、Confluence、Notion 等额外格式。
6. 导出后长期保存文件。
7. 对 Office 文档做高保真格式转换。
8. 覆盖导入到已有空间。
9. 导入时复用原始空间 ID 或原始数据主键。
10. 导入第三方系统生成的 zip 包。

---

## 3. 当前项目基础

### 3.1 前端入口基础

空间管理页面已存在操作列和子菜单，适合直接增加“导出空间”入口，也适合在列表上方工具区增加“导入空间”按钮：

1. `apps/web/src/admin/pages/AdminSpacesPage.tsx`
2. 当前操作菜单已使用项目封装的 `DropdownMenu`，并且 `modal={false}`，新增菜单项需要继续遵守该约束。
3. 页面已有空间设置、成员管理、转让、删除等操作入口，可复用当前 `AdminSpace` 数据作为导出浮层输入。
4. 页面顶部工具区已有查询、刷新、分类管理、新建空间等按钮，导入入口应放在“新建空间”附近，语义上属于空间级创建动作。

### 3.2 前端数据网关基础

前端后台接口契约集中在：

1. `apps/web/src/data-access/types.ts`
2. `apps/web/src/data-access/http/adapter.ts`

新增导入导出能力需要同步补充 `AdminGateway` 类型和 HTTP adapter 实现，页面层不能直接散写业务 `fetch`。

### 3.3 后端路由与权限基础

后台空间治理路由集中在：

1. `apps/server/internal/server/router.go`
2. `apps/server/internal/server/handler/admin_space.go`
3. `apps/server/internal/service/admin_space_service.go`

当前仓库已有部分后台权限基础：

1. `middleware.RequireAdminSession`
2. `middleware.RequireSpaceManagement(adminAccessService, "spaceId")`
3. `AdminAccessService.CanManageSpace`

导入导出需要在这些基础上新增能力判定，不能只按 `platform_admin` / `space_admin` 写死：

1. `CanExportSpace(ctx, actorUserID, spaceID)`
   - `platform_admin` 可导出任意正常空间。
   - `space_admin` 仅可导出自己 scope 内可管理的空间。
   - 普通用户如果具备创建空间能力，也只能导出自己拥有或明确可管理的空间，不能导出任意空间。
2. `CanImportSpace(ctx, actorUserID)`
   - 有空间管理权限的用户可以导入。
   - 普通用户如果当前系统允许其创建空间，也可以导入 zip 并创建新空间。
   - 如果系统配置不允许普通用户创建空间，则普通用户不能导入。

导出入口使用 `RequireAdminSession` 取得 actor，再由 service 调用 `CanExportSpace`。导入 inspect 和 commit 都使用 `RequireAdminSession` 取得 actor，再由 service 调用 `CanImportSpace`。不要把导入限制为单一平台管理员角色，也不要让“能创建空间”的普通用户借导入覆盖或写入已有空间。

### 3.4 数据读取基础

空间导出可复用以下仓储能力：

1. `SpaceRepository.GetBySpaceID`：读取空间基础信息。
2. `WorkspaceRepository.ListTreeNodesBySpaceID`：读取目录树。
3. `WorkspaceRepository.GetDocumentByDocumentID`：读取文档内容和 Office 源文件引用。
4. `DocumentAttachmentRepository.ListByDocumentID`：读取文档附件。
5. `DocumentAttachmentRepository.GetBlobByBlobID`：读取 Office 源文件 blob。

EPUB 导出可复用现有 Office 本地 HTML 渲染能力：

1. `apps/server/internal/service/office_html_render_service.go`
2. `OfficeHTMLRenderService.RenderImportHTML`
3. `docx` 通过 Mammoth 转纯 HTML
4. `xlsx` 通过 excelize 渲染为多 tab table

EPUB 不新建 Office 高保真渲染链路，只把现有阅读/分享页使用的纯 HTML 结果清洗为 EPUB 兼容 XHTML。

空间导入可复用以下写入能力：

1. `AdminSpaceService.CreateSpace`：创建新空间。
2. `WorkspaceRepository.CreateNode`：恢复目录节点和文档节点。
3. `WorkspaceRepository.SaveOfficeDocument` 或现有 Office source blob 落地逻辑：恢复 `docx/xlsx` 源文件。
4. `DocumentAttachmentRepository.Create` 和 `CreateBlob`：恢复附件与物理文件 blob。
5. 现有图片/附件本地与对象存储配置：决定导入文件落地位置。

---

## 4. 导出格式与空间交换包

### 4.1 第一期支持格式

第一期支持三个导出 profile：

1. `markdown_zip`
   - 导出目录树、Markdown 文档、附件。
   - Office 文档不转换为 Markdown，必须同时导出源文件，否则无法原样导入。
2. `source_zip`
   - 偏完整备份。
   - 导出目录树、Markdown 文档、Office 源文件、附件和 manifest。
3. `epub`
   - 阅读分发格式。
   - 导出空间标题页、目录、Markdown 文档章节和 Office 文档纯 HTML 章节。
   - 不作为空间交换包，不支持原样导入。

为了满足“导出的 zip 包可原样导入到新空间”的要求，系统生成的 zip 包必须包含：

1. `manifest.json`
2. `tree.json`
3. 所有 Markdown 文档正文文件
4. 所有普通附件文件
5. 所有 `docx/xlsx` 文档的 source 文件

如果用户显式关闭附件或 Office 源文件导出，该 zip 仍可被解析，但导入预览必须提示“非完整交换包”，并禁止执行“原样导入”。第一期推荐不暴露关闭选项，默认导出完整交换包。

EPUB 不参与上述原样导入约束。EPUB 是阅读产物，允许为了阅读器兼容性做语义级降级。

### 4.2 格式选项

请求参数：

```json
{
  "format": "markdown_zip",
  "includeAttachments": true,
  "includeOfficeSources": true
}
```

字段说明：

1. `format`
   - 必填。
   - 可选值：`markdown_zip`、`source_zip`、`epub`。
2. `includeAttachments`
   - 默认 `true`。
   - 控制是否导出普通文档附件。
3. `includeOfficeSources`
   - 默认 `true`。
   - 控制是否导出 `docx/xlsx` 的源文件。
   - 若为 `false`，该 zip 不能执行原样导入。
   - `epub` 格式忽略该字段。

### 4.3 EPUB 导出规则

EPUB 导出遵循“可读优先、语义降级、不做高保真”的原则。

导出内容：

1. 空间标题页。
2. 基于目录树生成的 EPUB 目录。
3. Markdown 文档章节。
4. `docx` 文档章节：复用 Mammoth 生成的纯 HTML。
5. `xlsx` 文档章节：复用 excelize 生成的多 sheet HTML table。
6. 图片资源：尽量打包进 EPUB，并改写为相对路径。
7. 附件清单：普通附件不直接作为阅读章节，只生成附件列表。

明确不做：

1. EPUB 反向导入。
2. Word 像素级版式还原。
3. Excel 高保真布局还原。
4. Office 宏、批注、修订记录、复杂图表还原。
5. 依赖 JavaScript 的交互内容。
6. 复杂 Mermaid/KaTeX 的高保真渲染；第一期按现有 HTML 能力降级。

EPUB 文件名：

```text
space-{spaceId}-{yyyyMMddHHmmss}.epub
```

EPUB 包结构：

```text
mimetype
META-INF/
└── container.xml
OEBPS/
├── content.opf
├── nav.xhtml
├── styles.css
├── chapters/
│   ├── title.xhtml
│   ├── 001-需求说明.xhtml
│   └── 002-预算表.xhtml
└── assets/
    └── ...
```

EPUB XHTML 生成规则：

1. Markdown 先转 HTML，再清洗为 EPUB 兼容 XHTML。
2. Office 文档调用现有 `OfficeHTMLRenderService.RenderImportHTML` 生成纯 HTML。
3. 清洗阶段移除脚本、事件属性和阅读器不支持的危险属性。
4. 将 `<img src>` 改写为 EPUB 内部相对路径。
5. 宽表格允许横向滚动样式降级，但不追求 Excel 原样宽度。
6. 每个文档节点独立生成一个章节，目录层级来自 `tree.json` 或导出时内存目录树。

### 4.4 EPUB 第三方库选择

EPUB 打包不手写底层规范，第一期推荐引入 Go 包：

```text
github.com/go-shiori/go-epub
```

选择理由：

1. 专门用于创建 EPUB 3.0。
2. API 简单，适合“已有 XHTML 内容，组装成 EPUB”的场景。
3. 支持添加章节、CSS、图片、封面并写出 `.epub` 文件。
4. 生成 EPUB 2.0 TOC，阅读器兼容性更好。
5. MIT License，依赖风险较低。

使用边界：

1. `go-epub` 负责 EPUB 包结构、OPF、目录、资源引用和最终写出。
2. PlainDoc 自己负责 Markdown/Office HTML 渲染、XHTML 清洗、图片资源本地化和路径改写。
3. `go-epub` 不负责校验输入 XHTML 合法性，因此仍需要服务端清洗与测试。
4. 实施时会新增 Go 依赖，修改 `apps/server/go.mod` 和 `apps/server/go.sum`，必须按依赖变更流程先确认后执行。

可选校验工具：

1. 本地或 CI 可选接入 `epubcheck` 对生成文件做规范校验。
2. `epubcheck` 不作为第一期运行时依赖，仅作为测试/验收工具。

---

## 5. Zip 目录结构与导入约束

导出文件名：

```text
space-{spaceId}-{yyyyMMddHHmmss}.zip
```

zip 内部结构：

```text
space-{spaceId}/
├── manifest.json
├── tree.json
├── documents/
│   ├── README.md
│   └── 产品文档/
│       └── 需求说明.md
├── attachments/
│   └── {documentId}/
│       └── 附件文件名.ext
└── sources/
    └── {documentId}/
        └── 原始文件.docx
```

### 5.1 `manifest.json`

`manifest.json` 用于描述导出包元信息，支撑导入预览、原样恢复和排障。

```json
{
  "version": 1,
  "packageType": "plaindoc-space",
  "exportedAt": "2026-05-15T12:00:00+08:00",
  "format": "markdown_zip",
  "importable": true,
  "space": {
    "spaceId": "space-demo",
    "name": "示例空间",
    "visibility": "member"
  },
  "summary": {
    "folderCount": 5,
    "documentCount": 20,
    "attachmentCount": 8,
    "officeSourceCount": 2
  },
  "documents": [
    {
      "documentId": "doc-demo",
      "nodeId": "node-demo",
      "parentNodeId": "folder-demo",
      "title": "需求说明",
      "format": "markdown",
      "sort": 10,
      "visibility": "member",
      "path": "documents/产品文档/需求说明.md",
      "contentSha256": "e3b0c44298fc1c149afbf4c8996fb924...",
      "attachments": [
        "attachments/doc-demo/design.png"
      ],
      "source": null
    }
  ]
}
```

### 5.2 `tree.json`

`tree.json` 保存原始目录树和节点关系。即使导出的 Markdown 文件名经过清洗，也能通过该文件恢复原始结构。

示例：

```json
{
  "version": 1,
  "root": [
    {
      "nodeId": "folder-demo",
      "parentNodeId": null,
      "type": "folder",
      "title": "产品文档",
      "sort": 10,
      "children": [
        {
          "nodeId": "node-demo",
          "documentId": "doc-demo",
          "parentNodeId": "folder-demo",
          "type": "doc",
          "title": "需求说明",
          "sort": 10,
          "format": "markdown"
        }
      ]
    }
  ]
}
```

### 5.3 导入 ID 映射规则

导入永远创建新空间，不复用导出包中的原始 ID。

导入过程中必须维护以下映射：

1. `oldSpaceId -> newSpaceId`
2. `oldNodeId -> newNodeId`
3. `oldDocumentId -> newDocumentId`
4. `oldAttachmentId -> newAttachmentId`
5. `oldBlobId -> newBlobId`

恢复规则：

1. 先创建新空间。
2. 再按目录树拓扑顺序创建 folder 节点。
3. 再创建 doc 节点和对应 document。
4. 最后恢复附件和 Office source blob。
5. 引用关系只通过映射表回填，不从文件名反推。

导入完成后，新空间内容应和原空间在用户可见层面一致，包括：

1. 目录层级。
2. 节点标题。
3. 节点排序。
4. 文档格式。
5. Markdown 正文。
6. Office 源文件。
7. 普通附件。
8. 文档可见性。

### 5.4 路径清洗规则

所有写入 zip 的路径必须经过统一清洗：

1. 禁止绝对路径。
2. 禁止 `..` 路径片段。
3. 禁止空文件名。
4. 替换 Windows 非法字符：`< > : " / \ | ? *`。
5. 去除控制字符。
6. 同目录重名时追加递增后缀，例如 `需求说明 (1).md`。

这部分必须集中在一个 helper 中实现，避免不同导出分支各自处理。

导入读取 zip 时也必须复用同一套安全校验，禁止 zip slip 和异常路径覆盖服务端文件。

---

## 6. 后端 API 设计

### 6.1 创建导出任务

```http
POST /api/admin/spaces/:spaceId/exports
Content-Type: application/json
Authorization: Bearer <access-token>
```

请求体：

```json
{
  "format": "markdown_zip",
  "includeAttachments": true,
  "includeOfficeSources": false
}
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "requestId": "req-demo",
  "data": {
    "jobId": "01hspaceexportjob0000000001",
    "streamUrl": "/api/admin/spaces/space-demo/exports/01hspaceexportjob0000000001/events?token=***"
  }
}
```

权限：

1. 必须登录。
2. 必须通过 `CanExportSpace(actorUserID, spaceId)`。
3. 普通用户即使具备创建空间能力，也只能导出自己拥有或明确可管理的空间。
4. 不要求 operation token。导出是敏感读取，但不是破坏性写操作；后续如果要将导出文件长期留存或开放给其他用户，再升级为 operation token。

当 `format=epub` 时，导出任务返回同样的 `jobId/streamUrl`，完成事件中的 `fileName` 使用 `.epub` 后缀，`downloadUrl` 指向同一个下载接口。

### 6.2 订阅导出进度

```http
GET /api/admin/spaces/:spaceId/exports/:jobId/events?token=***
Accept: text/event-stream
```

说明：

1. 这里使用 SSE。
2. 不使用 `Authorization` header 作为唯一凭据，因为浏览器原生 `EventSource` 不能设置自定义 header。
3. `token` 为短期导出流 token，绑定 `actorUserId + spaceId + jobId`，默认 10 分钟有效。
4. 若前端改用 `fetch` 读取 SSE stream，可保留 header 鉴权；但第一期优先支持原生 `EventSource`。

响应头：

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

### 6.3 下载导出文件

```http
GET /api/admin/space-exports/:jobId/download?token=***
```

说明：

1. 下载 token 由 completed 事件下发。
2. token 绑定 `actorUserId + jobId`，默认 10 分钟有效。
3. 下载 token 单次使用，下载成功、任务过期或 token 过期后都不可再次使用。
4. 响应使用 `Content-Disposition: attachment`。
5. 响应设置 `Cache-Control: no-store` 和 `Referrer-Policy: no-referrer`。
6. zip 文件只从服务端私有目录读取，不放到公开 `/uploads` 目录。

### 6.4 解析空间导入包

```http
POST /api/admin/space-imports/inspect
Content-Type: multipart/form-data
Authorization: Bearer <access-token>
```

表单字段：

1. `file`: zip 空间交换包。

响应：

```json
{
  "code": 0,
  "message": "ok",
  "requestId": "req-demo",
  "data": {
    "importId": "01hspaceimport0000000001",
    "packageVersion": 1,
    "packageType": "plaindoc-space",
    "importable": true,
    "space": {
      "spaceId": "space-demo",
      "name": "示例空间",
      "visibility": "member"
    },
    "summary": {
      "folderCount": 5,
      "documentCount": 20,
      "attachmentCount": 8,
      "officeSourceCount": 2
    },
    "warnings": []
  }
}
```

说明：

1. inspect 只解析元数据，不写入业务空间数据。
2. 后端将上传 zip 暂存到私有 staging 目录，并绑定 `importId + actorUserId`。
3. 必须校验 `manifest.json`、`tree.json`、必要文件是否存在。
4. 若缺少附件或 Office source，返回 `importable=false` 和 warnings。
5. inspect 必须通过 `CanImportSpace(actorUserID)`，避免无创建空间能力的用户滥用 staging 存储或探测导入包结构。

### 6.5 提交导入到新空间

```http
POST /api/admin/space-imports/:importId/commit
Content-Type: application/json
Authorization: Bearer <access-token>
```

请求体：

```json
{
  "spaceName": "示例空间",
  "spaceId": "",
  "categoryId": "",
  "visibility": "member"
}
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "requestId": "req-demo",
  "data": {
    "jobId": "01hspaceimportjob00000001",
    "streamUrl": "/api/admin/space-imports/01hspaceimportjob00000001/events?token=***"
  }
}
```

说明：

1. `spaceName` 默认取 manifest 中的原空间名称，若重名由后端按现有规则处理或由前端提示用户修改。
2. `spaceId` 可为空，默认由后端生成。
3. 导入永远创建新空间，不覆盖已有空间。
4. commit 必须通过 `CanImportSpace(actorUserID)`；有空间管理权限的用户可导入，普通用户仅在系统允许其创建空间时可导入。

### 6.6 订阅导入进度

```http
GET /api/admin/space-imports/:jobId/events?token=***
Accept: text/event-stream
```

导入 completed 事件返回：

```json
{
  "type": "completed",
  "stage": "done",
  "progress": 100,
  "message": "导入完成",
  "spaceId": "new-space-id",
  "spaceName": "示例空间",
  "editorUrl": "/editor/new-space-id",
  "readerUrl": "/r/new-space-id"
}
```

---

## 7. SSE 协议

### 7.1 事件类型

统一事件结构：

```json
{
  "type": "running",
  "stage": "documents",
  "progress": 45,
  "message": "正在导出文档 9/20",
  "downloadUrl": "",
  "fileName": "",
  "sizeBytes": 0,
  "spaceId": "",
  "editorUrl": "",
  "readerUrl": ""
}
```

事件类型：

1. `queued`
   - 任务已创建，等待执行。
2. `running`
   - 任务执行中。
3. `completed`
   - 任务完成，包含 `downloadUrl`。
4. `failed`
   - 任务失败，包含失败阶段和错误信息。

导出任务的 `completed` 事件包含 `downloadUrl`；导入任务的 `completed` 事件包含新空间的 `spaceId/editorUrl/readerUrl`。

### 7.2 事件示例

```text
event: progress
data: {"type":"queued","stage":"queued","progress":0,"message":"导出任务已创建"}

event: progress
data: {"type":"running","stage":"tree","progress":10,"message":"正在读取空间目录树"}

event: progress
data: {"type":"running","stage":"documents","progress":50,"message":"正在写入文档 10/20"}

event: completed
data: {"type":"completed","stage":"done","progress":100,"message":"导出完成","downloadUrl":"/api/admin/space-exports/01h.../download?token=***","fileName":"space-demo-20260515120000.zip","sizeBytes":1048576}
```

失败示例：

```text
event: failed
data: {"type":"failed","stage":"attachments","progress":62,"message":"附件读取失败：文件不存在"}
```

### 7.3 进度计算

进度采用阶段权重：

1. 初始化与权限复核：0% - 5%
2. 读取空间与目录树：5% - 15%
3. 写入文档：15% - 60%
4. 写入附件：60% - 85%
5. 写入 Office 源文件：85% - 95%
6. 收尾、校验、生成下载链接：95% - 100%

如果某阶段无数据，直接跳过并把权重合并到下一阶段。

EPUB 导出进度复用导出任务事件类型，阶段改为：

1. 读取空间与目录树：5% - 15%
2. 渲染 Markdown/Office HTML：15% - 65%
3. 本地化图片资源：65% - 85%
4. 生成 OPF/nav/XHTML：85% - 95%
5. 打包 EPUB 并生成下载链接：95% - 100%

导入进度采用阶段权重：

1. 初始化与权限校验：0% - 5%
2. 校验 staging zip 与 manifest：5% - 15%
3. 创建新空间：15% - 25%
4. 恢复目录树：25% - 45%
5. 恢复文档：45% - 70%
6. 恢复附件与 Office 源文件：70% - 95%
7. 收尾、审计、返回新空间入口：95% - 100%

---

## 8. 后端服务设计

### 8.1 新增服务

新增文件建议：

1. `apps/server/internal/service/admin_space_export_service.go`
2. `apps/server/internal/service/admin_space_import_service.go`
3. `apps/server/internal/server/handler/admin_space_export.go`
4. `apps/server/internal/server/handler/admin_space_import.go`
5. `apps/server/internal/server/response/admin_space_export.go`
6. `apps/server/internal/server/response/admin_space_import.go`

服务结构：

```go
type AdminSpaceExportService struct {
    spaceRepo              repository.SpaceRepository
    workspaceRepo          repository.WorkspaceRepository
    documentAttachmentRepo repository.DocumentAttachmentRepository
    adminAccessService     *AdminAccessService
    jobStore               *AdminSpaceExportJobStore
    exportRootDir          string
}
```

导入服务结构：

```go
type AdminSpaceImportService struct {
    adminSpaceService      *AdminSpaceService
    workspaceRepo          repository.WorkspaceRepository
    documentAttachmentRepo repository.DocumentAttachmentRepository
    adminAccessService     *AdminAccessService
    jobStore               *AdminSpaceImportJobStore
    stagingRootDir         string
}
```

### 8.2 任务状态

第一期导入导出任务都使用进程内任务表，不新增数据库表。

原因：

1. 空间导入导出是短生命周期后台操作。
2. 第一阶段不提供历史任务页面。
3. 不引入跨进程任务恢复，能显著降低复杂度。

任务状态结构：

```go
type AdminSpaceExportJob struct {
    JobID         string
    ActorUserID   string
    SpaceID       string
    Format        AdminSpaceExportFormat
    Status        AdminSpaceExportStatus
    Progress      int
    Stage         string
    Message       string
    FilePath      string
    FileName      string
    SizeBytes     int64
    ErrorMessage  string
    CreatedAt     time.Time
    ExpiresAt     time.Time
}
```

导入 inspect 暂存记录：

```go
type AdminSpaceImportStaging struct {
    ImportID       string
    ActorUserID    string
    FilePath       string
    Manifest       AdminSpaceExportManifest
    Tree           AdminSpaceExportTree
    Importable     bool
    Warnings       []string
    CreatedAt      time.Time
    ExpiresAt      time.Time
}
```

### 8.3 并发与限流

第一期限制：

1. 同一 `actorUserId + spaceId` 同时最多 1 个 running 导出任务。
2. 同一 `actorUserId` 同时最多 1 个 running 导入任务。
3. 全局同时最多 2 个 running 导出任务，最多 1 个 running 导入任务。
4. 超出限制时返回明确错误：`当前已有导入或导出任务正在执行，请稍后再试`。

实现方式：

1. `jobStore` 内部使用 `sync.Mutex` 保护任务表和订阅者列表。
2. 每个任务通过 `context.Context` 控制生命周期。
3. 后台 worker 必须有明确退出路径，不允许裸 `go func()` 无管理地长期运行。

---

## 9. 导出执行流程

### 9.1 创建任务

1. handler 读取 `spaceId`。
2. handler 解析请求体并校验 `format`。
3. service 调用 `CanExportSpace(actorUserID, spaceID)` 确认当前用户可导出该空间。
4. service 创建 `jobId`。
5. service 注册任务并推送 `queued` 事件。
6. service 启动受控后台任务。
7. handler 返回 `jobId` 和 `streamUrl`。

### 9.2 执行任务

后台任务步骤：

1. 再次读取空间，确认空间存在且未删除。
2. 再次调用 `CanExportSpace(actorUserID, spaceID)` 复核权限，避免任务创建后权限被撤销仍继续导出。
3. 读取目录树。
4. 根据目录树构造导出路径映射。
5. 创建临时 zip 文件，例如：

```text
data/exports/admin-space/{jobId}.part
```

6. 写入 `manifest.json` 和 `tree.json`。
7. 遍历文档节点：
   - Markdown 文档写入 `.md`。
   - Office 文档写入 manifest，若开启源文件导出则写入 `sources/`。
8. 若开启附件导出，按文档遍历附件并写入 `attachments/`。
9. zip writer 正常关闭后，将 `.part` 原子重命名为 `.zip`。
10. 生成下载 token 和 `downloadUrl`。
11. 推送 `completed` 事件。

EPUB 执行任务步骤：

1. 再次读取空间，确认空间存在且未删除。
2. 再次调用 `CanExportSpace(actorUserID, spaceID)` 复核权限。
3. 读取目录树和文档记录。
4. 创建 EPUB 临时目录。
5. 生成标题页和 `nav.xhtml`。
6. 遍历文档节点：
   - Markdown 文档转 HTML/XHTML 章节。
   - `docx/xlsx` 读取 source blob，调用现有 Office HTML 渲染服务生成纯 HTML，再清洗为 XHTML 章节。
7. 收集并本地化章节图片资源。
8. 生成 `content.opf`、`styles.css`、`META-INF/container.xml`。
9. 按 EPUB 要求打包，`mimetype` 必须作为 zip 第一个 entry 且不压缩。
10. 生成下载 token 和 `downloadUrl`。
11. 推送 `completed` 事件。

### 9.3 失败处理

失败规则：

1. 权限失败：任务不创建，直接返回错误。
2. 数据读取失败：任务进入 `failed`，SSE 推送失败阶段。
3. 单个附件缺失：
   - 第一期建议视为任务失败，避免导出包不完整。
   - 后续如需要“部分成功”，可在 manifest 中增加 `warnings`。
4. zip 写入失败：删除 `.part` 文件，任务进入 `failed`。
5. SSE 断开：
   - 不取消后端任务。
   - 用户重新打开浮层时，第一期不恢复旧任务；后续可增加任务查询接口。

---

## 10. 导入执行流程

### 10.1 解析导入包

1. handler 接收 multipart zip 文件。
2. 调用 `CanImportSpace(actorUserID)`，确认当前用户具备创建新空间的能力。
3. 将 zip 写入 staging 私有目录，例如：

```text
data/imports/admin-space/{importId}.zip
```

4. 打开 zip，查找根目录下的 `manifest.json` 和 `tree.json`。
5. 校验 `packageType == "plaindoc-space"`。
6. 校验 `version` 是否在兼容范围内。
7. 校验 manifest 引用的文档、附件和 source 文件是否存在。
8. 返回导入预览。

### 10.2 提交导入任务

1. handler 读取 `importId`。
2. service 校验 staging 记录存在、未过期、归属当前 actor。
3. 再次调用 `CanImportSpace(actorUserID)`，确认当前用户仍具备创建新空间的能力。
4. service 校验 `importable=true`。
5. service 创建导入 job 并推送 `queued` 事件。
6. service 启动受控后台任务。
7. handler 返回 `jobId` 和 `streamUrl`。

### 10.3 执行导入任务

后台任务步骤：

1. 再次调用 `CanImportSpace(actorUserID)`，避免任务创建后创建空间能力被撤销仍继续导入。
2. 重新打开 staging zip。
3. 创建新空间。
4. 建立 `oldSpaceId -> newSpaceId` 映射。
5. 按 `tree.json` 拓扑顺序创建 folder 节点。
6. 按文档节点创建 document：
   - `markdown`：读取 manifest 中的 `path`，创建 Markdown 文档。
   - `docx/xlsx`：读取 `sources/{oldDocumentId}/...`，创建 Office 文档并保存 source blob。
7. 建立 `oldDocumentId -> newDocumentId` 映射。
8. 导入附件：
   - 读取 `attachments/{oldDocumentId}/...`。
   - 为新文档创建新附件和新 blob。
9. 回写空间更新时间。
10. 写审计日志。
11. 推送 `completed` 事件，返回新空间入口。

### 10.4 导入失败处理

失败规则：

1. inspect 阶段发现 zip 结构非法：不创建 staging 记录，直接返回错误。
2. commit 阶段发现 staging 过期：返回过期错误。
3. 导入执行中失败：任务进入 `failed`，并尽量回滚已创建的新空间。
4. 如果无法完整回滚，必须将新空间标记为 deleted 或导入失败状态，并写入审计日志。
5. 导入完成后删除 staging zip。
6. 导入失败后保留 staging zip 到过期时间，便于用户重试或排查。

---

## 11. 前端设计

### 11.1 入口

在 `AdminSpacesPage.tsx` 增加两个入口。

空间列表上方工具区增加：

```text
导入空间
```

导入按钮放在“新建空间”左侧或右侧，语义上和空间创建同级。

空间操作菜单中增加：

```text
导出空间
```

入口位置建议放在：

1. `空间设置` 之后
2. `封禁空间` 之前

导出是读取型治理动作，视觉语义不应和删除、封禁放在同一危险区。

### 11.2 新增组件

新增文件：

```text
apps/web/src/admin/components/AdminSpaceExportDialog.tsx
apps/web/src/admin/components/AdminSpaceImportDialog.tsx
```

组件 props：

```ts
interface AdminSpaceExportDialogProps {
  open: boolean;
  space: AdminSpace | null;
  dataGateway: DataGateway;
  onOpenChange: (open: boolean) => void;
}
```

导入组件 props：

```ts
interface AdminSpaceImportDialogProps {
  open: boolean;
  dataGateway: DataGateway;
  onOpenChange: (open: boolean) => void;
  onImported: (spaceId: string) => void;
}
```

### 11.3 导出浮层状态

状态机：

```text
idle
  → starting
  → running
  → completed
  → failed
```

说明：

1. `idle`
   - 展示 zip 图标。
   - 展示格式选择。
   - 点击 zip 图标开始导出。
2. `starting`
   - 正在创建任务。
3. `running`
   - 展示进度条、阶段文案。
4. `completed`
   - 自动下载。
   - 展示手动下载链接。
5. `failed`
   - 展示错误信息。
   - 允许重新导出。

当导出格式为 `epub` 时，浮层文案需要把“zip 压缩包”切换为“EPUB 阅读包”。入口仍可复用同一导出浮层，但图标和确认按钮应根据格式变化。

### 11.4 导入浮层状态

状态机：

```text
idle
  → inspecting
  → preview
  → starting
  → running
  → completed
  → failed
```

说明：

1. `idle`
   - 展示文件选择区。
   - 只接受 `.zip`。
2. `inspecting`
   - 上传 zip 并解析元数据。
3. `preview`
   - 展示空间名称、导出时间、文档数、附件数、Office 源文件数、warnings。
   - 允许修改新空间名称、空间 ID、分类和可见性。
4. `starting`
   - 正在提交导入任务。
5. `running`
   - 展示导入进度。
6. `completed`
   - 展示新空间入口。
7. `failed`
   - 展示失败原因。
   - 允许重新选择 zip。

### 11.5 自动下载

收到 `completed` 事件后：

1. 创建隐藏 `<a>`。
2. 设置 `href = downloadUrl`。
3. 设置 `download = fileName`。
4. 调用 `click()`。
5. 保留手动下载按钮，避免浏览器拦截自动下载时用户无路可走。

### 11.6 SSE 封装

建议在 adapter 中新增：

```ts
startSpaceExport(input): Promise<AdminSpaceExportStartResult>
subscribeSpaceExport(input): AdminSpaceExportSubscription
inspectSpaceImport(input): Promise<AdminSpaceImportInspectResult>
commitSpaceImport(input): Promise<AdminSpaceImportStartResult>
subscribeSpaceImport(input): AdminSpaceImportSubscription
```

`subscribeSpaceExport` 可以返回一个轻量对象：

```ts
interface AdminSpaceExportSubscription {
  close(): void;
}
```

页面组件只消费回调，不直接拼接 SSE URL。

---

## 12. 安全设计

### 12.1 权限

1. 创建导出任务必须走后台登录态。
2. 创建导出任务必须通过 `CanExportSpace(actorUserID, spaceID)`。
3. 下载链接必须绑定创建任务的用户。
4. SSE token 必须绑定创建任务的用户、空间和任务。
5. inspect 导入包必须通过 `CanImportSpace(actorUserID)`。
6. commit 导入任务必须通过 `CanImportSpace(actorUserID)`。
7. 导入任务只能消费当前 actor 自己上传的 staging zip。
8. 普通用户如果具备创建空间能力，可以导入 zip 创建新空间；如果不具备创建空间能力，不能导入。
9. 普通用户如果具备创建空间能力，也不能导出任意空间，只能导出自己拥有或明确可管理的空间。

### 12.2 Token

第一期新增三类短期 token：

1. `streamToken`
   - 用于 SSE 订阅。
   - 默认 10 分钟有效。
2. `downloadToken`
   - 用于下载 zip。
   - 默认 10 分钟有效。
3. `importStreamToken`
   - 用于订阅导入进度。
   - 默认 10 分钟有效。

token 内容不要包含明文敏感信息，使用服务端签名校验；如果服务端需要记录 token 状态，只保存哈希或不可逆摘要。日志、审计 metadata、错误消息中都不能打印 query token。`downloadToken` 必须单次使用，下载接口响应设置 `Cache-Control: no-store` 和 `Referrer-Policy: no-referrer`，降低 query token 被缓存或被 Referer 带出的风险。

### 12.3 文件安全

1. zip 文件保存到私有目录，不进入公开上传目录。
2. 所有 zip entry 路径必须经过清洗。
3. 下载时只能通过 `jobId` 查找任务文件，禁止用户传任意路径。
4. `.part` 临时文件在失败后删除。
5. 过期任务和 zip 文件由清理逻辑删除。
6. 导入 staging zip 保存到私有目录，不进入公开上传目录。
7. inspect 阶段必须限制 zip 文件大小、entry 数量、单 entry 大小和总解压后大小。
8. 导入时只读取 zip 内 entry，不允许按 manifest path 访问本机文件系统。

### 12.4 敏感信息

导出包不包含：

1. 用户密码 hash。
2. refresh token。
3. operation token。
4. 系统配置密钥。
5. 私有对象存储凭据。

---

## 13. 审计与日志

空间导入导出都需要写后台审计日志。

导出审计字段：

1. `action`: `space.export`
2. `targetType`: `space`
3. `targetId`: `spaceId`
4. `metadata.format`
5. `metadata.jobId`
6. `metadata.includeAttachments`
7. `metadata.includeOfficeSources`
8. `metadata.fileName`
9. `metadata.sizeBytes`
10. `metadata.status`

导入审计字段：

1. `action`: `space.import`
2. `targetType`: `space`
3. `targetId`: 新空间 ID
4. `metadata.importId`
5. `metadata.jobId`
6. `metadata.sourceSpaceId`
7. `metadata.sourceSpaceName`
8. `metadata.newSpaceId`
9. `metadata.newSpaceName`
10. `metadata.status`

日志要求：

1. 创建任务时记录 info 日志。
2. 导出或导入完成时记录 info 日志。
3. 导出或导入失败时记录 warn 日志。
4. 日志中不打印 token。

---

## 14. 错误码建议

新增错误码语义：

1. `AdminSpaceExportErrSpaceIDRequired`
2. `AdminSpaceExportErrRequestBody`
3. `AdminSpaceExportErrFormatUnsupported`
4. `AdminSpaceExportErrJobNotFound`
5. `AdminSpaceExportErrJobTokenInvalid`
6. `AdminSpaceExportErrJobRunningLimit`
7. `AdminSpaceExportErrFileNotReady`
8. `AdminSpaceExportErrFileExpired`
9. `AdminSpaceExportErrDownloadForbidden`
10. `AdminSpaceImportErrFileRequired`
11. `AdminSpaceImportErrZipInvalid`
12. `AdminSpaceImportErrManifestMissing`
13. `AdminSpaceImportErrTreeMissing`
14. `AdminSpaceImportErrPackageUnsupported`
15. `AdminSpaceImportErrPackageNotImportable`
16. `AdminSpaceImportErrStagingNotFound`
17. `AdminSpaceImportErrStagingExpired`
18. `AdminSpaceImportErrJobRunningLimit`
19. `AdminSpaceImportErrCommitForbidden`

错误响应仍使用现有 `JsonResult` 协议：

```json
{
  "code": 4000,
  "message": "导出格式不支持",
  "requestId": "req-demo",
  "data": null
}
```

---

## 15. 测试方案

### 15.1 后端单元测试

重点覆盖：

1. 无空间导出权限的用户无法创建导出任务。
2. `space_admin` 无 scope 时无法导出目标空间。
3. `platform_admin` 可以导出任意正常空间。
4. 具备创建空间能力的普通用户可以导入 zip。
5. 不具备创建空间能力的普通用户不能导入 zip。
6. 普通用户不能导出自己无管理权的空间。
7. 不支持的 `format` 返回错误。
8. 同一用户同一空间已有 running 任务时拒绝新任务。
9. zip entry 路径清洗防止 zip slip。
10. Markdown 文档写入路径符合目录树。
11. 附件缺失时任务失败并推送 failed。
12. completed 事件包含可用下载链接。
13. 下载 token 过期后拒绝下载。
14. 导出的 zip 包包含可导入所需的 manifest、tree、文档、附件和 Office source。
15. inspect 非 PlainDoc zip 返回错误。
16. inspect 缺少 `manifest.json` 或 `tree.json` 返回错误。
17. inspect 缺少 source 文件时返回 `importable=false`。
18. commit 导入后创建新空间，不复用原始空间 ID。
19. 导入后目录树、文档内容、附件数量与 manifest 一致。
20. 导入失败时清理或标记已创建的新空间。
21. EPUB 导出包含合法 `mimetype`、`container.xml`、`content.opf` 和 `nav.xhtml`。
22. EPUB 中 Markdown 文档生成 XHTML 章节。
23. EPUB 中 `docx/xlsx` 复用现有 Office HTML 渲染链路生成 XHTML 章节。
24. EPUB 不被导入 inspect 接口识别为空间交换包。

### 15.2 前端测试

重点覆盖：

1. 空间操作菜单显示“导出空间”。
2. 点击菜单打开导出浮层。
3. 初始态显示 zip 图标和格式选择。
4. 点击 zip 后调用 `startSpaceExport`。
5. 收到 `running` 事件后展示进度。
6. 收到 `completed` 事件后触发下载并展示手动下载入口。
7. 收到 `failed` 事件后展示错误信息。
8. 关闭浮层时正确关闭 SSE 连接。
9. 空间管理上方显示“导入空间”按钮。
10. 选择 zip 后调用 `inspectSpaceImport`。
11. inspect 成功后展示导入预览和 warnings。
12. `importable=false` 时禁用确认导入。
13. 确认导入后调用 `commitSpaceImport`。
14. 收到导入 `completed` 事件后展示新空间入口。
15. 选择 EPUB 导出格式时，浮层显示 EPUB 阅读包文案。
16. EPUB completed 后触发 `.epub` 文件下载。

### 15.3 回归命令

后端：

```bash
cd apps/server && go test ./... -count=1
```

前端：

```bash
npm run web:build
npm run check:dropdown-menu -w @plaindoc/web
```

完整提交前：

```bash
cd apps/server && go test -race -timeout 120s ./...
```

---

## 16. 分阶段实施清单

### Phase 1：方案骨架与契约

- [ ] 新增前端导入导出类型和 `AdminGateway` 方法签名。
- [ ] 新增后端导入导出 request/response DTO。
- [ ] 注册 `POST /api/admin/spaces/:spaceId/exports`。
- [ ] 注册 `POST /api/admin/space-imports/inspect`。
- [ ] 注册 `POST /api/admin/space-imports/:importId/commit`。
- [ ] 实现导入导出格式校验和权限校验。
- [ ] 返回 `jobId` 和 `streamUrl`。
- [ ] 将 `epub` 加入导出格式枚举，但标记为不可导入格式。

### Phase 2：任务与 SSE

- [ ] 新增 `AdminSpaceExportJobStore`。
- [ ] 新增 `AdminSpaceImportJobStore`。
- [ ] 实现导入导出任务状态流转。
- [ ] 注册导入导出 SSE events 路由。
- [ ] 实现 `queued/running/completed/failed` 事件推送。
- [ ] 前端导入导出浮层接入 SSE 并展示进度。

### Phase 3：可回灌 Zip 导出

- [ ] 实现目录树读取。
- [ ] 实现 Markdown 文档导出。
- [ ] 实现 `manifest.json`。
- [ ] 实现 `tree.json`。
- [ ] 实现 zip entry 路径清洗。
- [ ] 实现 `.part -> .zip` 原子收尾。
- [ ] 确保导出包默认包含原样导入所需的附件和 Office source。

### Phase 4：附件与 Office 源文件

- [ ] 实现附件导出。
- [ ] 实现 Office source blob 导出。
- [ ] 实现缺失文件失败策略。
- [ ] 在 manifest 中记录附件和源文件映射。

### Phase 5：EPUB 阅读包导出

- [ ] 经确认后引入 `github.com/go-shiori/go-epub` 依赖。
- [ ] 实现 EPUB 标题页、目录和基础样式生成。
- [ ] 实现 Markdown 文档 XHTML 章节生成。
- [ ] 复用 `OfficeHTMLRenderService.RenderImportHTML` 渲染 `docx/xlsx` 纯 HTML。
- [ ] 实现 Office HTML 到 EPUB XHTML 的清洗与降级。
- [ ] 实现图片资源本地化与路径改写。
- [ ] 基于 `go-epub` 组装章节、CSS、图片与 EPUB 输出文件。
- [ ] 前端导出浮层支持 EPUB 格式选择和 `.epub` 下载。

### Phase 6：导入解析与预览

- [ ] 实现 zip 上传 staging。
- [ ] 实现 `manifest.json` 和 `tree.json` 解析。
- [ ] 实现导入包完整性校验。
- [ ] 实现导入预览响应。
- [ ] 前端实现“导入空间”按钮和导入浮层。
- [ ] 前端展示空间元数据、统计信息和 warnings。

### Phase 7：导入落地

- [ ] 实现新空间创建。
- [ ] 实现 oldID -> newID 映射。
- [ ] 实现目录树恢复。
- [ ] 实现 Markdown 文档恢复。
- [ ] 实现 Office source blob 恢复。
- [ ] 实现附件恢复。
- [ ] 导入完成后返回新空间入口。

### Phase 8：下载、审计与清理

- [ ] 实现短期下载 token。
- [ ] 注册下载接口。
- [ ] 前端 completed 后自动下载。
- [ ] 写入导入导出后台审计日志。
- [ ] 增加过期任务、导出 zip 和导入 staging zip 清理。

### Phase 9：测试与文档同步

- [ ] 补充后端导入导出服务测试。
- [ ] 补充 handler 权限测试。
- [ ] 补充前端导入导出浮层测试。
- [ ] 执行后端测试和前端构建。
- [ ] 同步更新 `BACKEND_DEVELOPER_GUIDE.md` 和 `FRONTEND_DEVELOPER_GUIDE.md` 中的空间导入导出说明。

---

## 17. 风险与后续演进

### 17.1 主要风险

1. 大空间导出耗时较长，占用磁盘和内存。
2. 附件或 Office 源文件可能位于远端对象存储，读取失败需要清晰暴露。
3. 进程内任务表不支持服务重启恢复。
4. 浏览器可能拦截自动下载，需要保留手动下载入口。
5. 大 zip 导入可能产生大量数据库写入和对象存储写入，需要限制文件大小与并发。
6. 导入失败回滚需要小心处理，避免留下半成品空间。
7. 不同版本 manifest 兼容性需要严格校验。

### 17.2 后续演进

后续可按实际使用情况追加：

1. 导出任务历史页面。
2. 任务取消能力。
3. 部分成功导出和 warnings manifest。
4. 数据库存储任务记录。
5. 分布式队列。
6. 覆盖导入到已有空间。
7. 跨版本导入兼容策略。
8. 导入前差异预览。
