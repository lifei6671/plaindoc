# 文档模板（场景模板）技术方案

**文档状态**: Implemented（已实现）  
**创建日期**: 2026-03-04  
**适用范围**: `apps/server`、`apps/web`（编辑器、后台管理、数据层）  
**目标**: 支持“编辑区选择模板应用到文档”，并支持“平台管理员在后台按场景管理模板”。

## 当前进展（2026-03-04）

1. **Phase A 已完成**
   - 三库迁移（`document_templates`）已落地。
   - 业务端只读接口已落地：`GET /api/document-templates`、`GET /api/document-templates/:templateId`。
   - 前端 data-access 已接入模板查询能力。
2. **Phase B 已完成（后端 + 管理页）**
   - 后端管理接口已落地：`GET/POST/PUT/DELETE /api/admin/document-templates`，并补充详情接口 `GET /api/admin/document-templates/:templateId`。
   - 接口已接入 `platform_admin` 权限与高风险操作 token（更新/删除）。
   - 审计日志已接入 `document_template` 模块。
   - 后台前端已新增“模板管理”菜单与基础 CRUD 页面。
3. **Phase C 已完成（创建文档链路）**
   - 新建文档接口已支持 `templateId`（可空）。
   - 服务端创建文档时会用模板内容初始化 `documents.content_md` 与首版 `document_revisions.content_md`。
   - 编辑器文档树“新建文档”模态已支持模板选择 + 自定义文档标识（可空）。
4. **Phase D 部分完成（体验与测试补强）**
   - “新建文档”模态已补齐模板场景分组（`optgroup`）与模板正文预览。
   - 模板预览支持失败重试，且已补充请求防竞态（快速切换模板时仅采纳最新请求结果）。
   - 模板空态已补充引导文案（无模板时提示联系管理员在后台创建并启用模板）。
   - 前端测试已新增：
     - `apps/web/src/workspace/use-workspace.test.ts`（创建节点透传 `templateId`/`documentIdentifier`）。
     - `apps/web/src/components/WorkspaceTree.test.tsx`（模板加载、预览、创建透传、预览失败重试）。
     - `apps/web/src/admin/pages/AdminDocumentTemplatesPage.test.tsx`（后台模板管理页创建/编辑/删除/内置模板禁用）。
5. **当前剩余**
   - 补齐更多跨端 E2E 场景（模板启停、空态引导、阅读路由可访问性）与 CI 分层执行。
6. **Phase E 部分完成（执行中）**
   - 已引入 Playwright 基线（`apps/web/playwright.config.ts`）与根脚本透传（`web:e2e*`）。
   - 已落地两条冒烟用例：
     - `apps/web/e2e/admin-document-templates.smoke.spec.ts`
     - `apps/web/e2e/workspace-document-template-create.smoke.spec.ts`
   - 已补充运行文档：`apps/web/e2e/README.md`。
7. **Phase F 已完成（本次实现）**
   - 引入独立场景主数据 `document_template_scenes`。
   - 约束：`sceneKey` 在模板中不可修改；场景被模板引用时禁止删除。
   - 由于功能未上线，直接移除 `document_templates.scene_name` 字段，模板展示名称统一由场景表提供。
   - 管理端模板页已接入场景管理（场景 CRUD + 引用保护）与模板创建场景下拉。

---

## 1. 背景与目标

当前编辑器支持文档树创建和正文编辑，但缺少标准化起稿能力。团队在会议纪要、周报、故障复盘、需求评审等场景中会重复手工搭建结构，效率较低且格式不统一。

目标能力：

1. 用户在编辑区可按场景快速选择模板并应用。
2. 平台管理员可在后台统一管理全站模板（按场景分组）。
3. 在不影响现有文档链路（创建、保存、修订、权限、SEO 路由）的前提下渐进式上线。

---

## 2. 设计原则

1. **兼容优先**：模板能力是增量功能，旧客户端与旧流程保持可用。
2. **低侵入**：文档仍以 `document_id` 为主键，不改变正文与修订主链。
3. **安全可控**：模板治理仅允许 `platform_admin`。
4. **性能可控**：模板读取走索引+分页，避免后台大列表全表扫描。
5. **可扩展**：后续可支持“空间级模板”与“模板版本化”，本期先做全站模板。

---

## 3. 范围定义（V1）

### 3.1 包含

1. 新增模板数据表与三套数据库迁移。
2. 新增场景主数据表与三套数据库迁移。
2. 业务端模板只读接口（编辑器可读取已启用模板）。
3. 后台模板管理接口（CRUD、启停、排序、审计）。
4. 后台场景管理接口（CRUD、排序、审计、引用保护）。
4. 编辑器模板选择弹窗（预览 + 应用）。
5. 新建文档时可选模板（可空，不影响原流程）。

### 3.2 不包含

1. 模板协作审批流。
2. 模板多版本回滚 UI。
3. 空间级模板权限隔离（后续版本考虑）。

---

## 4. 数据模型与迁移

新增表：

1. `document_template_scenes`（场景主数据）
2. `document_templates`（模板，引用场景）

`document_template_scenes` 建议字段：

1. `id`：自增主键。
2. `scene_key`：场景标识（唯一，如 `meeting`、`weekly-report`）。
3. `scene_name`：场景名称（如“会议纪要”）。
4. `description`：场景描述（可空）。
5. `sort`：场景排序。
6. `is_builtin`：是否内置。
7. `created_by_user_id`、`updated_by_user_id`。
8. `created_at`、`updated_at`。

`document_templates` 建议字段：

1. `id`：自增主键。
2. `template_id`：模板标识（唯一，程序使用）。
3. `scene_key`：场景标识（外键语义，指向 `document_template_scenes.scene_key`）。
4. `name`：模板名称。
5. `description`：模板简介。
6. `default_title`：应用模板时建议标题（可空）。
7. `content_md`：模板正文（Markdown）。
8. `sort`：同场景排序。
9. `is_builtin`：是否内置。
10. `is_enabled`：是否启用。
11. `created_by_user_id`、`updated_by_user_id`。
12. `created_at`、`updated_at`。

迁移策略（本期）：

1. 创建 `document_template_scenes`。
2. 从 `document_templates(scene_key, scene_name)` 去重回填场景。
3. 删除 `document_templates.scene_name`（功能尚未上线，允许直接收敛模型）。

索引与约束：

1. `document_template_scenes.UNIQUE(scene_key)`。
2. `document_templates.UNIQUE(template_id)`。
3. `document_templates.INDEX(scene_key, is_enabled, sort, updated_at)`。
4. `document_templates.INDEX(is_enabled, updated_at)`。
5. `CHECK(template_id <> '')`、`CHECK(scene_key <> '')`（按方言可用性实现）。

---

## 5. API 设计

### 5.1 业务端（编辑器）

1. `GET /api/document-templates`
   - 入参：`sceneKey?`、`keyword?`、`page?`、`pageSize?`
   - 仅返回 `enabled=true` 模板。
2. `GET /api/document-templates/:templateId`
   - 返回模板完整内容（含 `contentMd`）。
3. 扩展 `POST /api/spaces/:spaceId/nodes`
   - 新增可选参数：`templateId`（仅 `type=doc` 生效）。

### 5.2 后台（平台管理员）

1. 模板：
   - `GET /api/admin/document-templates`
   - `POST /api/admin/document-templates`
   - `PUT /api/admin/document-templates/:templateId`
   - `DELETE /api/admin/document-templates/:templateId`
2. 场景：
   - `GET /api/admin/document-template-scenes`
   - `POST /api/admin/document-template-scenes`
   - `PUT /api/admin/document-template-scenes/:sceneKey`
   - `DELETE /api/admin/document-template-scenes/:sceneKey`

权限策略：

1. 仅 `platform_admin`。
2. 高风险变更（更新/删除）接入后台一次性操作令牌机制。

---

## 6. 关键业务规则

1. `template_id`：`^[a-z0-9][a-z0-9_-]{1,63}$`。
2. `scene_key`：同样采用可读 key 规范。
3. `scene_key` 在模板创建后不可修改。
4. 场景被模板引用时禁止删除。
5. `name`、`scene_name` 不可空；`content_md` 可空（允许纯骨架模板）。
4. 删除内置模板：禁止。
5. 业务端只读取启用模板。
6. 新建文档时：
   - 未传 `templateId`：保持当前逻辑（空正文 + 版本 1）。
   - 传入有效模板：文档初始 `content_md` 使用模板内容，首版修订同步写入。

---

## 7. 前端改造

### 7.1 编辑器

1. 新建文档是选择可以选择模板
2. 模板弹窗能力：
   - 场景筛选。
   - 关键词过滤。
   - 右侧模板预览。
   - “应用模板”确认（当前文档有内容时二次确认）。
3. 应用后走现有保存机制，不新增专用保存接口。

### 7.2 新建文档流程

1. 在文档树“新建文档”模态中增加可选模板下拉。
2. 传参给创建接口，后端完成模板内容初始化。

### 7.3 后台

1. 新增“模板管理”菜单与页面。
2. 模板创建/编辑不再填写场景名称，改为从场景下拉选择。
3. 新增“场景管理”能力，支持场景列表、创建、编辑、删除（有引用禁止删除）。
4. 模板管理继续支持列表、创建、编辑、启停、删除。

---

## 8. 安全规范

1. 后台模板治理必须校验管理员身份与角色。
2. 后台场景治理必须校验管理员身份与角色。
2. 接口统一参数校验，防止超长/非法输入。
3. 模板正文长度设置上限（例如 200KB），避免资源滥用。
4. 模板变更写审计日志（module: `document_template`）。
5. 场景变更写审计日志（module: `document_template_scene`）。

---

## 9. 性能规范

1. 列表查询仅选取必要字段，详情接口再返回 `content_md`。
2. 模板列表通过 `scene_key` 关联场景表，仅返回必要字段。
2. 列表接口分页并限制 `pageSize` 上限（例如 100）。
3. 使用组合索引命中 `scene_key + is_enabled + sort` 查询路径。
4. 编辑器侧模板列表可做短期内存缓存，减少重复请求。

---

## 10. 分阶段实施

### Phase A（先做）

1. 迁移 + 模型 + 仓储。
2. 业务端模板只读接口。
3. 前端 data-access 接入模板列表/详情。

### Phase B

1. 后台模板管理 API + 审计。
2. 管理端页面联调。

### Phase C

1. 编辑器模板弹窗。
2. 新建文档可选模板。

### Phase D

1. 回归测试、接口压测、文档补齐。

### Phase F（场景主数据收敛，本次）

1. 场景表迁移、回填、模板表去冗余字段（删除 `scene_name`）。
2. 后台场景管理 API + 审计。
3. 模板 API 收敛（模板仅引用 `scene_key`，`sceneKey` 不可修改）。
4. 管理端模板页接入场景管理与场景下拉选择。

---

## 11. 测试清单

### 11.1 已完成

1. 迁移幂等与回滚验证（sqlite/mysql/postgres）。
2. 业务端：
   - 仅返回启用模板。
   - scene/keyword/page 参数校验。
   - 详情 not found 分支。
3. 后台：
   - 非平台管理员访问拒绝。
   - 模板与场景 CRUD 审计完整。
4. 创建文档带模板：
   - 文档内容正确初始化。
   - 首版修订内容与文档一致。
5. 前端：
   - 新建文档模态模板加载、预览、预览失败重试。
   - 新建文档创建参数透传 `templateId` 与 `documentIdentifier`。
   - 后台模板管理页：创建、编辑、删除、内置模板禁用交互。

### 11.2 待补充

1. 补齐模板“启停后前台可见性联动”E2E。
2. 补齐“无模板空态引导”E2E。
3. 补齐“场景删除引用保护”E2E。
4. 补齐“创建后阅读路由可访问性”E2E（需接入可控测试数据的 SSR 校验）。

---

## 12. 回滚策略

1. 若模板功能异常，可前端隐藏入口并停止调用模板 API。
2. 后端保留只读 API 不影响现有文档编辑链路。
3. 模板能力关闭时，新建文档自动回退空正文初始化。

---

## 13. 跨端 E2E 实施方案（部分已执行）

### 13.0 已落地项（2026-03-04）

1. Playwright 框架与脚本：
   - `apps/web/playwright.config.ts`
   - `apps/web/package.json`（`e2e`、`e2e:headed`、`e2e:ui`、`e2e:install`）
   - `package.json`（`web:e2e*` 透传脚本）
2. 冒烟用例：
   - 后台模板管理 CRUD：`apps/web/e2e/admin-document-templates.smoke.spec.ts`
   - 编辑器模板创建文档：`apps/web/e2e/workspace-document-template-create.smoke.spec.ts`
3. 运行说明：
   - `apps/web/e2e/README.md`

### 13.1 工具与基线

1. 建议引入 `Playwright` 作为跨端 E2E 框架（Chromium first，后续扩展 WebKit/Firefox）。
2. 通过 API fixture 或测试专用 seed 脚本准备数据，避免 UI 逐步初始化导致用例不稳定。
3. 所有 E2E 用例使用独立测试账号与隔离空间，防止并行执行互相污染。

### 13.2 首批必测场景（模板链路）

1. 管理员创建模板 -> 编辑器“新建文档”可见模板 -> 选择模板创建 -> 正文按模板初始化。
2. 管理员更新模板（名称/内容/启停）-> 编辑器模板列表与预览同步更新。
3. 管理员停用模板 -> 编辑器列表不再展示该模板。
4. 编辑器“无模板”空态引导可见（提示后台配置路径）。
5. 创建文档时同时传 `templateId + documentIdentifier`，创建成功后阅读路由可访问且内容正确。

### 13.3 稳定性与安全要求

1. 所有用例禁止依赖固定时间与随机排序，断言基于稳定字段（`templateId`、`sceneKey`、`documentIdentifier`）。
2. 用例中涉及高风险后台变更时，统一通过受控测试账号执行，避免误触生产风格权限策略。
3. 每个场景执行完毕后显式清理（删除测试模板/空间/文档），防止脏数据累积。

### 13.4 CI 接入策略

1. PR 阶段运行“精简冒烟集”（1~2 条关键链路，控制时长）。
2. Nightly 运行“完整模板回归集”并生成 HTML 报告与失败截图。
3. E2E 失败时自动上传 `trace.zip` 与 screenshot，便于快速定位。
