# Markdown 编辑器全栈路线图（React + Vite + CodeMirror + Gin）

> 目标：前端优先交付可独立使用的编辑器（无后端可运行），并提前完成接口抽象；后端能力后置接入，最终支持登录注册、空间与目录组织、版本冲突检测（SQLite / PostgreSQL / MySQL）。

## 1. 范围定义（按你当前需求）

- 前端：双栏实时预览（左编辑 / 右预览）
- 渲染能力：GFM、代码高亮、Mermaid（流程图/甘特图）、公式（KaTeX）
- 内容模型：空间（Space） + 目录树（Folder） + 文档（Doc）
- 用户体系：注册、登录、会话鉴权
- 权限模型：
  - 角色：`owner`（拥有者）/ `collaborator`（协作者）/ `reader`（阅读者）
  - 目录授权：目录权限自动继承到所有子目录与文档
  - 文档授权：支持对单篇文档单独授权（优先于目录继承）
- 版本策略：
  - 服务端：保存时做版本冲突检测（乐观锁）
  - 本地：多版本历史缓存与查看（无需多人协同）
- 后端：Golang + Gin，支持 SQLite / PostgreSQL / MySQL
- 交付策略：前端功能先完整可用，后端按接口契约后置接入

---

## 2. 总体架构

### 前端（React）

- `editor`：CodeMirror 6 编辑区
- `preview`：Markdown 渲染管线 + Mermaid 渲染
- `workspace`：空间/目录树/文档列表
- `history`：本地版本历史与 Diff 查看
- `sync`：自动保存、冲突提示、版本恢复
- `data-access`：统一数据接口层（本地实现 + HTTP 实现）

### 前端接口抽象（优先实现）

- 目标：在没有后端时，编辑器全功能可用；后续切换后端时 UI 代码最小改动
- 抽象建议：
  - `AuthGateway`：当前可提供本地/匿名实现，后续切换 JWT
  - `WorkspaceGateway`：空间、目录、文档查询与变更
  - `DocumentGateway`：文档读取、保存、历史与冲突信息
- 适配器建议：
  - `localAdapter`：基于 `IndexedDB`/`localStorage`，作为默认实现
  - `httpAdapter`：后置实现，严格对齐后端 API 契约
- 切换方式：通过环境变量或配置注入（如 `VITE_DATA_DRIVER=local|http`）

### 后端（Gin）

- `auth`：注册、登录、JWT 鉴权、刷新
- `space`：空间 CRUD、成员管理
- `tree`：目录节点（folder/doc）维护
- `acl`：目录级授权继承、文档级单独授权、权限计算
- `doc`：文档内容 CRUD
- `revision`：服务端文档版本记录
- `storage`：多数据库适配层（SQLite / PostgreSQL / MySQL）

---

## 3. 数据模型（核心表）

### 用户与空间

- `users(id, email, password_hash, created_at, updated_at)`
- `spaces(id, name, owner_id, created_at, updated_at)`
- `space_members(id, space_id, user_id, role, created_at)`
  - `role`: `owner` | `collaborator` | `reader`

### 目录与文档

- `nodes(id, space_id, parent_id, type, title, sort, created_at, updated_at)`
  - `type`: `folder` | `doc`
- `documents(id, node_id, title, content_md, version, updated_by, created_at, updated_at)`
- `document_revisions(id, document_id, version, content_md, base_version, editor_id, created_at)`
- `node_permissions(id, node_id, user_id, role, granted_by, created_at)`
  - 目录授权，天然继承到子节点
- `document_permissions(id, document_id, user_id, role, granted_by, created_at)`
  - 文档单独授权，覆盖目录继承结果

> 建议 `documents.version` 从 1 开始，每次成功保存 +1，作为冲突检测基准。

---

## 4. 权限与冲突策略

### 角色能力矩阵（MVP）

- `owner`：空间内所有资源的查看/编辑/删除/权限管理
- `collaborator`：可查看/编辑/删除已授权目录与文档，不可做权限管理
- `reader`：仅查看已授权目录/文档，不可编辑

### 权限计算规则（建议固定）

- 同时命中文档授权与目录继承时，文档授权优先（可提权或降权覆盖）
- 无文档授权时，使用最近祖先目录的授权结果
- 同一资源命中多条授权时，按最高角色生效：`owner > collaborator > reader`
- 空间 `owner` 默认拥有全量权限，不受单条授权限制

### 服务端冲突检测（必做）

- 前端保存请求携带 `base_version`
- 后端更新语句采用条件更新：
  - `WHERE id = ? AND version = base_version`
- 若 `RowsAffected = 0` 返回 `409 Conflict`，并返回最新文档与版本号
- 前端弹窗提供 3 个动作：
  - 打开差异对比（本地版本 vs 最新服务端版本）
  - 手动合并后再保存
  - 放弃本次修改

### 本地多版本历史（必做）

- 使用 `IndexedDB`（建议 `Dexie`）存储本地快照
- 快照建议字段：
  - `id, doc_id, local_version, remote_version, content_md, created_at, source(auto|manual)`
- 策略建议：
  - 自动保存每 30~60 秒一版
  - 保留最近 100 版（可配置）
  - 提供历史列表 + Diff 预览 + 恢复为当前草稿

---

## 5. API 设计（MVP）

> 本节为后端后置实现的契约设计。前端先基于同名接口完成本地实现。

### 认证

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/refresh`
- `GET /api/auth/me`

### 空间与目录

- `GET /api/spaces`
- `POST /api/spaces`
- `GET /api/spaces/:spaceId/tree`
- `POST /api/spaces/:spaceId/nodes`
- `PATCH /api/nodes/:nodeId`
- `DELETE /api/nodes/:nodeId`

### 权限管理

- `GET /api/nodes/:nodeId/permissions`
- `PUT /api/nodes/:nodeId/permissions`
- `GET /api/docs/:docId/permissions`
- `PUT /api/docs/:docId/permissions`

### 文档

- `GET /api/docs/:docId`
- `PUT /api/docs/:docId`（含 `base_version`）
- `GET /api/docs/:docId/revisions`
- `GET /api/docs/:docId/revisions/:revisionId`

---

## 6. 里程碑排期（8 周建议）

### 第 1 周：工程初始化

- 前端：编辑器页面骨架、双栏布局、基础状态管理
- 前端：定义 `Gateway` 接口与 `localAdapter` 基础实现
- 验收：无后端时可打开、编辑、预览并本地保存文档

### 第 2 周：渲染能力完善

- Markdown 渲染链：GFM + 代码高亮 + Mermaid + KaTeX
- Mermaid 错误兜底与重渲染优化
- 验收：长文档下实时预览稳定，渲染能力完整

### 第 3 周：本地工作区能力

- 本地空间/目录树/文档 CRUD（先单用户）
- 本地排序与目录展开收起状态持久化
- 验收：可完整管理多空间与目录结构（纯前端）

### 第 4 周：编辑体验增强

- 工具栏、快捷键、大纲导航、滚动同步
- 自动保存状态提示（保存中/已保存/失败）
- 验收：核心写作体验达到可日常使用水平

### 第 5 周：文档保存与冲突检测

- 本地多版本快照（按时间与操作来源）
- 冲突模拟机制（基于 `base_version` 的本地冲突场景）
- 差异对比 + 手动合并流程
- 验收：在无后端条件下可演练并完成冲突手动合并

### 第 6 周：后端接入准备（前端侧）

- `httpAdapter` 空实现与契约对齐
- 错误码规范（重点 `409`）与重试/提示策略
- 关键流程契约测试（mock API / MSW）
- 验收：切换 `local|http` 驱动不影响页面层逻辑

### 第 7 周：后端基础能力（后置）

- Gin 基础模块：auth、space、tree、doc
- SQLite / PostgreSQL / MySQL 基础连接与迁移
- 验收：核心查询/保存接口可跑通

### 第 8 周：联调与质量加固

- `httpAdapter` 联调接入真实后端
- 冲突检测 `409` + 手动合并闭环验证
- 前后端回归测试、性能与安全加固
- 验收：前后端一体化发布条件达成

---

## 7. 推荐依赖清单

### 前端

- 基础：`react react-dom vite typescript`
- 编辑器：`@uiw/react-codemirror codemirror @codemirror/lang-markdown @codemirror/language-data`
- 渲染：`react-markdown remark-gfm remark-math rehype-katex katex rehype-sanitize`
- Mermaid：`mermaid`
- 代码高亮：`rehype-highlight`（后续可升级 `shiki`）
- 状态与数据：`zustand @tanstack/react-query`
- 本地历史：`dexie`
- Diff（建议）：`diff react-diff-viewer-continued`

### 后端（Go）

- Web：`github.com/gin-gonic/gin`
- 鉴权：`github.com/golang-jwt/jwt/v5`
- 密码：`golang.org/x/crypto/bcrypt`
- ORM：`gorm.io/gorm`
- DB 驱动：
  - SQLite：`gorm.io/driver/sqlite`
  - PostgreSQL：`gorm.io/driver/postgres`
  - MySQL：`gorm.io/driver/mysql`
- 迁移：`github.com/golang-migrate/migrate/v4`
- 配置：`github.com/caarlos0/env/v11`（或 `viper`）
- 日志：`go.uber.org/zap`（或 `slog`）

---

## 8. 风险与规避

- 接口抽象不完整导致后续重构成本高
  - 方案：先定义网关接口与 DTO，再开发页面；页面禁止直接调用 HTTP
- 授权继承与单文档授权叠加复杂
  - 方案：固定优先级（文档授权 > 目录继承），并做权限计算单测
- Mermaid 渲染与预览性能
  - 方案：输入防抖 + 仅在代码块变化时重渲染 Mermaid
- 安全风险（XSS）
  - 方案：`rehype-sanitize` 严格白名单，后端存储前后都做必要校验
- 冲突对比体验复杂
  - 方案：先实现双栏 Diff + 手动复制合并，不做自动三方合并

---

## 9. MVP 验收标准

### 阶段 A（前端优先 MVP，无后端）

- 双栏实时预览稳定，支持 Mermaid、公式、代码高亮
- 本地空间/目录/文档可完整管理
- 本地历史可查看和恢复
- 本地冲突模拟 + 差异对比 + 手动合并可用
- 具备 `Gateway + localAdapter + httpAdapter` 接口抽象

### 阶段 B（后端接入后）

- 登录/注册可用，接口鉴权有效
- 保存具备服务端冲突检测（返回 `409` + 手动合并）
- 权限模型生效：目录继承 + 文档单独授权覆盖
- 同一业务逻辑可运行在 SQLite / PostgreSQL / MySQL

---

## 10. 已确认决策

1. `collaborator` 允许删除目录与文档（仍不具备权限管理能力）。
2. 单独授权权限高于继承权限（支持提权/降权覆盖）。

