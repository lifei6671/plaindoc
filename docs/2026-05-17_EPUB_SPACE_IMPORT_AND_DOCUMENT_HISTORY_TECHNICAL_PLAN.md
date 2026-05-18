# EPUB 空间导入与文档历史版本技术方案

**文档状态**: In Progress
**创建日期**: 2026-05-17
**适用范围**: `apps/server`、`apps/web`、`docs`
**目标**: 在现有后台空间导入能力中新增 EPUB 导入格式，将标准 EPUB 导入为一个新空间，并把 EPUB 目录映射为 PlainDoc 空间目录树；同时在编辑器右侧增加历史版本入口，以弹窗方式展示版本列表、当前版本差异，并支持恢复到指定历史版本。

---

## 1. 方案结论

本方案包含两条能力：

1. EPUB 空间导入
   - 沿用现有后台“导入空间”入口和导入任务中心。
   - `.epub` 与 `.plaindoc` 都属于空间导入格式。
   - EPUB 导入永远创建新空间，不覆盖已有空间。
   - 只要用户具备创建空间权限，就可以导入 EPUB。
   - EPUB 的目录结构导入为空间目录结构，章节正文转换为 Markdown 文档。
   - EPUB OPF 的 `dc:description` 写入空间简介；EPUB3 `cover-image` / EPUB2 `meta name="cover"` 复用现有空间封面处理链路，归一化为 WebP 空间封面资产。

2. 文档历史版本
   - 在编辑器右侧操作区增加历史版本 icon 按钮。
   - 点击后打开弹窗，左侧为版本列表，右侧为当前内容与选中历史版本的差异。
   - 版本列表展示版本号、创建时间和创建人。
   - 支持将当前文档恢复到指定历史版本；恢复不是覆盖历史记录，而是基于目标历史内容创建一个新的当前版本。
   - Markdown 文档使用文本差异视图；Office 文档首期展示文件版本元数据和恢复入口，不做二进制差异。

EPUB 容器解析不建议引入新的 Go 第三方包。仓库已经具备 `archive/zip`、`encoding/xml` 和 `golang.org/x/net/html`，足以解析标准 EPUB 的容器、OPF、nav/toc 和 XHTML 内容。新增第三方 EPUB 容器解析库的收益有限，反而会扩大依赖面和安全审计范围。

HTML 转 Markdown 建议引入 Go 依赖 `github.com/JohannesKaufmann/html-to-markdown/v2`。该库支持自定义规则和 renderer 扩展，适合把 EPUB XHTML 清洗结果转换成符合 PlainDoc 编辑体验的 Markdown。真正落地前需要按仓库规则单独确认依赖变更。

历史版本差异视图建议引入前端依赖 `@codemirror/merge`。项目已经使用 CodeMirror 6，使用同生态 diff 组件能复用编辑器样式、只读视图、行号和长文本渲染能力。该依赖变更需要在实施前单独确认。

当前实现已完成 EPUB inspect/commit、OPF 简介与封面导入、EPUB 导出空间封面、HTML 清洗与 Markdown 转换、图片本地化、内部链接重写、导入任务进度、SSE 自动重连与断线快照恢复、SSR Worker Node `--no-opt` 启动兜底、文档历史版本摘要/详情/恢复接口，以及前端历史版本弹窗。当前已完成 1 本真实 EPUB 样本的 inspect 验证；`go test -race -timeout 60s ./...` 与 `golangci-lint run ./...` 仍有已记录的收尾阻塞项。

---

## 2. 目标与非目标

### 2.1 目标

1. 后台空间导入弹窗支持选择 `.epub`。
2. `inspect` 阶段识别 EPUB，解析书名、作者、目录层级、章节数和图片资源数。
3. `commit` 阶段创建一个新空间，并按 EPUB 目录创建 `folder/doc` 节点。
4. EPUB XHTML/HTML 章节必须转换为 Markdown，写入 `documents.content_md` 和首版 `document_revisions.content_md`。
5. EPUB 内图片资源需要本地化为 PlainDoc 可访问资源，Markdown 链接指向本地资源 URL；OPF 声明的封面图片复用现有封面处理链路，归一化为 WebP 空间封面资产。
6. 权限按“是否可创建空间”判断，不能要求用户已有空间管理权限。
7. EPUB OPF `dc:description` 写入空间简介；缺失时再使用导入来源兜底描述。
8. 导入过程复用现有导入任务持久化、SSE 进度和右下角全局任务浮层；SSE 连接异常只代表事件通道中断，不能覆盖后端任务真实状态为失败；前端先允许 `EventSource` 在 30 秒窗口内自动重连，持续失败后再刷新后端任务快照，运行中任务重新订阅，已完成任务直接触发完成流程；EPUB 文档写入阶段按“已导入文档数 / 总文档数”推进进度。
9. 编辑器增加历史版本按钮与弹窗，支持查看 Markdown 历史版本差异。
10. 版本列表必须展示版本号、创建时间和创建人。
11. 用户可以选择一个历史版本并恢复；恢复操作需要二次确认，并生成一个新的文档版本。
12. 历史版本接口拆成“摘要列表”和“单版本详情”，避免列表接口一次性返回大量正文。

### 2.2 非目标

1. 不支持导入 DRM 加密 EPUB。
2. 不承诺固定版式 EPUB 的像素级还原。
3. 不把 EPUB 导入到已有空间。
4. 不在首期支持批量恢复、跨文档恢复或恢复后自动合并未保存内容。
5. 不在首期支持 Office 二进制内容差异。
6. 不新增 EPUB 长期解析任务平台；EPUB 导入仍属于空间导入任务。

---

## 3. 现有基础

### 3.1 空间导入基础

现有导入链路已经具备以下能力：

1. `apps/server/internal/server/handler/admin_space_import.go`
   - 负责 `/admin/space-imports/inspect`、`/admin/space-imports/:importId/commit` 和 SSE。
2. `apps/server/internal/service/admin_space_import_service.go`
   - 负责导入暂存、任务创建、空间创建、目录树恢复、附件/Office 源文件恢复和失败清理。
3. `apps/web/src/admin/components/AdminSpaceImportDialog.tsx`
   - 负责上传、inspect 预览、commit 和任务跟踪。
4. `apps/web/src/admin/space-transfer/AdminSpaceTransferTaskProvider.tsx`
   - 负责导入/导出任务恢复、订阅和右下角浮层。

EPUB 导入应复用这些入口，不另建编辑器导入入口，也不绕过全局任务中心。

### 3.2 HTML 转 Markdown 基础

现有编辑器导入链路已经有 HTML 到 Markdown 转换脚本：

1. `apps/server/internal/server/handler/workspace_import_conversion.go`
2. `apps/server/internal/service/scripts/convert_html_to_markdown.mjs`

EPUB 导入不建议继续依赖 handler 层的 Node 脚本。建议新增 service 层 HTML 转 Markdown 组件，基于 `github.com/JohannesKaufmann/html-to-markdown/v2` 实现：

1. 输入为已清洗、已完成链接和图片重写的 HTML 片段。
2. 输出为 PlainDoc Markdown。
3. 通过自定义规则处理 EPUB 常见标签、图片、表格、代码块、脚注、章节锚点和特殊 span。
4. 现有 Node 脚本可作为兼容参考或迁移前回退方案，但不作为 EPUB 导入的首选实现。

### 3.3 历史版本基础

现有工作区已经有 Markdown 修订表和接口：

1. `document_revisions`
2. `document_file_revisions`
3. `GET /api/docs/:docId/revisions`
4. `DocumentGateway.listRevisions`

需要调整的是接口粒度和前端呈现方式，而不是重新设计历史表。

---

## 4. EPUB 导入数据流

### 4.1 总体流程

```text
后台空间管理
  -> 导入空间
  -> 选择 .epub
  -> POST /admin/space-imports/inspect
  -> 解析 EPUB 元数据和目录
  -> 返回预览摘要
  -> 用户确认导入
  -> POST /admin/space-imports/:importId/commit
  -> 创建导入任务
  -> 创建新空间，写入 OPF description 和封面资产
  -> 创建 folder/doc 节点
  -> XHTML 清洗 + 图片本地化 + HTML 转 Markdown
  -> 写入 documents / document_revisions
  -> 按已写入文档数发布 epub_documents 进度
  -> SSE completed + newSpaceId
  -> 阅读页 SSR 按独立响应上限读取 Worker 输出
```

### 4.2 Inspect 阶段

`inspect` 接口需要识别文件后缀和 EPUB 容器：

1. 后缀为 `.epub`。
2. zip 内存在未压缩 `mimetype`，内容为 `application/epub+zip`。
3. 存在 `META-INF/container.xml`。
4. `container.xml` 指向 OPF package 文件。
5. OPF 中至少存在一个 spine item 可解析为 XHTML/HTML。

Inspect 返回结构保留现有字段，并增加 EPUB 摘要：

```ts
export type AdminSpaceImportPackageType = "plaindoc-space" | "plaindoc" | "epub";

export interface AdminSpaceImportInspectResult {
  importId: string;
  packageVersion: number;
  packageType: AdminSpaceImportPackageType;
  exportedAt?: string;
  sourcePublishedAt?: string;
  sourceAuthors?: string[];
  importable: boolean;
  space: {
    spaceId: string;
    name: string;
    categoryId?: string;
    visibility: Visibility;
    hasCover?: boolean;
  };
  summary: {
    folderCount: number;
    documentCount: number;
    attachmentCount: number;
    officeSourceCount: number;
    imageCount?: number;
    maxDepth?: number;
  };
  warnings: string[];
}
```

EPUB 没有原始 `spaceId`，服务端可在 inspect 响应中生成临时展示 ID，commit 时仍生成真实新空间 ID。

时间字段语义：

1. `.plaindoc`：`exportedAt` 表示 PlainDoc 包导出时间，沿用现有语义。
2. `.epub`：不填 `exportedAt`，避免把出版日期误解为系统导出时间。
3. `.epub` 如果 OPF metadata 中存在 `dc:date`，写入 `sourcePublishedAt`，仅作为书籍出版/更新时间展示。
4. `.epub` 如果 OPF metadata 中存在 `dc:creator`，写入 `sourceAuthors`，用于预览作者信息。
5. 前端展示时按 `packageType` 区分字段：`.plaindoc` 显示导出时间，`.epub` 显示作者和出版日期；字段为空则隐藏对应行或显示空态。
6. 现有 `.plaindoc` 导入包响应中的 `packageType` 仍可能是历史值 `plaindoc-space`，前端类型需保持兼容；新 EPUB 响应使用 `epub`。

### 4.3 Commit 阶段

Commit 继续使用现有参数：

```ts
export interface AdminSpaceImportCommitInput {
  importId: string;
  spaceName: string;
  spaceId?: string;
  categoryId?: string;
  visibility?: Visibility;
}
```

对于 EPUB：

1. `spaceId` 由服务端生成，不接受客户端指定覆盖已有空间。
2. `spaceName` 默认取 EPUB metadata title，允许用户在弹窗中修改。
3. `categoryId` 和 `visibility` 复用现有导入弹窗能力。
4. `visibility` 默认使用系统新空间默认策略；如果当前导入弹窗已有默认值，则沿用现有逻辑。

---

## 5. EPUB 解析设计

### 5.1 包结构读取

新增内部解析器，建议文件：

1. `apps/server/internal/service/admin_space_epub_importer.go`
2. `apps/server/internal/service/admin_space_epub_importer_test.go`

核心结构：

```go
type adminSpaceEPUBImportPackage struct {
	Title     string
	Authors   []string
	Description string
	CoverPath string
	RootDir   string
	Chapters  []adminSpaceEPUBImportChapter
	Resources map[string]adminSpaceEPUBImportResource
	Warnings  []string
}

type adminSpaceEPUBImportChapter struct {
	ID        string
	Title     string
	Href      string
	ParentKey string
	Order     int
	HTML      string
}

type adminSpaceEPUBImportResource struct {
	Href      string
	MediaType string
	Content   []byte
}
```

解析步骤：

1. 使用 `archive/zip` 打开 EPUB。
2. 校验 entry 路径，拒绝绝对路径、`..`、空路径和重复路径。
3. 读取 `META-INF/container.xml`，解析 rootfile。
4. 读取 OPF，解析 metadata、manifest、spine。
5. 优先解析 EPUB3 `nav.xhtml`，其次解析 EPUB2 `toc.ncx`。
6. 如果没有 nav/toc，则按 spine 顺序生成扁平目录。
7. 读取 spine 对应 XHTML/HTML，进入清洗与转换流程。

### 5.2 目录映射规则

EPUB nav/toc 映射为空间目录：

```text
EPUB nav/toc
  ├── Part 1
  │   ├── Chapter 1
  │   └── Chapter 2
  └── Part 2
      └── Chapter 3

PlainDoc 空间
  ├── Part 1(folder)
  │   ├── Chapter 1(doc)
  │   └── Chapter 2(doc)
  └── Part 2(folder)
      └── Chapter 3(doc)
```

规则：

1. 解析 nav/toc 时先把 `href` 规范化为 `canonicalHref + fragment`：
   - `canonicalHref` 为相对 OPF 根目录清洗后的 XHTML/HTML 路径，不包含 `#fragment`。
   - `fragment` 为 `#` 后的锚点，保留大小写；为空表示整章。
   - `targetKey = canonicalHref + "#" + fragment`，fragment 为空时 `targetKey = canonicalHref`。
2. 无 `href` 且有子节点的目录项创建为 `folder`。
3. 叶子目录项默认创建为 `doc`。
4. 有子节点且自身也指向正文的目录项创建为 `folder`，并在该 folder 下创建标题为 `正文` 的文档；若同级已存在 `正文`，沿用唯一标题策略生成 `正文 2`、`正文 3`。
5. 带 fragment 的目录项按 `targetKey` 粒度处理：
   - 若 fragment 能在对应 XHTML 中定位到元素 ID，则优先把该 fragment 范围拆成独立 Markdown 文档。
   - 拆分范围为当前锚点元素到同文件下一个 nav/toc 目标锚点之前。
   - 如果无法安全定位 fragment，则回退为整章文档，并记录 warning。
   - fragment 可定位性判断必须在第一段预分配阶段完成，不能推迟到章节转换阶段；只有这样才能正确预分配 `documentID` 并建立内部链接映射。
6. 多个目录项指向同一个 `targetKey` 时，第一个出现的位置创建规范文档；后续重复项创建“参见”占位文档，内容链接到规范文档，并记录 warning。这样既不在树中留空，也避免重复复制正文。
7. 标题为空时使用 `章节 001` 这类稳定标题。
8. 同级标题冲突沿用现有导入唯一标题策略。

“参见”占位文档使用固定 Markdown 模板：

```markdown
> 本章节内容见：[{{canonicalTitle}}]({{readerURL}})
```

其中 `canonicalTitle` 为规范文档标题，`readerURL` 为规范文档阅读链接。该链接不能在预分配阶段写死为占位路径，必须在 commit 写库阶段结合新空间 `spaceID` 生成真实 `/r/{spaceID}/{documentID}` 阅读地址。占位文档不复制正文，不参与 fragment 拆分。

### 5.3 XHTML 清洗与 Markdown 转换

EPUB 章节必须先从 XHTML/HTML 转 Markdown，再写入 PlainDoc。

处理顺序：

```text
原始 XHTML
  -> HTML parser 解析
  -> 只保留 body 主体
  -> 删除 script/style/form/input/button/noscript
  -> 改写 img/src 和 a/href
  -> 渲染回 HTML
  -> 调用 service 层 html-to-markdown converter
  -> 写入 Markdown 文档
```

链接处理：

1. commit 阶段必须两段式处理：
   - 第一段：解析 nav/toc/spine，预分配待创建的 `nodeID/documentID`，建立 `targetKey -> documentID` 和 `canonicalHref -> primary documentID` 映射表。
   - 第二段：转换章节 HTML 时结合新空间 `spaceID` 生成真实 `/r/{spaceID}/{documentID}` 阅读链接并回写 `<a href>`，再执行 HTML 转 Markdown。
2. 指向 EPUB 内部章节的 `href`：
   - 先按当前章节路径解析为 `targetKey`。
   - 如果 `targetKey` 命中映射，改写为目标 PlainDoc 文档阅读链接。
   - 如果仅 `canonicalHref` 命中映射，改写为该文件的主文档阅读链接，并记录 fragment 降级 warning。
   - 如果完全未命中，保留链接文本，移除 href，并记录 warning。
3. 指向图片资源的 `img/src`，转存为 PlainDoc 本地资源后改写为本地 URL。
4. 外部 `http/https` 链接保留。
5. `javascript:`、`file:`、本机绝对路径和未知危险协议直接移除或降级为纯文本。

图片处理：

1. EPUB 内部图片资源必须从 zip entry 读取，不发起网络请求。
2. 支持 `image/png`、`image/jpeg`、`image/gif`、`image/webp`；不支持 `image/svg+xml`。
3. XHTML 内联非 SVG `data:image/*` 视为合法图片来源，但不能原样写入 Markdown：
   - 先按 MIME 和 base64 解析。
   - 解码前后都必须执行大小限制。
   - 解码成功后写入 PlainDoc blob/image hosting，并把 Markdown 图片 URL 改写为本地资源 URL。
   - 非 `data:image/*` 的 data URI 一律移除或降级为文本。
4. 单张图片建议沿用 20MiB 上限，超限时替换为 alt 文本并记录 warning。
5. 图片写入复用现有 blob/image hosting 能力，不绕过存储抽象。
6. `image/svg+xml` 不进入图片本地化写入流程：
   - 文件型 SVG 和 `data:image/svg+xml` 均降级为 alt 文本并记录 warning。
   - 不把 SVG 内容内联进 Markdown，也不作为不透明 blob 存储。
   - 后续只有在引入可靠 SVG 白名单清洗和渲染隔离后，才能重新评估是否放开。

### 5.4 HTML 转 Markdown 实现策略

EPUB 导入首选使用 Go 库：

```text
github.com/JohannesKaufmann/html-to-markdown/v2
```

选择理由：

1. 避免每个章节启动 Node 进程，减少批量章节转换开销。
2. 转换逻辑留在 Go service 层，避免 service 反向依赖 handler 或外部脚本。
3. 支持自定义规则和 renderer 扩展，可针对 PlainDoc 的 Markdown 风格补充规则。
4. 能和 EPUB 的链接重映射、图片本地化、SVG 拒绝降级等 Go 侧流程在同一内存模型内衔接。

安全边界：

1. 该库只负责 HTML 到 Markdown 转换，不承担 HTML 安全清洗职责。
2. 调用 converter 前必须已经完成 HTML 节点清洗、危险协议过滤、图片本地化和 SVG 拒绝降级。
3. 自定义 renderer 不得重新放行已被清洗阶段拒绝的原始 HTML。

建议新增：

1. `apps/server/internal/service/html_markdown_converter.go`
2. `apps/server/internal/service/html_markdown_converter_test.go`

核心接口：

```go
type HTMLMarkdownConverter interface {
	Convert(ctx context.Context, input ConvertHTMLMarkdownInput) (ConvertHTMLMarkdownResult, error)
}

type ConvertHTMLMarkdownInput struct {
	HTML       string
	SourceKey  string
	PlainDocMode bool
}

type ConvertHTMLMarkdownResult struct {
	Markdown string
	Warnings []string
}
```

自定义规则至少覆盖：

1. `<img>`：输出 `![alt](localURL)`，禁止输出 `data:image/*`。
2. `<pre><code>`：保留 fenced code block，尽量保留 language class。
3. `<table>`：优先输出 GFM table；复杂表格降级为 HTML 或文本，并记录 warning。
4. `<a>`：只输出已经完成 PlainDoc 链接重写的安全 URL。
5. `<sup>/<sub>`、脚注、EPUB 常见 `span`：按 PlainDoc 可编辑性优先降级。

现有 `convert_html_to_markdown.mjs` 保留为迁移参考，不作为 EPUB 导入首选路径。如果实施时发现 Go 库无法覆盖现有 HTML 导入质量，再单独评估是否保留 Node 脚本作为回退。

### 5.5 HTML 转 Markdown 性能验证

虽然 Go 库可以消除 Node 进程启动开销，Phase 2 仍必须补基准测试，至少覆盖：

1. 20 个章节、50 个章节、100 个章节。
2. 每章 10KiB、100KiB、1MiB HTML。
3. 串行转换总耗时、单章 P95 耗时、失败重试行为。

升级触发条件：

1. 50 个 100KiB 章节串行转换超过 10 秒。
2. 单章 P95 转换耗时超过 300ms。
3. 内存峰值超过导入任务可接受范围。

触发任一条件时，优先优化自定义规则和批量处理方式；如果仍无法满足，再评估以下回退：

1. 使用现有 Node 脚本作为特定标签的兼容回退。
2. 引入更专门的转换库或保留部分 HTML。
3. 对超大章节做分段转换。

### 5.6 EPUB 兼容性边界与升级条件

第一期目标是“标准 EPUB 可导入”，并尽量兼容主流工具生成的 EPUB。兼容性不足时不要无限 patch 解析器，需要有升级判断。

第一期兼容范围：

1. EPUB 2：`toc.ncx + OPF spine`。
2. EPUB 3：`nav.xhtml + OPF spine`。
3. XHTML 或可被 `golang.org/x/net/html` 容错解析的 HTML。
4. OPF/nav/toc 为 UTF-8；如果声明了其它编码，只做有限兼容并记录 warning。
5. `media-type` 不标准但文件扩展名明确时，允许按扩展名兜底识别。

触发重新评估第三方解析库的条件：

1. 手工收集的 20 本主流 EPUB 样本中，因解析器兼容性导致失败超过 3 本。
2. 同一类非标准结构在真实用户导入中出现 3 次以上。
3. 为兼容同一类 EPUB 需要在解析器中加入超过 2 个特例分支。
4. nav/toc/OPF 解析逻辑开始影响安全边界，导致路径清洗或资源限制难以维护。

### 5.7 是否引入第三方解析包

不建议新增 Go EPUB 容器解析依赖。

原因：

1. 标准 EPUB 本质是 zip + XML + XHTML，当前 Go 标准库和已有依赖足够覆盖。
2. 现有导出侧已经引入 `github.com/go-shiori/go-epub`，该包定位是创建 EPUB，不适合作为导入解析核心。
3. 解析器只需要读取 metadata、manifest、spine、nav/toc 和资源，不需要完整电子书阅读器能力。
4. 自研轻量解析器更容易加项目安全限制：entry 数、解压大小、路径清洗、图片大小、危险协议过滤。
5. HTML 转 Markdown 的复杂度和 EPUB 容器解析不同，应通过 `github.com/JohannesKaufmann/html-to-markdown/v2` 这类专门转换库解决，而不是引入 EPUB 容器解析库。

允许使用的现有能力：

1. `archive/zip`
2. `encoding/xml`
3. `golang.org/x/net/html`
4. `github.com/JohannesKaufmann/html-to-markdown/v2`

如果达到 5.6 中的升级条件，再评估引入专门 EPUB 解析库。第一期不需要。

---

## 6. 权限与安全

### 6.1 权限规则

EPUB 导入权限：

```text
CanImportEPUBSpace(actorUserID)
  -> actor 具备创建空间能力
```

实现上建议在 `AdminSpaceImportService` 中区分：

1. `.plaindoc` 导入：沿用现有 `CanImportSpace`。
2. `.epub` 导入：调用新能力判断 `CanCreateSpace` 或等价方法。

如果当前系统还没有独立的“创建空间能力”接口，需要补到 `AdminAccessService` 或空间服务侧，避免把 EPUB 导入权限硬编码成管理员角色。

### 6.2 安全限制

EPUB 处理必须满足：

1. `zip slip` 防护：拒绝 `../`、绝对路径、空路径和重复 entry。
2. 解压炸弹防护：EPUB 初始上限固定为 entry 数不超过 2000、总解压内容不超过 128MiB、单 entry 不超过 32MiB、目录深度不超过 16；该组数值与现有工作区 zip 导入限制保持一致，后续可根据真实样本单独调大。
3. HTML 清洗：移除脚本、表单和事件属性。
4. 协议过滤：拒绝 `javascript:`、`file:`、`data:text/html` 等危险链接；非 SVG `data:image/*` 只能走图片解码、本地化和大小限制流程。
5. 图片大小限制：单图最大 20MiB。
6. SVG 不作为 EPUB 导入图片类型支持，文件型和 `data:image/svg+xml` 均降级为 alt 文本，避免持久化未清洗 SVG。
7. 错误降级：单个图片失败不应导致整本 EPUB 导入失败；核心 OPF/spine/章节缺失才失败。

### 6.3 暂存 TTL 与清理

EPUB inspect 与 `.plaindoc` inspect 一样必须写入私有 staging 目录，并绑定 `importId + actorUserID`。

清理规则：

1. inspect 暂存默认 TTL 沿用现有空间导入 staging TTL；如果当前实现没有显式 TTL，EPUB 导入落地时必须补齐。
2. 用户 inspect 后未 commit，TTL 到期后后台清理循环删除原始 `.epub`、解析缓存和临时资源。
3. commit 成功后删除 staging 文件。
4. commit 失败后保留短期错误排查窗口，过期后同样清理。
5. 清理失败必须写结构化日志，但不能阻断其它任务清理。

---

## 7. 历史版本设计

### 7.1 后端接口

将现有版本接口拆成三个：

```text
GET /api/docs/:docId/revisions?page=1&pageSize=30
GET /api/docs/:docId/revisions/:revisionId
POST /api/docs/:docId/revisions/:revisionId/restore
```

摘要列表不返回正文内容。Markdown 修订来自 `document_revisions`，Office 文件修订来自 `document_file_revisions`，前端统一展示为同一种摘要模型：

```ts
export interface DocumentRevisionSummary {
  id: string;
  documentId: string;
  version: number;
  baseVersion: number;
  createdAt: string;
  source: "local" | "remote";
  format: "markdown" | "docx" | "xlsx";
  fileName?: string;
  mimeType?: string;
  editorUser?: {
    userId: string;
    displayName: string;
  };
}
```

列表 UI 必须直接展示：

1. 版本号：`version`。
2. 创建时间：`createdAt`。
3. 创建人：`editorUser.displayName`；缺失时显示“未知用户”或“系统导入”。

详情接口返回正文或文件版本元数据：

```ts
export interface DocumentRevisionDetail extends DocumentRevisionSummary {
  contentMd?: string;
  file?: {
    fileName?: string;
    mimeType?: string;
    blobId?: string;
  } | null;
}
```

当前实现进度：Task 5.1 已完成前端 `DocumentRevisionSummary` /
`DocumentRevisionDetail` 类型拆分和 `getRevisionDetail` gateway；后端列表接口已改为摘要响应，
不再返回 `contentMd`，并补充 `format` 与 `editorUser`。Task 5.2 已完成 repository 层
Markdown / Office revision 摘要分页、创建人关联、版本倒序稳定排序、单版本详情查询和
跨文档 revision not found 语义；摘要分页会将每个 revision 来源查询限制在 `offset+limit`
候选集内，再做跨来源合并排序，避免长历史文档每次翻页全量加载。Task 5.3 已完成 handler 层 `page/pageSize` 参数、
`pageSize` 上限保护、详情路由、读权限复用、Markdown 正文详情、Office 文件元数据详情和
跨文档 revision 防护。Phase 6 已完成恢复接口契约与 Markdown 历史版本恢复：
前端新增 `restoreRevision` gateway，后端注册
`POST /api/docs/:docId/revisions/:revisionId/restore`，恢复时要求调用方传当前
Markdown 文档的 `Document.version` 或 Office 文档的 `Document.contentVersion` 作为 `baseVersion`；
Markdown 恢复会校验写权限、当前文档版本和目标
revision 格式，以目标 revision 正文覆盖当前正文，递增文档版本并新增一条
`document_revisions`，同时复用阅读缓存清理和图片引用同步规则。Office 恢复也已完成：
后端会读取目标 `document_file_revisions`，校验当前文档格式，并按 Office 当前
`contentVersion` 做并发检查，用目标 file revision 的 `blobID/fileName/mimeType` 覆盖当前 source
文件引用，不重复写入 `file_blobs`，递增版本并新增一条 `document_file_revisions`。恢复后
`render_status` 会被标记为 `pending`，等待阅读 HTML 重新渲染。

恢复接口请求体：

```ts
export interface RestoreDocumentRevisionInput {
  docId: string;
  revisionId: string;
  // Markdown 恢复传当前 Document.version。
  // Office 恢复传当前 Document.contentVersion。
  baseVersion: number;
}
```

恢复接口响应体沿用保存文档结果：

```ts
export interface RestoreDocumentRevisionResult {
  document: Document;
  restoredFromRevision: DocumentRevisionSummary;
}
```

权限规则：

1. 列表和详情仍然使用 `visibilityService.GetDocument(ctx, documentID, actorUserID)`。只要能读取当前文档，就能读取该文档历史版本。
2. 恢复操作必须使用空间写权限，权限边界与 `SaveDocument` / Office 保存回调一致。
3. 恢复操作支持 Markdown、docx、xlsx；Markdown 恢复正文，Office 恢复 source blob 引用。
4. 恢复接口错误响应沿用统一 `JsonResult`，保留 `requestID` 便于排查；成功恢复会输出结构化日志，记录 `documentID`、`revisionID`、`actorUserID`、`baseVersion` 和新版本号。

恢复语义：

1. 后端读取目标历史版本。
2. 校验当前文档格式与目标版本格式一致。
3. 前端按当前文档格式传 `baseVersion`：Markdown 传 `Document.version`；Office 传 `Document.contentVersion`。后端按当前文档格式决定实际冲突字段：Markdown 校验 `documents.version`；Office 校验 `documents.content_version`；任一不一致都返回版本冲突，并带最新文档。
4. Markdown：以目标 revision 的 `contentMd` 覆盖当前正文。
5. Office：以目标 file revision 的 `blobID/fileName/mimeType` 覆盖当前 source 文件引用，不重复写入文件 blob。
6. 将当前文档版本递增，写入新的 `document_revisions` 或 `document_file_revisions` 记录。
7. 新 revision 的 `baseVersion` 为恢复前当前版本，`editorUserID` 为执行恢复的用户，`source` 首期仍使用 `remote`。
8. 清理阅读渲染缓存并同步文档图片引用；Office 恢复后需要触发或标记阅读 HTML 重新渲染。

恢复不会删除、修改或重排旧 revision。恢复到旧版本后，系统只会新增一个“恢复后的新版本”，便于继续审计和再次恢复。

`RevisionSource` 首期不新增 `restore` 枚举值。原因是当前三套数据库迁移都用 `source IN ('local', 'remote')` 约束，新增枚举会引入迁移和兼容成本。本文中 `remote` 表示“由已认证用户通过服务端写入的版本”，普通保存和恢复都属于该范围。如果后续产品需要在版本列表中明确标识“由恢复操作生成”，再单独设计 `operation` 字段或扩展 `RevisionSource`。

### 7.2 前端交互

在编辑器 `header-actions` 增加 `History` 图标按钮：

```text
附件
目录
预览模式
主题
历史版本
```

弹窗布局：

```text
┌────────────────────────────────────────────┐
│ 历史版本                                   │
├──────────────┬─────────────────────────────┤
│ v12 当前版本 │ 当前内容 ↔ v8                │
│ 2026-05-17   │                             │
│ 张三          │ 只读差异视图                 │
│              │                             │
│ v8 选中       │ [恢复到此版本]               │
└──────────────┴─────────────────────────────┘
```

状态：

1. 加载中：列表区域展示 loading。
2. 空状态：无历史版本时展示“暂无历史版本”。
3. 错误态：展示错误信息和重试按钮。
4. 长文档：右侧 diff 区域独立滚动，不撑破弹窗。
5. 当前文档有未保存内容时，恢复按钮禁用，并提示先保存或放弃当前编辑。
6. 点击“恢复到此版本”必须弹出二次确认，确认文案包含目标版本号、创建时间和创建人。
7. 恢复成功后关闭确认态，刷新版本列表和当前编辑器内容，并更新底部保存状态。
8. 版本列表使用分页加载：初始加载 30 条，滚动到底部或点击“加载更多”请求下一页。
9. Office 文档弹窗右侧不展示文本 diff，改为展示文件名、MIME、版本号、创建时间、创建人和“不支持二进制差异”的说明；恢复按钮保留，但二次确认需提示会切换当前 Office 源文件版本。

当前实现进度：Task 7.1 已完成顶栏历史版本入口和受控弹窗壳层。
入口放在附件、目录、预览模式、主题之后，使用与现有顶栏一致的 34px 图标按钮、tooltip 和
`aria-label`；未选中文档时按钮禁用，点击入口只打开弹窗，不提前触发版本列表请求。

Task 7.2 已完成历史版本弹窗基础状态：弹窗通过 `DataGateway.document.listRevisions(docId, { page,
pageSize: 30 })` 加载版本摘要，初始请求 30 条，滚动到底部或点击“加载更多”会请求下一页；前端按
revision ID 去重追加，保留后端返回顺序。左侧列表展示版本号、创建时间和创建人，创建人缺失时展示
“未知创建人”；列表区覆盖 loading、空状态、错误态和重试按钮。右侧详情区当前展示选中版本摘要，并
独立滚动，后续 Task 7.3/7.4 继续接入 Markdown diff 和 Office 元数据详情。

Task 7.3 已完成 Markdown diff 视图：前端新增 `@codemirror/merge`，选中 Markdown 历史版本后通过
`DataGateway.document.getRevisionDetail(docId, revisionId)` 拉取详情正文，并用 CodeMirror
`MergeView` 将历史 `contentMd` 与当前编辑器 `content` 做左右只读对比。diff 视图启用行号、Markdown
高亮、行换行、独立滚动、未变更折叠和 `diffConfig` 超时保护；切换版本时通过详情请求序号忽略过期
响应，避免旧响应覆盖新选中版本。当前文档存在未保存内容时仍展示 diff，且对比对象仍是编辑器当前
`content`；恢复按钮暂保持禁用，等待 Task 7.5 接入二次确认和恢复 API 调用。

Task 7.4 已完成 Office 历史版本展示：选中 docx/xlsx revision 时同样通过
`DataGateway.document.getRevisionDetail(docId, revisionId)` 拉取详情，但右侧不创建 CodeMirror
`MergeView`，只展示 Office 文件版本元数据。当前 UI 展示文件名、MIME、版本号、创建时间、创建人和
“Office 文档暂不支持二进制差异预览。”说明；恢复入口与 Markdown 保持一致，确认文案会明确提示
Office 恢复会切换当前源文件版本。

Task 7.5 已完成恢复确认与成功状态刷新：点击“恢复到此版本”会在弹窗内打开二次确认区域，确认文案
包含目标版本号、创建时间和创建人；当前文档存在未保存修改时恢复按钮禁用，并提示先保存或放弃当前
编辑。确认恢复时前端调用 `DataGateway.document.restoreRevision`，Markdown 传入当前
`Document.version`，Office 传入当前 `Document.contentVersion` 作为 `baseVersion`。恢复成功后弹窗关闭确认态、刷新版本列表第一页，并通过 `App` 同步当前
`content`、`Document.version`、`Document.contentVersion`、`lastSavedAt`、Office source 元数据和保存状态，避免恢复后的内容被
误判为本地未保存。Office 恢复成功后会显式重载 ONLYOFFICE 编辑配置，确保编辑器使用恢复后的
source blob / document key。恢复失败时弹窗保持打开并展示错误；版本冲突会提示用户刷新或重新选择历史版本。

### 7.3 Diff 依赖选择

推荐引入：

```text
@codemirror/merge
```

理由：

1. 项目已经使用 `@uiw/react-codemirror` 和 CodeMirror 6。
2. merge 视图可复用 CodeMirror 的只读编辑器能力。
3. 长 Markdown 文档的滚动、选中、行级渲染更稳。
4. 比手写 unified diff 更容易保持可维护性。

备选：

```text
diff
```

该包更轻，但需要自己实现行号、折叠、滚动、样式和大文本性能处理。除非强烈要求减少依赖，否则不推荐。

---

## 8. 前端改动清单

1. `apps/web/src/data-access/types.ts`
   - 扩展 `AdminSpaceImportInspectResult.packageType`。
   - 增加 EPUB summary 字段和 `sourcePublishedAt` / `sourceAuthors` 预览字段。
   - 拆分 `DocumentRevisionSummary` 和 `DocumentRevisionDetail`。
   - 新增 `RestoreDocumentRevisionInput` 和 `RestoreDocumentRevisionResult`。
2. `apps/web/src/data-access/http/adapter.ts`
   - 兼容 `.epub` inspect/commit。
   - 新增 `getRevisionDetail`。
   - 新增 `restoreRevision`。
3. `apps/web/src/admin/components/AdminSpaceImportDialog.tsx`
   - `accept` 改为 `.plaindoc,.epub`。
   - EPUB 预览文案展示作者、出版日期、章节数、目录层级、图片数和 warnings。
4. `apps/web/src/App.tsx`
   - 新增历史版本按钮。
   - 接入历史版本弹窗。
   - 恢复成功后同步 `content`、`activeDocument.version`、`activeDocument.contentVersion`、`lastSavedAt` 和保存状态。
5. 新增 `apps/web/src/components/DocumentRevisionHistoryDialog.tsx`
   - 负责版本列表、详情加载、diff 渲染、恢复确认和错误态。
   - 版本列表项展示版本号、创建时间和创建人。
   - 版本列表支持分页加载，避免一次性渲染大量历史记录。
   - 当前有未保存内容时禁用恢复入口。
   - Markdown 版本展示 diff；Office 版本展示文件元数据和恢复说明。

---

## 9. 后端改动清单

1. `apps/server/internal/server/handler/admin_space_import.go`
   - inspect 错误映射支持 EPUB。
2. `apps/server/internal/service/admin_space_import_service.go`
   - 导入暂存记录增加 `PackageType`。
   - commit 根据 package type 分发 `.plaindoc` 或 `.epub` restore。
   - EPUB commit 使用创建空间权限。
3. `apps/server/internal/service/admin_space_epub_importer.go`
   - 新增 EPUB 解析、目录构建、资源收集和 XHTML 清洗。
4. `apps/server/internal/service/html_markdown_converter.go`
   - 基于 `github.com/JohannesKaufmann/html-to-markdown/v2` 新增 service 层 HTML 转 Markdown 组件。
   - 增加 PlainDoc 自定义转换规则，覆盖图片、代码块、表格、链接、脚注和 EPUB 常见标签。
5. `apps/server/go.mod` / `apps/server/go.sum`
   - 新增 `github.com/JohannesKaufmann/html-to-markdown/v2` 依赖；实施前必须单独确认依赖变更。
6. `apps/server/internal/storage/repository/interfaces.go`
   - 如需分页版本摘要，补充 revision 查询参数结构。
7. `apps/server/internal/storage/repository/gorm_workspace_repository.go`
   - 增加 `ListRevisionSummariesByDocumentID` 和 `GetRevisionByRevisionID`。
   - 增加 Office 文件版本摘要和详情查询。
   - 列表查询需要关联用户表，返回创建人 ID 和展示名。
8. `apps/server/internal/server/handler/workspace.go`
   - 拆分历史版本列表和详情接口。
   - 新增恢复指定版本接口。
   - Markdown 恢复已复用 `SaveDocument` 的版本冲突、缓存清理和图片引用同步规则。
   - Office 恢复已复用 `SaveOfficeDocument` 的版本递增、file revision 和阅读渲染失效规则。
9. `apps/server/internal/server/router.go`
   - 注册 `GET /docs/:docId/revisions/:revisionId`。
   - 注册 `POST /docs/:docId/revisions/:revisionId/restore`。

---

## 10. 测试计划

### 10.1 EPUB 导入后端测试

1. 标准 EPUB3：`nav.xhtml + content.opf + spine`。
2. 标准 EPUB2：`toc.ncx + content.opf + spine`。
3. 无 nav/toc：按 spine 生成扁平目录。
4. 嵌套目录：导入后生成 folder/doc 层级。
5. 目录项自身有正文且有子项：生成 folder + 正文 doc。
6. XHTML 转 Markdown：标题、段落、列表、表格、代码块、链接。
7. EPUB 内部链接重映射：同文件、跨文件、缺失目标都按规则处理。
8. nav/toc fragment：同文件不同 fragment 可拆分，无法定位时记录 warning。
9. 图片本地化：相对路径图片和非 SVG `data:image/*` 写入本地资源，Markdown 链接被改写。
10. SVG 安全：文件型 SVG 与 `data:image/svg+xml` 均降级为 alt 文本，不写入本地 blob。
11. 非法 EPUB：缺少 mimetype、container、OPF、spine。
12. 安全场景：zip slip、重复 entry、超大 entry、超多 entry、危险协议。
13. 权限场景：有创建空间权限可导入；无创建空间权限拒绝。
14. 暂存清理：inspect 后放弃 commit，TTL 到期后清理 staging 文件。
15. 转换性能：基于 Go HTML 转 Markdown converter 跑 20/50/100 章节基准测试。
16. 兼容性样本：至少收集 EPUB2、EPUB3、Calibre 转换样本和一个非标准 `media-type` 样本。

### 10.2 历史版本测试

1. 列表接口不返回正文。
2. 列表接口返回版本号、创建时间和创建人。
3. 详情接口返回指定版本正文。
4. 无文档权限时列表和详情都拒绝。
5. 无写权限时恢复接口拒绝。
6. 恢复接口在 `baseVersion` 过期时返回版本冲突。
7. Markdown 恢复指定版本后，当前文档内容等于目标 revision 内容，版本号递增，并新增一条 revision。
8. 恢复操作不会修改旧 revision。
9. Office 恢复指定版本后，当前 source blob/fileName/mimeType 等于目标 file revision，版本号递增，并新增一条 file revision。
10. 不存在 revision 返回 404。
11. 前端弹窗加载、空状态、错误态和版本切换。
12. 当前未保存内容参与 diff，对比对象为编辑器当前 `content`。
13. 前端在未保存状态下禁用恢复按钮。
14. 恢复确认文案包含版本号、创建时间和创建人。
15. 版本列表分页加载下一页，不重复、不乱序。
16. Office 历史弹窗展示文件元数据，不展示文本 diff。

### 10.3 标准验证命令

按仓库约定执行：

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

---

## 11. 实施阶段

### Phase 1：EPUB inspect

- [x] 新增 EPUB package type。
- [x] 解析 mimetype、container、OPF、nav/toc。
- [x] 返回 EPUB 空间预览摘要。
- [x] 前端导入弹窗支持 `.epub`。

### Phase 2：EPUB commit

- [x] 新增 EPUB restore 流程。
- [x] 创建新空间和目录树。
- [x] 建立 `targetKey -> documentID` 与 `canonicalHref -> primary documentID` 映射，并在 commit 写库阶段结合真实 `spaceID` 回写 EPUB 内部链接。
- [ ] 定义并实现 fragment 拆分与降级策略。
- [x] XHTML 清洗并转换 Markdown。
- [x] 图片资源本地化。
- [x] 支持非 SVG `data:image/*` 本地化；SVG 图片统一降级为 alt 文本。
- [x] 引入并封装 `github.com/JohannesKaufmann/html-to-markdown/v2`，补 PlainDoc 自定义转换规则。
- [x] 完成 HTML 转 Markdown 批量性能基准，必要时优化规则、分段转换或保留 Node 脚本回退。
- [x] 补齐 staging TTL 和清理策略。
- [x] SSE 进度接入。

### Phase 3：历史版本接口

- [ ] 版本列表改为摘要分页。
- [ ] 版本摘要包含版本号、创建时间和创建人。
- [ ] 新增单版本详情接口。
- [ ] 新增 Markdown/Office 恢复指定版本接口，恢复时生成新版本。
- [ ] 补权限和仓储测试。

### Phase 4：历史版本弹窗

- [x] 新增历史版本 icon 按钮。
- [x] 新增弹窗和版本列表。
- [x] 版本列表展示版本号、创建时间和创建人。
- [x] 版本列表支持分页加载。
- [x] 接入 `@codemirror/merge` 差异视图。
- [x] Office 历史弹窗展示文件元数据和恢复入口，不展示文本 diff。
- [x] 增加恢复到指定版本的二次确认和成功后状态刷新。
- [x] 补前端单测和构建验证。

### Phase 5：文档同步与回归

- [x] 更新 `BACKEND_DEVELOPER_GUIDE.md`。
- [x] 更新 `FRONTEND_DEVELOPER_GUIDE.md`。
- [x] 更新空间导入导出执行清单。
- [ ] 完成后端、前端和手工 EPUB 样本验证。
