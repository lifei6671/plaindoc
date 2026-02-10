# Markdown 编辑器全栈路线图（React + Vite + CodeMirror + Gin）

> 目标：实现类似语雀的双栏实时预览 Markdown 编辑器，支持 Mermaid、公式、代码高亮；后端支持登录注册、空间与目录组织、版本冲突检测；数据库先支持 SQLite / PostgreSQL / MySQL。

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

---

## 2. 总体架构

### 前端（React）

- `editor`：CodeMirror 6 编辑区
- `preview`：Markdown 渲染管线 + Mermaid 渲染
- `workspace`：空间/目录树/文档列表
- `history`：本地版本历史与 Diff 查看
- `sync`：自动保存、冲突提示、版本恢复

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

- 前端：Vite + React + TS + 双栏布局骨架
- 后端：Gin 项目分层（handler/service/repo）
- 验收：项目能同时跑起，基础健康检查可用

### 第 2 周：认证与基础权限

- 注册/登录/JWT
- 基础鉴权中间件
- 空间角色（owner/collaborator/reader）落表
- 验收：受保护接口需登录访问，角色可鉴别

### 第 3 周：空间与目录树

- 空间 CRUD
- 目录树（folder/doc）增删改查
- 目录授权继承 + 文档单独授权
- 验收：目录授权可覆盖整棵子树，文档可单独授权

### 第 4 周：编辑器与实时预览

- CodeMirror + Markdown 渲染链
- GFM + 代码高亮 + Mermaid + KaTeX
- 验收：编辑与预览实时联动稳定

### 第 5 周：文档保存与冲突检测

- 文档保存接口（乐观锁）
- 409 冲突返回与前端差异对比交互
- 手动合并后再保存
- 验收：并发编辑冲突可被发现，并通过对比界面手动合并

### 第 6 周：本地多版本历史

- IndexedDB 历史快照
- 历史查看与恢复
- 验收：断网/误操作后可从本地历史恢复内容

### 第 7 周：多数据库支持

- SQLite / PostgreSQL / MySQL 统一仓储实现
- 验收：通过配置切换数据库后核心 API 正常

### 第 8 周：质量与发布准备

- 单元测试 + API 集成测试
- 性能与安全加固
- 验收：达到 MVP 上线标准

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

- 注册/登录可用，接口鉴权有效
- 可创建多个空间，并在空间内按目录组织文档
- 支持目录级授权继承与文档级单独授权
- 双栏实时预览稳定，支持 Mermaid、公式、代码高亮
- 保存具备版本冲突检测（返回 409 + 差异对比 + 手动合并）
- 本地历史可查看和恢复
- 同一业务逻辑可运行在 SQLite / PostgreSQL / MySQL

---

## 10. 已确认决策

1. `collaborator` 允许删除目录与文档（仍不具备权限管理能力）。
2. 单独授权权限高于继承权限（支持提权/降权覆盖）。

