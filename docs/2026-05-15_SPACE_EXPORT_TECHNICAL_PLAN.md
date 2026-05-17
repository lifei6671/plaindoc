# 空间导入导出功能技术方案

**文档状态**: In Progress
**创建日期**: 2026-05-15  
**适用范围**: `apps/server`、`apps/web`、`docs`  
**目标**: 在管理后台空间管理中增加空间导出与导入能力。导出侧从空间操作菜单发起，后端异步生成可回灌的 `.plaindoc` 空间交换包，或生成用于离线阅读分发的 EPUB 文件；导入侧在空间管理上方提供“导入空间”按钮，选择 `.plaindoc` 后先解析元数据，再将导出包原样导入为一个新空间。导入和导出任务统一进入全局任务中心，只要任一任务处于进行中，后台右下角展示全局进度浮层；点击浮层可展开查看所有导入/导出任务。页面刷新后，只要用户仍保持登录态，前端应能恢复当前用户未结束任务并继续展示进度。

> **修订说明（2026-05-17）**：EPUB 不再仅作为阅读导出产物。新的产品口径要求后台空间导入支持 `.epub`，并将 EPUB 作为一个新空间导入，目录映射为空间目录树，章节 HTML 转换为 Markdown。EPUB 导入细节以 `docs/2026-05-17_EPUB_SPACE_IMPORT_AND_DOCUMENT_HISTORY_TECHNICAL_PLAN.md` 为准；本文中“EPUB 不作为导入包”的旧表述仅保留为历史背景。

---

## 1. 方案结论

采用“空间交换包 + 持久化后台任务 + 全局任务中心 + SSE 进度订阅 + 短期文件链接”的实现方式。

导出核心流程如下：

```text
管理后台空间列表
  → 操作菜单点击“导出空间”
  → 打开导出浮层
  → 选择导出格式
  → 点击开始导出
  → POST 创建导出任务
  → 任务进入全局任务中心
  → 右下角浮层展示导出进度
  → 前端订阅 SSE 进度，刷新后可通过任务 API 恢复订阅
  → 后端异步生成 `.plaindoc` 或 EPUB
  → SSE 推送 completed + downloadUrl
  → 前端展示手动下载入口
```

本方案默认不让 `POST` 请求直接返回文件，也不让导出动作占用一个长时间 HTTP 请求。导出任务由后端异步执行，前端只负责创建任务、展示进度和拉起下载。

导入核心流程如下：

```text
管理后台空间列表上方
  → 点击“导入空间”
  → 选择 `.plaindoc` 空间交换包
  → POST 上传并解析 manifest/tree 元数据
  → 前端展示导入预览
  → 用户确认导入到新空间
  → POST 提交导入任务
  → 任务进入全局任务中心
  → 右下角浮层展示导入进度
  → 前端订阅 SSE 进度，刷新后可通过任务 API 恢复订阅
  → 后端创建新空间并恢复目录、文档、附件和 Office 源文件
  → SSE 推送 completed + 新空间信息
```

本方案要求系统导出的 `.plaindoc` 包必须能被同版本或兼容版本的 PlainDoc 原样导入。导入时不复用原空间 ID、节点 ID、文档 ID、附件 ID，而是在新空间中生成新 ID，并通过导入过程中的 `oldID -> newID` 映射恢复引用关系。

全局任务中心是导入导出的唯一进度归属。导入/导出弹窗只负责收集参数和发起任务，不再承载长生命周期进度状态。任务状态由后端持久化，前端在后台壳层统一恢复、订阅、展示和收尾。

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
8. 导出文件支持 `.plaindoc` 空间交换包、Markdown ZIP 内容归档包和 EPUB 阅读包。
9. 权限按能力判断，不按单一管理员角色判断：有目标空间管理权限的用户可导出该空间；具备创建空间能力的用户可导入 `.plaindoc` 创建新空间。
10. EPUB 导出复用现有阅读渲染链路：Markdown 必须走阅读页 SSR Worker，`docx/xlsx` 走无副作用纯 HTML 渲染，不追求 Office 高保真还原。
11. 在空间管理列表上方增加“导入空间”按钮，支持选择 `.plaindoc` 空间交换包。
12. 导入前必须解析 `manifest.json` 和 `tree.json`，展示空间名称、文档数、附件数、Office 源文件数和导出版本。
13. 用户确认后，系统将 `.plaindoc` 包导入为一个新空间，并原样恢复目录树、文档内容、附件、Office 源文件和空间封面。
14. 导入过程同样通过 SSE 推送进度，完成后返回新空间 ID 和可跳转入口。
15. 导入和导出任务统一展示在后台全局右下角浮层中。
16. 页面刷新后，若用户仍处于登录态，前端必须通过后端任务列表恢复当前用户的 `queued/running` 任务。
17. 导出完成后不自动消耗下载 token，用户点击下载时再获取一次性下载链接。
18. 后端持久化导入/导出任务快照，支持刷新恢复、终态短期保留和后续排障。

### 2.2 非目标

本期不做以下能力：

1. 多空间批量导出。
2. 独立导入导出历史页面。
3. 导出任务手动取消。
4. 分布式任务调度和多实例 worker 协调。
5. PDF、HTML 站点包、Confluence、Notion 等额外格式。
6. 导出后长期保存文件。
7. 对 Office 文档做高保真格式转换。
8. 覆盖导入到已有空间。
9. 导入时复用原始空间 ID 或原始数据主键。
10. 导入第三方系统生成的 zip 包。
11. 服务进程重启后自动续跑已经中断的导入/导出 worker。
12. 将全局任务中心扩展为通用后台任务平台；本方案只覆盖空间导入/导出。

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
2. `OfficeHTMLRenderService.RenderExportHTML`
3. `docx` 通过 Mammoth 转纯 HTML
4. `xlsx` 通过 excelize 渲染为多 tab table

`RenderExportHTML` 必须以任务级导出模式内联图片，不能复制 `OfficeHTMLRenderService` 实例来置空依赖；该 service 内含 `sync.Once` 和任务 channel，复制会触发 `copylocks` 风险。

Markdown EPUB 章节必须复用阅读页 SSR Worker：

1. 后端通过 `AdminSpaceExportSSRReaderHTMLRenderer` 构造 `space-reader` payload。
2. Worker 返回完整阅读页 HTML 后，只提取 `#plaindoc-preview-body` article 作为 EPUB 章节主体。
3. 不允许用 Go 侧 Markdown 解析器直接渲染正文，也不允许把原始 `.md` 内容作为 EPUB 章节。
4. SSR Worker 未启用或渲染失败时，EPUB Markdown 章节导出应失败，而不是降级为原始 Markdown。

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
	 - Office 文档不转换为 Markdown，也不导出为可恢复 source；该格式固定为内容归档包，`importable=false`。
2. `source_zip`
   - 偏完整备份。
   - 导出目录树、空间封面、Markdown 文档、Office 源文件、附件和 manifest。
   - 下载文件后缀为 `.plaindoc`，底层仍是 zip 容器。
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
6. 源空间已有封面文件和封面元数据

如果源空间没有封面，后端不在导出包中伪造封面；导入完成后，前端会复用“创建空间”的浏览器默认封面生成逻辑生成并上传默认封面。普通导入用户可以创建封面资产，并且只允许对自己导入的新空间执行纯封面绑定，不能借此修改名称、分类、可见性等其它元数据。普通空间 owner 只能绑定自己创建的封面资产；具备空间管理权限的管理员才可以绑定其它已有封面资产。

导入已有封面时，服务端必须在写入封面对象前校验 payload：`sha256`、文件大小、真实 WebP 解码结果、尺寸上限和像素数都必须通过；资产宽高以解码结果为准，不信任 manifest 中的宽高字段。

只有 `source_zip` 且同时包含附件与 Office 源文件时，manifest 才能标记 `importable=true`。服务端创建 `source_zip` 导出任务时会强制开启附件和 Office 源文件，避免前端绕过选项后生成不可回灌的 `.plaindoc`。

EPUB 导出不参与 `.plaindoc` 原样回灌约束；导出的 EPUB 仍是阅读产物，允许为了阅读器兼容性做语义级降级。EPUB 作为外部文件导入新空间的能力属于后续补充范围，详见 `docs/2026-05-17_EPUB_SPACE_IMPORT_AND_DOCUMENT_HISTORY_TECHNICAL_PLAN.md`。

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
2. 基于完整目录树生成的 EPUB 目录，包含文件夹、父文档和子文档的上下级关系。
3. Markdown 文档章节。
4. `docx` 文档章节：复用 Mammoth 生成的纯 HTML。
5. `xlsx` 文档章节：复用 excelize 生成的多 sheet HTML table。
6. 图片资源：尽量打包进 EPUB，并改写为相对路径。
7. 附件清单：普通附件不直接作为阅读章节，只生成附件列表。

明确不做：

1. `.epub` 导出产物的原样反向回灌；外部标准 EPUB 导入新空间以 2026-05-17 补充方案为准。
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

1. Markdown 必须通过阅读页 SSR Worker 渲染为 HTML，再提取 `#plaindoc-preview-body` 并清洗为 EPUB 兼容 XHTML；未注入 SSR renderer 或渲染失败时导出任务失败，禁止使用 Go Markdown fallback 静默生成不一致内容。
2. EPUB 保留阅读页/分享页的正文语义与代码块内容，但必须剥离代码块复制按钮等浏览器交互控件，避免静态阅读器出现“复制/复制成功”按钮。
3. Office 文档调用无副作用的 `OfficeHTMLRenderService.RenderExportHTML` 生成纯 HTML。
4. 清洗阶段移除脚本、事件属性和阅读器不支持的危险属性。
5. 将可信 `<img src>` 改写为 EPUB 内部相对路径。可信来源仅包括 `data:image/*` 和 `/uploads/*`；写入 EPUB 前先落到服务端私有临时目录，再交给 `go-epub.AddImage`。任意远程 URL、本机绝对路径或其他未知来源必须降级为 alt 文本。单张图片最大 20MiB，超限时按图片缺失降级，不中断 EPUB 导出。
6. 宽表格允许横向滚动样式降级，但不追求 Excel 原样宽度。
7. 每个文档节点独立生成一个章节，目录层级来自 `tree.json` 或导出时内存目录树；文档节点本身也可以作为父级继续挂载子文档。
8. EPUB 输出不得包含 Markdown 源语法（如 `# 标题`、`![图片]`）作为章节正文。

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
space-{spaceId}-{yyyyMMddHHmmss}.plaindoc
```

zip 内部结构：

```text
space-{spaceId}/
├── manifest.json
├── tree.json
├── covers/
│   └── space-cover.webp
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
	    "visibility": "member",
	    "cover": {
	      "path": "covers/space-cover.webp",
	      "mimeType": "image/webp",
	      "width": 1600,
	      "height": 2560,
	      "source": "user_upload",
	      "sha256": "..."
	    }
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
4. 如果前端在任务已 completed 后才建立 SSE 订阅，服务端不得重放旧明文 token，应为该次订阅签发新的短期一次性下载 token。
5. 响应使用 `Content-Disposition: attachment`。
6. 响应设置 `Cache-Control: no-store` 和 `Referrer-Policy: no-referrer`。
7. zip 文件只从服务端私有目录读取，不放到公开 `/uploads` 目录。

### 6.4 解析空间导入包

```http
POST /api/admin/space-imports/inspect
Content-Type: multipart/form-data
Authorization: Bearer <access-token>
```

表单字段：

1. `file`: `.plaindoc` 空间交换包。

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
    "exportedAt": "2026-05-15T12:00:00Z",
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
4. 若 `manifest.importable=false`，保留预览并返回 warnings，但后续 commit 禁止提交。
5. 若 `importable=true` 但缺少文档、附件或 Office source 引用文件，inspect 直接返回 package not importable。
6. inspect 必须通过 `CanImportSpace(actorUserID)`，避免无创建空间能力的用户滥用 staging 存储或探测导入包结构。
7. `.plaindoc` inspect 会拒绝重复 zip entry、不安全路径、超大上传、超大元数据 entry、过多 entry 和超出总解压大小上限的包；`.epub` inspect 属于后续补充能力，校验规则见 2026-05-17 EPUB 空间导入方案。

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

### 6.7 查询当前用户空间传输任务

```http
GET /api/admin/space-transfer-tasks?status=active
Authorization: Bearer <access-token>
```

说明：

1. 该接口是页面刷新恢复的入口。
2. 只返回当前登录用户可见的任务，不允许通过参数查询他人任务。
3. `status=active` 返回 `queued/running` 任务；不传时返回当前用户短期保留的最近任务。
4. 返回结果按 `updatedAt desc` 排序。
5. 终态任务默认短期保留，供用户下载或查看失败原因；超过 `expiresAt` 后由清理任务删除。

响应：

```json
{
  "code": 0,
  "message": "ok",
  "requestId": "req-demo",
  "data": {
    "items": [
      {
        "jobId": "01hspaceexportjob0000000001",
        "kind": "space_export",
        "status": "running",
        "stage": "documents",
        "progress": 45,
        "message": "正在导出文档 9/20",
        "spaceId": "space-demo",
        "spaceName": "示例空间",
        "format": "source_zip",
        "fileName": "",
        "sizeBytes": 0,
        "newSpaceId": "",
        "createdAt": "2026-05-17T10:00:00Z",
        "updatedAt": "2026-05-17T10:01:30Z",
        "expiresAt": "2026-05-17T10:30:00Z"
      }
    ]
  }
}
```

### 6.8 重新签发任务订阅链接

```http
POST /api/admin/space-transfer-tasks/:kind/:jobId/stream-token
Authorization: Bearer <access-token>
```

说明：

1. 用于页面刷新后重新订阅任务进度。
2. `kind` 只能是 `space_export` 或 `space_import`。
3. 后端必须校验任务归属当前 actor。
4. 只为未过期任务签发新的短期 `streamUrl`。
5. 原 stream token 过期不影响任务本身，只影响旧 SSE 链接继续使用。

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

### 6.9 获取任务快照

```http
GET /api/admin/space-transfer-tasks/:kind/:jobId
Authorization: Bearer <access-token>
```

说明：

1. 用于浮层展开时补齐单个任务快照，或 SSE 断开后的兜底查询。
2. 返回字段和任务列表项一致。
3. 如果任务是导出完成态，不直接返回可消费下载 token。

### 6.10 重新签发导出下载链接

```http
POST /api/admin/space-transfer-tasks/space_export/:jobId/download-token
Authorization: Bearer <access-token>
```

说明：

1. 仅导出任务支持。
2. 任务必须属于当前 actor，且状态为 `completed`。
3. 后端每次请求签发一个短期一次性 `downloadUrl`。
4. 全局任务列表和任务快照都不能返回可长期保存的下载链接。

响应：

```json
{
  "code": 0,
  "message": "ok",
  "requestId": "req-demo",
  "data": {
    "downloadUrl": "/api/admin/space-exports/01hspaceexportjob0000000001/download?token=***",
    "fileName": "space-demo-20260517100000.plaindoc",
    "sizeBytes": 1048576
  }
}
```

---

## 7. SSE 协议

### 7.1 事件类型

统一事件结构：

```json
{
  "type": "progress",
  "status": "running",
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

1. `progress`
   - 任务处于 `queued` 或 `running`，通过 `stage/progress/message` 表示当前阶段。
2. `completed`
   - 任务完成。SSE 首次 completed 事件可以包含短期 `downloadUrl`；任务列表和快照接口不返回可消费下载 token。
3. `failed`
   - 任务失败，包含失败阶段和错误信息。

导出任务的 `completed` SSE 事件可以包含 `downloadUrl`；若订阅建立时任务已经完成，初始 `completed` 快照也必须包含重新签发的可用 `downloadUrl`。全局浮层中用户点击“下载”时，优先调用 `download-token` API 重新签发一次性链接。导入任务的 `completed` 事件包含新空间的 `spaceId/editorUrl/readerUrl`。

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
3. `apps/server/internal/service/admin_space_transfer_task_service.go`
4. `apps/server/internal/storage/models/admin_space_transfer_job.go`
5. `apps/server/internal/storage/repository/gorm_admin_space_transfer_job_repository.go`
6. `apps/server/internal/server/handler/admin_space_export.go`
7. `apps/server/internal/server/handler/admin_space_import.go`
8. `apps/server/internal/server/handler/admin_space_transfer_task.go`
9. `apps/server/internal/server/response/admin_space_export.go`
10. `apps/server/internal/server/response/admin_space_import.go`
11. `apps/server/internal/server/response/admin_space_transfer_task.go`

服务结构：

```go
type AdminSpaceExportService struct {
    spaceRepo              repository.SpaceRepository
    workspaceRepo          repository.WorkspaceRepository
    documentAttachmentRepo repository.DocumentAttachmentRepository
    adminAccessService     *AdminAccessService
    jobStore               *AdminSpaceExportJobStore
    transferJobRepo        repository.AdminSpaceTransferJobRepository
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
    transferJobRepo        repository.AdminSpaceTransferJobRepository
    stagingRootDir         string
}
```

统一任务服务结构：

```go
type AdminSpaceTransferTaskService struct {
    exportService  *AdminSpaceExportService
    importService  *AdminSpaceImportService
    transferJobRepo repository.AdminSpaceTransferJobRepository
}
```

职责：

1. 聚合导出和导入任务为统一 DTO。
2. 查询当前 actor 可见任务。
3. 为已有任务重新签发 SSE `streamUrl`。
4. 为导出完成任务重新签发一次性 `downloadUrl`。
5. 提供前端全局浮层使用的任务快照。

### 8.2 任务状态

导入导出任务最终版使用“数据库持久化快照 + 进程内订阅者表”：

1. 数据库存储任务身份、归属、状态、进度、文件结果和过期时间。
2. 进程内 store 只保存当前进程的订阅者 channel、短期 token hash 和正在运行的 worker 引用。
3. 页面刷新后，前端通过 `GET /api/admin/space-transfer-tasks` 查询当前用户任务，再通过 `stream-token` 重新建立 SSE。
4. 服务进程重启后，已中断的 `queued/running` 任务不能自动续跑，应在启动恢复或查询时标记为 `failed`，错误信息为“服务重启，任务已中断，请重新发起”。
5. 终态任务短期保留，超过 `expiresAt` 后清理任务记录和对应私有文件。

任务状态结构：

```go
type AdminSpaceTransferJob struct {
    ID            uint
    JobID         string
    Kind          string // space_export | space_import
    ActorUserID   string
    SpaceID       string
    SpaceName     string
    Format        string
    ImportID      string
    Status        string // queued | running | completed | failed
    Stage         string
    Progress      int
    Message       string
    FilePath      string
    FileName      string
    SizeBytes     int64
    NewSpaceID    string
    ErrorMessage  string
    CreatedAt     time.Time
    StartedAt     *time.Time
    CompletedAt   *time.Time
    UpdatedAt     time.Time
    ExpiresAt     time.Time
}
```

建议新增表名：

```text
admin_space_transfer_jobs
```

索引：

1. `uk_admin_space_transfer_jobs_job_id`：唯一约束 `job_id`。
2. `idx_admin_space_transfer_jobs_actor_status_updated`：`actor_user_id, status, updated_at`。
3. `idx_admin_space_transfer_jobs_status_expires`：`status, expires_at`。
4. `idx_admin_space_transfer_jobs_kind_job`：`kind, job_id`。

迁移要求：

1. 同步新增 SQLite/MySQL/PostgreSQL 三套 migration。
2. PostgreSQL 可使用项目当前支持的 dollar-quoted block。
3. 不手工编辑 `go.sum`。
4. repository 查询优先使用 models 结构体或 `TableName()` 派生表名，避免硬编码长 SQL。

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
5. service 写入 `admin_space_transfer_jobs`，状态为 `queued`。
6. service 注册进程内订阅状态并推送 `queued` 事件。
7. service 启动受控后台任务。
8. handler 返回 `jobId` 和 `streamUrl`。

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
11. 更新持久化任务为 `completed`，写入文件名、文件路径、大小和过期时间。
12. 推送 `completed` 事件。

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
11. 更新持久化任务为 `completed`，写入文件名、文件路径、大小和过期时间。
12. 推送 `completed` 事件。

### 9.3 失败处理

失败规则：

1. 权限失败：任务不创建，直接返回错误。
2. 数据读取失败：任务进入 `failed`，SSE 推送失败阶段。
   - 同时更新 `admin_space_transfer_jobs.status=failed`、`stage`、`message`、`error_message` 和 `expires_at`。
3. 单个附件缺失：
   - 第一期建议视为任务失败，避免导出包不完整。
   - 后续如需要“部分成功”，可在 manifest 中增加 `warnings`。
4. zip 写入失败：删除 `.part` 文件，任务进入 `failed`。
5. SSE 断开：
   - 不取消后端任务。
   - 前端可通过任务查询接口恢复任务，并重新签发 `streamUrl`。
6. 服务重启：
   - 未完成任务无法自动续跑。
   - 启动恢复或任务查询时将遗留 `queued/running` 任务标记为 `failed`，提示用户重新发起。

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
8. staging 读取时只保留 zip entry 索引；manifest 引用的文档、附件、source 和封面在恢复阶段按需打开 entry 读取，未引用 entry 只参与 zip entry 数量、大小和路径安全校验，不读入内存。
9. 返回导入预览。

### 10.2 提交导入任务

1. handler 读取 `importId`。
2. service 校验 staging 记录存在、未过期、归属当前 actor。
3. 再次调用 `CanImportSpace(actorUserID)`，确认当前用户仍具备创建新空间的能力。
4. service 校验 `importable=true`。
5. service 创建导入 job 并推送 `queued` 事件。
6. service 写入 `admin_space_transfer_jobs`，状态为 `queued`。
7. service 启动受控后台任务。
8. handler 返回 `jobId` 和 `streamUrl`。

### 10.3 执行导入任务

后台任务步骤：

1. 再次调用 `CanImportSpace(actorUserID)`，避免任务创建后创建空间能力被撤销仍继续导入。
2. 重新打开 staging zip。
3. 创建新空间。
4. 建立 `oldSpaceId -> newSpaceId` 映射。
5. 按 `tree.json` 拓扑顺序创建 folder 节点。
6. 按文档节点创建 document：
   - `markdown`：读取 manifest 中的 `path`，创建 Markdown 文档。
   - `docx/xlsx`：读取 `sources/{oldDocumentId}/...`，创建 Office 文档并保存 source blob；source MIME 必须按文档格式固定映射，不能用内容嗅探把 Office ZIP 容器误判为 `application/zip`。
   - 新建 document 必须显式写入默认 `theme_id=default`，保持和协作端新建文档路径一致。
7. 建立 `oldDocumentId -> newDocumentId` 映射。
8. 导入附件：
   - 读取 `attachments/{oldDocumentId}/...`。
   - 为新文档创建新附件和新 blob。
9. 回写空间更新时间。
10. 写审计日志。
11. 更新持久化任务为 `completed`，写入新空间 ID 和过期时间。
12. 推送 `completed` 事件，返回新空间入口。

### 10.4 导入失败处理

失败规则：

1. inspect 阶段发现 zip 结构非法：不创建 staging 记录，直接返回错误。
2. commit 阶段发现 staging 过期：返回过期错误。
3. 导入执行中失败：任务进入 `failed`，并通过空间仓储事务硬删除已创建的新空间，先移除文档、附件、file revision 等 blob 引用。
   - 同时更新持久化任务失败快照，记录原始失败阶段和错误摘要。
4. 如果无法完整回滚，任务仍保留原始失败阶段和原始错误，同时附带回滚错误，并写入审计日志。
5. 导入完成后删除 staging zip。
6. 导入失败后保留 staging zip 到过期时间，便于用户重试或排查。
7. 导入失败回滚必须清理本次导入新建的空间封面资产、本地封面对象、本地附件 blob 和 Office source blob；blob 必须先确认数据库记录可硬删除，再删除物理对象，复用的既有 blob 只删除新建引用，不删除物理对象。

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
apps/web/src/admin/components/AdminSpaceTransferFloatingPanel.tsx
apps/web/src/admin/space-transfer/AdminSpaceTransferTaskProvider.tsx
apps/web/src/admin/space-transfer/useAdminSpaceTransferTasks.ts
```

导出弹窗 props：

```ts
interface AdminSpaceExportDialogProps {
  open: boolean;
  space: AdminSpace | null;
  dataGateway: DataGateway;
  onOpenChange: (open: boolean) => void;
  onStartExport: (input: AdminSpaceExportStartInput & { spaceName?: string }) => Promise<void>;
}
```

导入组件 props：

```ts
interface AdminSpaceImportDialogProps {
  open: boolean;
  dataGateway: DataGateway;
  onOpenChange: (open: boolean) => void;
  onStartImport: (input: AdminSpaceImportCommitInput & {
    importId: string;
    sourceSpaceName?: string;
    needsDefaultCover?: boolean;
  }) => Promise<void>;
}
```

### 11.3 导出弹窗状态

状态机：

```text
idle
  → starting
  → submitted
  → failed
```

说明：

1. `idle`
   - 展示 zip 图标。
   - 展示格式选择。
   - 点击 zip 图标开始导出。
2. `starting`
   - 正在创建任务。
3. `submitted`
   - 任务已交给全局任务中心。
   - 弹窗可以关闭，右下角浮层继续展示进度。
4. `failed`
   - 展示错误信息。
   - 允许重新导出。

当导出格式为 `epub` 时，浮层文案需要把“zip 压缩包”切换为“EPUB 阅读包”。入口仍可复用同一导出浮层，但图标和确认按钮应根据格式变化。

### 11.4 导入弹窗状态

状态机：

```text
idle
  → inspecting
  → preview
  → starting
  → failed
```

说明：

1. `idle`
   - 展示文件选择区。
   - 只接受 `.plaindoc`。
2. `inspecting`
   - 上传 `.plaindoc` 并解析元数据。
3. `preview`
   - 展示空间名称、导出时间、文档数、附件数、Office 源文件数、warnings。
   - 允许修改新空间名称、空间 ID、分类和可见性。
4. `starting`
   - 正在提交导入任务。
5. 提交成功
   - 任务已交给全局任务中心。
   - 弹窗可以关闭，右下角浮层继续展示进度。
6. `failed`
   - 展示失败原因。
   - 允许重新选择 zip。

### 11.5 全局任务中心

全局任务中心挂载在 `AdminApp.tsx` 登录后的后台壳层中，覆盖所有后台页面。导入/导出弹窗只负责发起任务，不再保存长生命周期进度状态。

职责：

1. 初始化时调用 `listSpaceTransferTasks({ status: "active" })`。
2. 对每个 `queued/running` 任务调用 `issueSpaceTransferStreamToken`，重新建立 SSE。
3. 创建导出或提交导入后，将任务加入本地任务列表并立即订阅。
4. 维护 `subscriptionRef` map，按 `kind + jobId` 去重。
5. 收到 `progress/completed/failed` 事件后更新本地任务状态。
6. 任务完成或失败后关闭对应 SSE。
7. 页面卸载或退出登录时关闭全部 SSE。
8. 导出完成后点击下载时调用 `issueSpaceTransferDownloadToken`，再触发浏览器下载。
9. 导入完成后在浮层提供“打开编辑器”“打开阅读页”入口；如导入包无封面，由 Provider 统一执行默认封面生成和绑定。

前端任务模型：

```ts
interface AdminSpaceTransferTask {
  jobId: string;
  kind: "space_export" | "space_import";
  status: "queued" | "running" | "completed" | "failed";
  stage?: string;
  progress: number;
  message?: string;
  spaceId?: string;
  spaceName?: string;
  format?: AdminSpaceExportFormat;
  fileName?: string;
  sizeBytes?: number;
  newSpaceId?: string;
  createdAt: string;
  updatedAt: string;
  expiresAt: string;
}
```

恢复流程：

```text
AdminApp 登录态确认
  → Provider mount
  → GET /api/admin/space-transfer-tasks?status=active
  → 按任务重新签发 streamUrl
  → 建立 SSE
  → 右下角浮层展示 active 任务
```

如果恢复时某个任务已经被后端标记为 `failed`，浮层展示失败原因，不再重连 SSE。

### 11.6 右下角浮层

浮层显示规则：

1. 没有任务时不显示。
2. 存在 `queued/running` 任务时固定显示在后台右下角。
3. 存在未清除的 `completed/failed` 任务时可继续显示，方便用户下载或查看失败原因。
4. 点击折叠态浮层后展开任务列表。
5. 支持清除单个终态任务；清除只影响前端展示，不删除后端审计或任务记录。

折叠态内容：

```text
导入导出任务 · 2 个进行中
```

展开态每个任务展示：

1. 类型：导出 / 导入。
2. 空间名或空间 ID。
3. 状态 badge。
4. 进度条。
5. 当前阶段文案。
6. 失败原因。
7. 导出完成后的“下载文件”。
8. 导入完成后的“打开编辑器”“打开阅读页”。

UI 约束：

1. 使用固定尺寸和最大高度，任务多时内部滚动。
2. 移动端贴底展示，避免遮挡后台主操作按钮。
3. 不使用嵌套 card；浮层本身是一个工具面板，任务项可以是紧凑列表项。
4. 图标优先使用 `lucide-react`。

### 11.7 手动下载

收到 `completed` 事件后：

1. 展示“下载文件”按钮。
2. 用户点击后，前端调用 `issueSpaceTransferDownloadToken`，服务端为该次点击重新签发短期一次性 `downloadUrl`。
3. 前端创建隐藏 `<a>`。
4. 设置 `href = downloadUrl`。
5. 设置 `download = fileName`。
6. 调用 `click()`。

`downloadToken` 为单次使用 token。前端不要在 `completed` 事件里自动点击下载链接，否则会提前消耗 token，导致用户后续点击手动下载按钮时失败。
浏览器系统下载框弹出时，请求可能已经到达下载接口并消耗 token；即使用户随后取消保存，也不能复用旧链接。手动下载按钮每次点击都必须重新获取新的下载链接。
如果 `VITE_API_BASE_URL` 是绝对地址，前端必须先把 SSE 事件里的相对 `downloadUrl` 补全到后端 origin，再设置到 `<a href>`。

### 11.8 SSE 与任务 API 封装

建议在 adapter 中新增：

```ts
startSpaceExport(input): Promise<AdminSpaceExportStartResult>
subscribeSpaceExport(input): AdminSpaceExportSubscription
inspectSpaceImport(input): Promise<AdminSpaceImportInspectResult>
commitSpaceImport(input): Promise<AdminSpaceImportStartResult>
subscribeSpaceImport(input): AdminSpaceImportSubscription
listSpaceTransferTasks(input): Promise<AdminSpaceTransferTaskListResult>
getSpaceTransferTask(input): Promise<AdminSpaceTransferTask>
issueSpaceTransferStreamToken(input): Promise<AdminSpaceTransferStreamTokenResult>
issueSpaceTransferDownloadToken(input): Promise<AdminSpaceTransferDownloadTokenResult>
```

`subscribeSpaceExport` 可以返回一个轻量对象：

```ts
interface AdminSpaceExportSubscription {
  close(): void;
}
```

页面组件只消费回调，不直接拼接 SSE URL。全局 Provider 负责调用任务 API、恢复订阅和关闭订阅。

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
10. 任务列表、任务快照、stream token 重签和下载 token 重签都必须校验任务归属当前 actor。
11. 全局任务中心不能展示其它用户任务，即使当前用户是管理员。

### 12.2 Token

新增三类短期 token：

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

页面刷新恢复不依赖旧 token。前端刷新后先通过登录态查询任务，再请求服务端重新签发新的 `streamToken`。旧 `streamToken` 过期不应导致任务被清理或任务失败。

### 12.3 文件安全

1. zip 文件保存到私有目录，不进入公开上传目录。
2. 所有 zip entry 路径必须经过清洗。
3. 下载时只能通过 `jobId` 查找任务文件，禁止用户传任意路径；消费 `downloadToken` 后仍需校验文件位于导出私有目录内，且扩展名只能是 `.zip`、`.plaindoc` 或 `.epub`。
4. `.part` 临时文件在失败后删除。
5. 过期终态任务和 zip 文件由清理逻辑删除；`queued` / `running` 任务不因 SSE stream token 过期被清理，避免长任务后续完成事件和下载 token 丢失。
6. 持久化任务表中的 `file_path` 只能由服务端写入，下载前仍需校验路径位于导出私有目录。
7. 导入 staging zip 保存到私有目录，不进入公开上传目录。
8. inspect 阶段必须限制 zip 文件大小、entry 数量、单 entry 大小和总解压后大小。
9. 导入时只读取 zip 内 entry，不允许按 manifest path 访问本机文件系统。
10. 导入执行阶段按需读取 manifest 引用的 entry，避免 referenced payload 和未引用大文件同时驻留内存；未引用 entry 仍受 zip 总大小、entry 数量和路径清洗约束。
11. EPUB 导出本地化图片时，`data:image/*` 与 `/uploads/*` 单图都必须限制在 20MiB 内，避免超大图片占用过多内存或临时磁盘。
12. 审计错误信息只保留业务错误；若错误文本包含 token、私有目录或绝对路径，必须泛化为服务端日志可查。

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
20. `AdminSpaceTransferTaskErrJobNotFound`
21. `AdminSpaceTransferTaskErrKindUnsupported`
22. `AdminSpaceTransferTaskErrStreamTokenIssueFailed`
23. `AdminSpaceTransferTaskErrDownloadTokenIssueFailed`
24. `AdminSpaceTransferTaskErrForbidden`

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
24. EPUB 导出产物不被 `.plaindoc` inspect 识别为空间交换包；标准 `.epub` 文件作为新空间导入由 2026-05-17 补充方案覆盖。
25. 创建导出任务后写入 `admin_space_transfer_jobs`。
26. 创建导入任务后写入 `admin_space_transfer_jobs`。
27. 任务列表只返回当前 actor 的任务。
28. 页面刷新后可为当前 actor 的 active 任务重新签发 `streamUrl`。
29. 非任务 owner 不能重新签发 stream token 或 download token。
30. 导出完成后通过 `download-token` 接口签发一次性下载链接。
31. 服务启动恢复时，遗留 `queued/running` 任务被标记为 `failed`。

### 15.2 前端测试

重点覆盖：

1. 空间操作菜单显示“导出空间”。
2. 点击菜单打开导出浮层。
3. 初始态显示 zip 图标和格式选择。
4. 点击 zip 后调用 `startSpaceExport`。
5. 收到 `running` 事件后展示进度。
6. 收到 `completed` 事件后展示手动下载入口，点击后重新订阅 completed 快照并下载新签发的一次性链接。
7. 收到 `failed` 事件后展示错误信息。
8. 关闭浮层时正确关闭 SSE 连接。
9. 空间管理上方显示“导入空间”按钮。
10. 选择 `.plaindoc` 后调用 `inspectSpaceImport`。
11. inspect 成功后展示导入预览和 warnings。
12. `importable=false` 时禁用确认导入。
13. 确认导入后调用 `commitSpaceImport`。
14. 收到导入 `completed` 事件后展示新空间入口。
15. 选择 EPUB 导出格式时，浮层显示 EPUB 阅读包文案。
16. EPUB completed 后展示 `.epub` 文件手动下载入口。
17. 创建导出后弹窗关闭，右下角浮层仍展示进度。
18. 创建导入后弹窗关闭，右下角浮层仍展示进度。
19. Provider 初始化时调用任务列表并恢复 active 任务。
20. 恢复 active 任务后重新签发 `streamUrl` 并订阅 SSE。
21. 切换后台页面不丢失任务进度。
22. 刷新页面后仍显示登录用户进行中的导入/导出任务。
23. 导出完成后点击浮层下载按钮会调用 `issueSpaceTransferDownloadToken`。
24. 任务失败后浮层展示失败原因，并允许清除终态任务。

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

- [x] 实现目录树读取。
- [x] 实现 Markdown 文档导出。
- [x] 实现 `manifest.json`。
- [x] 实现 `tree.json`。
- [x] 实现 zip entry 路径清洗。
- [x] 实现 `.part -> .zip` 原子收尾。
- [x] 确保导出包默认包含原样导入所需的附件和 Office source。

### Phase 4：附件与 Office 源文件

- [ ] 实现附件导出。
- [ ] 实现 Office source blob 导出。
- [ ] 实现缺失文件失败策略。
- [ ] 在 manifest 中记录附件和源文件映射。

### Phase 5：EPUB 阅读包导出

- [x] 经确认后引入 `github.com/go-shiori/go-epub` 依赖。
- [x] 实现 EPUB 标题页、目录和基础样式生成。
- [x] 实现 Markdown 文档 XHTML 章节生成，SSR renderer 不可用时失败。
- [x] 复用 `OfficeHTMLRenderService.RenderExportHTML` 渲染 `docx/xlsx` 纯 HTML，避免导出阶段写 blob。
- [x] 实现 Office HTML 到 EPUB XHTML 的清洗与降级。
- [x] 实现可信图片资源本地化与路径改写，未知来源降级为 alt 文本。
- [x] 基于 `go-epub` 组装章节、CSS、图片与 EPUB 输出文件。
- [x] 前端导出浮层支持 EPUB 格式选择和 `.epub` 下载。

### Phase 6：导入解析与预览

- [x] 实现 zip 上传 staging。
- [x] 实现 `manifest.json` 和 `tree.json` 解析。
- [x] 实现导入包完整性校验。
- [x] 实现导入预览响应。
- [x] 前端实现“导入空间”按钮和导入浮层。
- [x] 前端展示空间元数据、统计信息和 warnings。

### Phase 7：导入落地

- [x] 实现新空间创建。
- [x] 实现 oldID -> newID 映射。
- [x] 实现目录树恢复。
- [x] 实现 Markdown 文档恢复。
- [x] 实现 Office source blob 恢复。
- [x] 实现附件恢复。
- [x] 导入完成后返回新空间入口。
- [x] 补齐 Office HTML 渲染排队与导入 warning 记录。
- [x] 补齐附件旧 ID 映射元数据与失败后未引用 blob 清理。

### Phase 8：下载、审计与清理

- [x] 实现短期下载 token。
- [x] 注册下载接口。
- [x] 前端 completed 后展示手动下载入口。
- [x] 写入导入导出后台审计日志。
- [x] 增加过期任务、导出 zip 和导入 staging zip 清理。

### Phase 9：测试与文档同步

- [x] 补充后端导入导出服务测试。
- [x] 补充 handler 权限测试。
- [x] 补充前端导入导出浮层测试。
- [x] 执行后端测试和前端构建。
- [ ] 同步更新 `BACKEND_DEVELOPER_GUIDE.md` 和 `FRONTEND_DEVELOPER_GUIDE.md` 中的空间导入导出说明。

### Phase 10：最终版全局任务中心与刷新恢复

- [ ] 新增 `admin_space_transfer_jobs` 持久化表和三套迁移。
- [ ] 新增 `AdminSpaceTransferJob` model、repository interface 和 GORM repository。
- [ ] 导出任务创建、进度更新、完成、失败都同步写入持久化任务表。
- [ ] 导入任务创建、进度更新、完成、失败都同步写入持久化任务表。
- [ ] 新增 `AdminSpaceTransferTaskService` 聚合导入导出任务。
- [ ] 新增 `GET /api/admin/space-transfer-tasks`。
- [ ] 新增 `GET /api/admin/space-transfer-tasks/:kind/:jobId`。
- [ ] 新增 `POST /api/admin/space-transfer-tasks/:kind/:jobId/stream-token`。
- [ ] 新增 `POST /api/admin/space-transfer-tasks/space_export/:jobId/download-token`。
- [ ] 服务启动恢复或任务查询时，将遗留 `queued/running` 任务标记为 `failed`。
- [ ] 前端新增 `AdminSpaceTransferTaskProvider`。
- [ ] 前端新增 `AdminSpaceTransferFloatingPanel`，挂载在 `AdminApp` 登录态后台壳层。
- [ ] 导出弹窗只负责发起任务，进度和下载交给全局任务中心。
- [ ] 导入弹窗只负责 inspect 和提交任务，进度、完成入口和默认封面补齐交给全局任务中心。
- [ ] 页面刷新后，Provider 查询 active 任务并重新签发 `streamUrl` 恢复 SSE。
- [ ] 补充后端任务列表、token 重签、下载重签、启动恢复测试。
- [ ] 补充前端浮层、刷新恢复、页面切换不中断、下载重签测试。
- [ ] 同步更新 `BACKEND_DEVELOPER_GUIDE.md` 和 `FRONTEND_DEVELOPER_GUIDE.md`。

---

## 17. 风险与后续演进

### 17.1 主要风险

1. 大空间导出耗时较长，占用磁盘和内存。
2. 附件或 Office 源文件可能位于远端对象存储，读取失败需要清晰暴露。
3. 持久化任务表支持刷新恢复，但不支持服务重启后自动续跑 worker；重启期间的 active 任务会失败，需要用户重新发起。
4. 下载链接使用一次性 token，需要避免前端自动触发下载提前消耗 token。
5. 大 zip 导入可能产生大量数据库写入和对象存储写入，需要限制文件大小与并发。
6. 导入失败回滚需要小心处理，避免留下半成品空间。
7. 不同版本 manifest 兼容性需要严格校验。

### 17.2 后续演进

后续可按实际使用情况追加：

1. 导出任务历史页面。
2. 任务取消能力。
3. 部分成功导出和 warnings manifest。
4. 分布式队列。
5. 覆盖导入到已有空间。
6. 跨版本导入兼容策略。
7. 导入前差异预览。
8. 独立任务历史页面和任务取消能力。
