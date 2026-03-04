# 阅读页文档自定义标识（SEO URL）技术方案

**文档状态**: Draft（待评审）  
**创建日期**: 2026-03-04  
**适用范围**: `apps/server`、`apps/web`（阅读页 SSR、编辑器文档树、首页/检索/sitemap 链接）  
**目标**: 在不影响现有功能的前提下，为文档引入可自定义阅读标识，替代现有 URL 中的 `document_id`（ULID）作为对外主路由键。

---

## 1. 背景与问题

当前阅读页路由形态为：

1. 空间入口：`/r/:spaceId`
2. 文档页：`/r/:spaceId/:docId`

其中 `:docId` 主要使用 `document_id`（ULID），技术上稳定，但对 SEO 与可读性不友好。  
目标能力是允许用户在文档创建/编辑阶段设置“文档标识”（如 `quick-start`），使阅读页 URL 更可读：

1. `旧`: `/r/my-space/01jx...`
2. `新`: `/r/my-space/quick-start`

---

## 2. 设计原则（必须满足）

1. **兼容优先**：旧链接必须继续可访问，不可一次性破坏外部已传播 URL。
2. **存量无感**：文档正文、修订、权限、附件等核心链路继续以 `document_id` 作为内部主键，不替换存储模型。
3. **渐进切换**：新增 slug 解析能力后，再逐步把首页、搜索、sitemap、SSR 树链接切到新 key。
4. **唯一可控**：文档标识在“同空间内”唯一，跨空间可重复。
5. **可回滚**：关闭新标识或清空标识后，系统仍可回落到 `document_id` 链路。

---

## 3. 总体方案

采用“新增 `reader_slug`，路由双解析，canonical 统一到 slug”的增量方案：

1. 在 `nodes` 表新增 `reader_slug`（仅文档节点使用）。
2. 路由仍保持 `/r/:spaceId/:docKey`，`docKey` 支持三种输入：
   - `document_id`
   - `node_id`（兼容历史）
   - `reader_slug`（新能力）
3. 当文档存在 `reader_slug` 时，阅读页 canonical 与页面内树链接统一输出 slug URL。
4. 旧 `document_id` 链接继续可访问，但 301/303 重定向到 slug canonical。

---

## 4. 数据模型与迁移

## 4.1 字段设计

在 `nodes` 增加字段：

1. `reader_slug`：`VARCHAR(120)`（SQLite 为 `TEXT`），可空。
2. 语义：阅读页公开标识，仅对 `type='doc'` 的节点生效。

## 4.2 约束与索引

1. 唯一约束：`UNIQUE(space_id, reader_slug)`（允许 `NULL` 重复）。
2. 检索索引：`INDEX(space_id, reader_slug)`（查询解析用）。
3. 可选约束：`CHECK(reader_slug IS NULL OR reader_slug <> '')`（防空串）。

> 说明：若数据库方言不便直接表达“仅 doc 节点唯一”，可先使用 `(space_id, reader_slug)` 唯一约束，业务层保证 folder 不写入 slug。

## 4.3 迁移范围

同步补齐三套迁移：

1. `sqlite`
2. `mysql`
3. `postgres`

并更新对应模型：

1. `models.Node` 新增 `ReaderSlug *string`
2. 涉及树查询、阅读页查询、首页/sitemap 元信息查询的仓储 row 结构同步带出 `reader_slug`

---

## 5. 路由解析与 canonical 策略

## 5.1 阅读页解析顺序

`ResolveDocumentID(spaceID, rawDocKey)` 调整为：

1. 优先按 `document_id` 匹配
2. 其次按 `node_id` 匹配
3. 最后按 `reader_slug`（限定同 `space_id`）匹配

## 5.2 canonical 规则

对同一文档定义 `canonicalDocKey`：

1. 若 `reader_slug` 非空：`canonicalDocKey = reader_slug`
2. 否则：`canonicalDocKey = document_id`

当请求 URL 的 `docKey != canonicalDocKey` 时：

1. 重定向到 `/r/:spaceId/:canonicalDocKey`
2. 阅读 SSR `<link rel="canonical">` 使用 canonical URL

## 5.3 缓存键调整

阅读页渲染缓存建议继续按 `document_id` 作为主键，不受 slug 变更影响；  
slug 仅用于入口 URL 与 canonical，不参与正文缓存实体 ID。

---

## 6. API 设计

## 6.1 创建节点接口扩展

现有：

1. `POST /api/spaces/:spaceId/nodes`

新增可选入参：

1. `documentIdentifier`（仅 `type=doc` 时生效）

行为：

1. 未传：保持旧行为（仅生成 ULID 文档 ID）。
2. 传入有效值：创建时写入 `nodes.reader_slug`。

## 6.2 更新文档标识接口（新增）

新增：

1. `PATCH /api/docs/:docId/identifier`

请求体建议：

1. `{ "identifier": "quick-start" }`（设置）
2. `{ "identifier": "" }` 或 `{ "identifier": null }`（清空）

权限：

1. 空间写权限（owner/collaborator）

返回：

1. `documentId`
2. `identifier`（最新值）
3. `readerURL`（可选，便于前端提示）

---

## 7. 标识规范与校验

建议规则：

1. 仅允许：`a-z`、`0-9`、`-`
2. 长度：`1~80`
3. 不能以 `-` 开头或结尾
4. 连续 `--` 允许与否可配置（建议允许，避免过严）
5. 同空间唯一（冲突返回明确错误码）
6. 保留词禁用：`api`、`admin`、`login`、`register`、`search`、`explore` 等

建议新增错误码：

1. `DOCUMENT_IDENTIFIER_INVALID`
2. `DOCUMENT_IDENTIFIER_CONFLICT`
3. `DOCUMENT_IDENTIFIER_RESERVED`

---

## 8. 前端改造（编辑器 + 阅读页）

## 8.1 编辑器文档树交互

入口：`WorkspaceTree` 节点右侧 `+` 菜单。

新增菜单项：

1. `新建子文档（自定义标识）`
2. `设置文档标识`（仅文档节点显示）

交互形式：

1. 使用项目内弹层/对话框（禁止 `window.prompt`）。
2. 创建文档时可选填写 `标题 + 标识`。
3. 已有文档可单独修改标识。

## 8.2 前端数据结构

树节点补充字段（建议）：

1. `documentIdentifier?: string`
2. `documentRouteKey?: string`（优先 slug，兜底 documentId）

阅读链接生成统一使用 `documentRouteKey`，避免前端多处重复判断。

## 8.3 阅读页 SSR 与异步切换

阅读页树链接、canonical、history pushState 均输出 canonical key（slug 或 document_id）。  
SSR 异步脚本仍按 `href` 同源加载，不需要改协议。

---

## 9. 首页、搜索、sitemap 联动

需要统一改为“路由 key”输出，避免链路混用：

1. 首页卡片跳转（空间入口可保持 `/r/:spaceId`）。
2. 首页搜索命中文档 URL：`/r/:spaceId/:documentRouteKey`。
3. sitemap 文档 URL：优先 slug，兜底 document_id。
4. 管理后台文档“查看”跳转链接同样使用 route key（避免后台复制链接仍是 ULID）。

---

## 10. 实施顺序（低风险）

1. **Phase A（后端兼容能力）**
   - 增加字段与索引迁移
   - 路由解析支持 slug
   - canonical 重定向
2. **Phase B（API 与校验）**
   - 创建接口可选 `documentIdentifier`
   - 新增 `PATCH /api/docs/:docId/identifier`
3. **Phase C（前端交互）**
   - 文档树菜单新增“自定义标识”入口
   - 树数据与链接改用 `documentRouteKey`
4. **Phase D（站点链路统一）**
   - 首页搜索、sitemap、后台查看链接统一 route key

---

## 11. 回归测试清单

## 11.1 路由兼容

1. 旧链接 `/r/:space/:document_id` 仍可访问。
2. 文档设置 slug 后，旧链接重定向到 slug。
3. `/r/:space/:node_id` 兼容逻辑不回归。

## 11.2 约束与校验

1. 非法标识被拒绝。
2. 同空间冲突被拒绝。
3. 清空标识后回落到 `document_id` canonical。

## 11.3 功能不回归

1. 编辑、保存、修订、附件、权限链路正常。
2. 阅读页 SSR 与异步切换正常。
3. 首页搜索与 sitemap 链接有效。

---

## 12. 风险与回滚

## 12.1 风险

1. URL 输出链路多（阅读页、首页搜索、sitemap、后台），容易出现部分遗漏。
2. 若 canonical 与重定向策略不一致，会产生重复收录。

## 12.2 回滚策略

1. 保留 `document_id` 解析能力作为永久兜底。
2. 前端链接生成可通过开关回退到 `document_id` 模式。
3. 出现线上异常时，可先禁用 slug 输出，仅保留读取兼容。

