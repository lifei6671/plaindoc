# ONLYOFFICE 一等文档集成技术方案

**文档状态**: Completed  
**创建日期**: 2026-03-06  
**完成日期**: 2026-03-08  
**适用范围**: `apps/server`、`apps/web`、`docs`  
**目标**: 在不走“附件编辑”模型的前提下，引入 Word/Excel 一等文档能力，支持工作区编辑、前端阅读页、分享阅读页；同时明确其不参与全文搜索与 sitemap。

---

## 1. 需求边界

### 1.1 本期目标

1. Office 文档以“一等文档”进入空间树，不复用 `document_attachments` 作为正文来源。
2. Office 文档支持三种模式：
   - 工作区编辑
   - 前端阅读页 `GET /r/:spaceId/:docKey`
   - 分享阅读页 `GET /s/:spaceId/:docKey`
3. 编辑页面左侧“新建文档”子菜单需要新增两个入口：
   - `新建 Word`
   - `新建表格`
4. Office 文档不参与：
   - 全文索引构建
   - 首页/空间检索结果
   - sitemap 生成
5. Office 文档与 Markdown 文档共享：
   - 空间树
   - 权限模型
   - 文档标识（`reader_slug` / route key）
   - 文档分享能力

### 1.2 本期建议约束

1. 首期只把 `docx` 与 `xlsx` 作为稳定编辑格式。
2. `doc` / `xls` 若要支持，建议在导入或首次打开前转换为 `docx` / `xlsx`。
3. Markdown 专属能力默认不适用于 Office：
   - Markdown 模板
   - Markdown 主题
   - 外链图片转存
   - Markdown 导出
   - 基于 Markdown 的 TOC、滚动同步、微信导出
4. 公开 Office 阅读页与分享页建议默认 `noindex`，避免“可访问但不进 sitemap”的 SEO 边界不一致。

---

## 2. 方案前提

### 2.1 格式字段前提

本文按“节点/文档层已经存在，或将新增，一个格式判别字段”来评估，字段可用于区分：

1. `markdown`
2. `docx`
3. `xlsx`

为降低现有代码改面，**推荐保持 `nodes.type` 继续表达结构语义（`folder/doc`），把格式字段放到 `documents` 或单独的内容表中**。  
如果当前分支已经把格式字段放在节点层，本文中的“文档格式字段”可映射到你现有实现；若主干库尚无该字段，则 Phase 0 先补齐迁移与模型。

### 2.2 阅读与分享前提

本方案明确要求：

1. Office 文档要有正常阅读页。
2. Office 文档要支持分享阅读。
3. Office 阅读与分享阅读均使用 ONLYOFFICE 的只读模式，不要求把 Office 内容转成 Markdown 或 SSR 为 HTML 正文。

---

## 3. 总体设计

### 3.1 文档内容模型

建议把“文档结构”和“文档内容格式”拆开：

1. `nodes`
   - 仍负责目录树层级、排序、`reader_slug`
   - `type` 继续只区分 `folder/doc`
2. `documents`
   - 继续是一等文档主表
   - 新增格式判别字段，例如 `format`
   - `markdown` 文档继续使用 `content_md`
   - `docx/xlsx` 文档改为引用“正文文件 blob”
3. `file_blobs`
   - 可复用现有物理文件抽象，承载 Office 正文文件
4. Office 文档版本
   - 需要独立的版本号/会话 key 轮换字段，供 ONLYOFFICE `document.key` 使用
5. 修订历史
   - 现有 `document_revisions.content_md` 仅适合 Markdown
   - Office 建议新增独立的 `document_file_revisions`

### 3.2 阅读与分享渲染模式

阅读页与分享页改为双模式：

1. Markdown
   - 继续复用当前阅读 SSR：输出 Markdown HTML 正文
2. Office
   - SSR 只输出页面壳、元信息、容器节点、状态数据
   - 浏览器端挂载 ONLYOFFICE DocEditor，只读模式
   - 不渲染 Markdown TOC、导出 Markdown 按钮

这样可以避免为了阅读/分享去做 Office -> HTML 转换，也不会侵入现有 Markdown 阅读链路。

### 3.3 ONLYOFFICE 集成模式

按标准 Docs API 集成：

1. 工作区打开 Office 文档
   - 前端调用“生成编辑配置”接口
   - 后端校验权限，返回 ONLYOFFICE config JSON
   - 浏览器加载 Document Server `api.js`
   - 以编辑模式挂载 DocEditor
2. 阅读页打开 Office 文档
   - 调用只读配置接口
   - 仅允许 view / comment / download 等受控能力
3. 分享页打开 Office 文档
   - 调用分享态只读配置接口
   - 只读，不允许回调写回
4. 编辑保存
   - Document Server 按 callback 协议回调后端
   - 后端下载新文件、写入 blob、新增 file revision、更新文档当前版本与版本 key

### 3.4 搜索与 sitemap 规则

Office 文档统一按格式过滤：

1. 不进入索引源查询
2. 不进入 SQL fallback 搜索
3. 不进入首页搜索 metadata 汇总
4. 不进入 sitemap 列表
5. 可访问的公开阅读页建议加 `noindex`

---

## 4. 改造清单

### 4.1 数据库与模型

- [x] 为文档引入格式字段：`markdown/docx/xlsx`
- [x] 为 Office 文档引入正文文件引用字段：如 `source_blob_id`、`source_file_name`、`source_mime_type`
- [x] 为 Office 文档引入 `content_version` 或等价版本 key 字段
- [x] 新增 Office 修订表：如 `document_file_revisions`
- [x] 为 Markdown 旧数据补默认格式 `markdown`
- [x] 若主干代码尚无格式字段，补三套迁移：`sqlite/mysql/postgres`
- [x] 更新 Go 模型：
  - `apps/server/internal/storage/models/document.go`
  - `apps/server/internal/storage/models/document_revision.go` 或新增 `document_file_revision.go`
- [x] 更新前端契约：
  - `apps/web/src/data-access/types.ts`

### 4.2 仓储与服务层

- [x] 让以下仓储查询统一返回文档格式字段：
  - [x] `apps/server/internal/storage/repository/gorm_workspace_repository.go`
  - [x] `apps/server/internal/storage/repository/gorm_document_repository.go`
  - [x] `apps/server/internal/storage/repository/gorm_reader_page_repository.go`
  - [x] `apps/server/internal/storage/repository/gorm_document_share_repository.go`
- [x] 新增 Office 文档正文读取/写回仓储接口
- [x] 新增 Office 文档版本写回事务：
  - 下载 ONLYOFFICE 输出文件
  - 写入 blob
  - 写入 file revision
  - 更新 `documents.current_blob_id + content_version + updated_at`
- [x] 删除文档时补 Office 正文文件生命周期清理
- [x] 让权限服务按“文档是否可读/可写”工作，不与 Markdown 耦合

### 4.3 工作区后端 API

- [x] `CreateNode` 增加格式入参
- [x] `GetDocument` 增加格式判别字段
- [x] `SaveDocument` 明确只对 `markdown` 可用；Office 文档请求直接拒绝
- [x] 新增工作区 ONLYOFFICE 编辑配置接口
- [x] 新增工作区 ONLYOFFICE callback 接口
- [x] 新增 Office 正文文件受控访问接口，供 Document Server 拉取
- [x] 文档重命名、标识、可见性接口保持复用，但要兼容 Office 文档
- [x] Markdown 专属接口加格式保护：
  - 远程图片本地化
  - Markdown 主题
  - Markdown 模板初始化

### 4.4 工作区前端

- [x] 左侧“新建文档”子菜单新增：
  - `新建 Word`
  - `新建表格`
- [x] 创建节点弹窗增加格式选择：`Markdown / Word / Excel`
- [x] 空间树节点增加格式标识与图标
- [x] `useWorkspace.openDocument()` 改为 format-aware
- [x] `App.tsx` 编辑工作台拆成双模式：
  - Markdown：现有 CodeMirror + Preview
  - Office：DocEditor 容器
- [x] Office 模式隐藏 Markdown 工具栏、预览区、TOC、滚动同步、外链图片转存
- [x] Office 模式不走现有自动保存与冲突提示 footer
- [x] 为 Office 编辑页补加载态、回源失败态、权限拒绝态
- [x] 在前端对 ONLYOFFICE script 加载失败与 session 过期做明确提示

### 4.5 阅读页与分享阅读页

- [x] `ReaderPageService` 增加格式判别，按格式构建不同 view model
- [x] `DocumentShareService` 增加格式判别，按格式构建不同分享页 view model
- [x] `apps/web/src/ssr/render-space-reader.tsx` 改为双模式：
  - Markdown 输出 HTML 正文
  - Office 输出阅读容器与只读状态数据
- [x] `apps/web/src/ssr/render-space-reader.async-script.ts` 增加 Office 只读挂载逻辑
- [x] 分享页复用 Office 只读模式，不允许编辑写回
- [x] Office 阅读页/分享页隐藏 Markdown 导出与 PDF 打印按钮
- [x] Office 阅读页/分享页保留下载原文件能力
- [x] 公开 Office 阅读页与分享页增加 `robots noindex` 配置

### 4.6 搜索与 sitemap 排除

- [x] 在索引源查询中过滤非 Markdown：
  - [x] `apps/server/internal/storage/repository/gorm_search_index_source_repository.go`
- [x] 在 SQL fallback 搜索中过滤非 Markdown：
  - [x] `apps/server/internal/storage/repository/gorm_search_visibility_repository.go`
- [x] 在首页搜索 metadata 汇总中过滤非 Markdown：
  - [x] `apps/server/internal/storage/repository/gorm_home_search_repository.go`
- [x] 在 sitemap 源查询中过滤非 Markdown：
  - [x] `apps/server/internal/storage/repository/gorm_sitemap_repository.go`
- [x] `apps/server/internal/service/sitemap_service.go` 保持二次保护，避免误入 sitemap

### 4.7 首页与阅读入口联动

- [x] 若 Office 文档具备阅读页，则首页空间可见性逻辑可继续把 Office 文档视作“可读文档”
- [x] `ReaderPageService.ResolveLandingDocumentID()` 需要按格式选择首篇可读文档
- [x] 若一个空间首篇是 Office 文档，`/r/:spaceId` 自动跳转也应正常进入 Office 阅读页
- [x] 本期收口：后台文档列表已增加格式标识；首页卡片格式标识不纳入本期实现范围

### 4.8 运营、审计与后台治理

- [x] 后台文档列表增加格式字段/筛选
- [x] 审计日志补充 Office 文档编辑回写记录
- [x] 本期收口：管理端不新增按格式区分的批量操作入口；Markdown 专属能力继续由接口侧格式保护
- [x] 系统配置中预留 ONLYOFFICE 连接参数：
  - Document Server 地址
  - JWT 密钥
  - 回调外部可达地址

### 4.9 测试与文档

- [x] 迁移测试：三库结构与回填数据验证
- [x] 仓储测试：Office 正文回写事务、版本递增、清理补偿
- [x] Handler/Service 测试：
  - [x] 创建 Office 文档
  - [x] 工作区编辑配置生成
  - [x] callback 成功/失败
  - [x] Office 阅读页
  - [x] Office 分享阅读页
  - [x] 搜索与 sitemap 排除
- [x] 前端测试：
  - [x] 创建弹窗格式选择
  - [x] 工作区格式切换
  - [x] 阅读/分享只读挂载
  - [x] Markdown 功能按格式禁用
- [x] 更新 `docs/README.md` 索引
- [x] 后续把最终结果并入 `BACKEND_DEVELOPER_GUIDE.md` 与 `FRONTEND_DEVELOPER_GUIDE.md`

---

## 5. 建议实施顺序

### Phase 0：结构准备（已完成）

1. 补格式字段与 Office 正文文件模型
2. 补版本字段与 file revision 表
3. 完成三库迁移与主干数据回填

### Phase 1：工作区编辑（已完成）

1. 工作区创建 Office 文档
2. 获取 Office 文档详情
3. ONLYOFFICE 编辑配置接口
4. callback 写回
5. 前端工作区切换到 Office 编辑器

### Phase 2：阅读与分享阅读

1. Office 阅读页容器与只读挂载
2. 分享页只读挂载
3. 下载原文件能力
4. Markdown 导出/PDF 按格式屏蔽

### Phase 3：排除搜索与 sitemap

1. 索引源过滤
2. SQL fallback 过滤
3. 首页搜索过滤
4. sitemap 过滤
5. `noindex` 策略与回归测试

### Phase 4：后台治理与收口

1. 后台文档列表格式可视化
2. 审计补齐
3. 运维参数配置化
4. 开发文档并入主指南

---

## 6. 关键设计决策

### 6.1 是否继续复用 `content_md`

建议：

1. `markdown` 文档继续使用 `content_md`
2. `docx/xlsx` 文档把 `content_md` 置空
3. Office 正文以 blob 引用为准

### 6.2 是否复用现有 `document_revisions`

不建议直接复用原表存 Office 正文：

1. 当前表语义是“Markdown 内容快照”
2. Office 正文是二进制文件，字段语义不匹配
3. 新增 `document_file_revisions` 更清晰，也更容易和 ONLYOFFICE callback 对齐

### 6.3 是否让 Office 阅读页参与 SEO

默认建议：

1. 不进 sitemap
2. 页面加 `noindex`
3. 仍允许公开访问，但不作为 SEO 内容页经营

---

## 7. 风险与注意事项

1. 真正的大头不是搜索/sitemap，而是编辑态与阅读态的分叉。
2. 如果把“格式”继续塞进 `NodeType`，当前大量 `type == doc` 代码都会需要审计，风险高于新增独立格式字段。
3. ONLYOFFICE callback 写回必须走事务，避免只更新 blob 未更新文档版本。
4. 阅读页和分享页一旦支持 Office，就不再是纯 Markdown SSR；前端脚本与 SSR worker 都要格式感知。
5. 如果后续还要支持 `pptx`，当前方案可平移，但 UI 与阅读能力要再补一个 presentation 分支。

---

## 8. 结论

在“不走附件、支持工作区编辑、支持阅读页与分享阅读、但排除搜索与 sitemap”的前提下，本方案**可行**，且推荐路径明确：

1. 保留现有目录树与权限模型。
2. 给文档引入独立格式字段与正文文件引用。
3. 工作区、阅读页、分享页改为 Markdown / Office 双模式。
4. 搜索与 sitemap 在查询源头按格式排除 Office。

该方案属于**中到大改**；若阅读页与分享页都要在一期完成，整体更接近“大改”。
