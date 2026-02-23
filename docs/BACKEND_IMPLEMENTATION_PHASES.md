# Implementation Phases: 后端功能里程碑（Gin）

**Project Type**: 现有项目前后端联调（后端从健康检查扩展到业务 API）  
**Scope**: `auth` + `spaces/tree` + `documents/revisions` + `ACL` + 冲突检测  
**Stack**: Go + Gin + GORM + SQL Migrations（SQLite 起步，兼容 PostgreSQL/MySQL）  
**Current Baseline（2026-02-21）**: 已具备 `healthz`、`/api/auth/*`、`/api/themes`、`/api/docs/:docId/theme` 及 `/api/admin/*` 首期治理接口；核心工作区后端链路（spaces/tree/docs/revisions/acl）仍待完成。  
**Estimated Total**: 6~8 周（约 160~220 工时）

---

## 最新进展（2026-02-21）

1. Milestone 1、Milestone 2、Milestone 3 已完成。  
2. 已落地主题能力（`themes` 表 + 文档 `theme_id` 引用 + `/api/themes` + `/api/docs/:docId/theme`）。  
3. 已完成“原 TS 主题”全量迁移（7 个内置主题），并补齐历史库升级策略（upsert）。  
4. 登录注册 UI 已接入并与后端认证接口联调完成（`/api/auth/*`）。  
5. 管理后台规划文档已建立：`docs/ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`（含全站/空间双管理模型与任务清单）。  
6. 管理后台已推进到 Milestone 6（进行中）；主题/系统配置能力已接入，审计中心与发布前加固待收口，详见 `docs/ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`。  

详见：`docs/backend-ai-handoff.md`

---

## 目标与约束

1. 与前端 `apps/web/src/data-access/http/adapter.ts` 现有契约保持一致，优先保证可联调。  
2. 业务实体对外 ID 统一为小写 ULID，允许保留自增 `id` 作为技术主键。  
3. 文档保存必须支持乐观锁冲突检测（`baseVersion` -> `409 Conflict`）。  
4. 权限规则采用 `owner > collaborator > reader`，并支持目录继承 + 文档覆盖。  
5. 每个阶段都必须包含可执行验收（HTTP 用例 + 数据一致性 + 回归项）。

---

## Milestone 1: 工程基础与可观测性
**Status**: Completed（2026-02-19）  
**Type**: Infrastructure  
**Estimated**: 8~12 小时  
**Files**:
- `apps/server/internal/config/config.go`
- `apps/server/internal/server/router.go`
- `apps/server/internal/server/middleware/*`（新增鉴权前置中间件骨架）
- `apps/server/cmd/server/main.go`
- `apps/server/.env.example`

**Tasks**:
- 扩展配置项：数据库连接、JWT 密钥、Token 过期时间、日志级别。
- 统一 API 错误响应结构（至少包含 `code` 与 `message`）。
- 增加请求链路基础能力：请求 ID、统一恢复、超时与日志字段。
- 保持现有 `/api/healthz` 不回归。

**Verification Criteria**:
- [x] 服务可启动，配置缺失时有明确报错。
- [x] `GET /api/healthz` 返回 `200`。
- [x] 非法路由返回统一 JSON 错误响应。
- [x] 中间件对 `OPTIONS`、`GET`、`POST` 请求行为一致且可预期。

**Exit Criteria**: 后续业务模块可直接复用统一配置与错误处理，不再重复造轮子。

---

## Milestone 2: 数据库接入与迁移基线
**Status**: Completed（2026-02-19）  
**Type**: Database  
**Estimated**: 14~18 小时  
**Files**:
- `apps/server/internal/storage/db.go`（新增）
- `apps/server/internal/storage/migrations/*.sql`（新增）
- `apps/server/internal/storage/models/*.go`（新增）
- `apps/server/.env.example`
- `README.md`

**Tasks**:
- 建立数据库抽象与连接管理，先跑通 SQLite，本地保留 PostgreSQL/MySQL 兼容配置。
- 建立首版迁移：`users`、`spaces`、`space_members`、`nodes`、`documents`、`document_revisions`、`node_permissions`、`document_permissions`。
- 建立关键索引与唯一约束（尤其 ULID、外键与排序相关索引）。
- 定义仓储层最小接口（不实现业务，仅打通 CRUD 骨架）。

**Verification Criteria**:
- [x] 迁移可在空库执行并成功回滚。
- [x] 核心表约束生效（唯一索引、外键、非空）。
- [x] 简单 CRUD 冒烟通过（新增用户、空间、节点、文档）。
- [x] SQLite 本地运行稳定，不影响服务启动。

**Exit Criteria**: 所有业务 API 可以在迁移完成后的数据库上实现，不再改动基础表结构。

---

## Milestone 3: 认证与会话
**Status**: Completed（2026-02-20）  
**Type**: API  
**Estimated**: 16~24 小时  
**Files**:
- `apps/server/internal/server/handler/auth.go`（新增）
- `apps/server/internal/storage/migrations/*/0003_auth_sessions*.sql`（新增）
- `apps/server/internal/server/auth_handler_test.go`（新增）
- `apps/server/internal/server/router.go`

**Tasks**:
- 实现 `POST /api/auth/register`、`POST /api/auth/login`、`POST /api/auth/refresh`、`GET /api/auth/me`、`POST /api/auth/logout`。
- 使用 bcrypt 存储密码哈希，禁止明文密码落库。
- 实现 access/refresh token 流程（刷新时支持旋转与失效，基于 `user_sessions`）。
- 为后续业务接口接入鉴权中间件。

**Verification Criteria**:
- [x] 注册成功返回用户信息，重复邮箱返回 `409`。
- [x] 登录失败返回 `401`，成功返回有效会话信息。
- [x] 刷新 token 可续期，旧 refresh token 在旋转后返回 `401`。
- [x] `GET /api/auth/me` 在未登录时返回 `401`，登录后返回 `200`。

**Exit Criteria**: 用户身份可稳定识别，后续空间/文档 API 全部可挂鉴权。

---

## Milestone 4: 空间与目录树读取链路
**Status**: Pending  
**Type**: API  
**Estimated**: 12~18 小时  
**Files**:
- `apps/server/internal/server/handler/space.go`（新增）
- `apps/server/internal/service/workspace_service.go`（新增）
- `apps/server/internal/repository/space_repo.go`（新增）
- `apps/server/internal/repository/node_repo.go`（新增）
- `apps/server/internal/server/router.go`

**Tasks**:
- 实现 `GET /api/spaces`、`POST /api/spaces`、`GET /api/spaces/:spaceId/tree`。
- 创建空间时自动建立 owner 关系。
- 树接口返回结构与前端 `TreeNode` 契约一致（`id/spaceId/parentId/type/title/sort/children`）。
- 引入最小读取权限校验（无权限返回 `403`）。

**Verification Criteria**:
- [ ] 空间创建后可在列表中看到。
- [ ] 树接口可返回多层目录结构并保持稳定排序。
- [ ] 未授权用户访问他人空间返回 `403`。
- [ ] 参数非法返回 `400`，目标不存在返回 `404`。

**Exit Criteria**: 前端 `listSpaces/getTree/createSpace` 可在 `http` 驱动下跑通。

---

## Milestone 5: 目录树写操作与级联删除
**Status**: Pending  
**Type**: API  
**Estimated**: 16~24 小时  
**Files**:
- `apps/server/internal/server/handler/node.go`（新增）
- `apps/server/internal/service/node_service.go`（新增）
- `apps/server/internal/repository/node_repo.go`
- `apps/server/internal/repository/document_repo.go`（新增）
- `apps/server/internal/server/router.go`

**Tasks**:
- 实现 `POST /api/spaces/:spaceId/nodes`、`PATCH /api/nodes/:nodeId`、`DELETE /api/nodes/:nodeId`。
- 文档节点创建时自动建立 `documents` 记录，并返回 `CreateNodeResult`。
- 实现节点重命名、父级变更、排序字段更新。
- 实现目录递归删除，文档节点删除时联动删除 `documents/revisions`。

**Verification Criteria**:
- [ ] 创建文档节点后可立即通过文档接口读取内容。
- [ ] 重命名/移动节点后树结构正确。
- [ ] 删除目录后子树与文档版本数据被完整清理。
- [ ] 并发写入下无脏数据（事务回滚可验证）。

**Exit Criteria**: 前端 `createNode/updateNode/deleteNode` 在后端模式可完整使用。

---

## Milestone 6: 文档保存、版本历史与冲突检测
**Status**: Pending  
**Type**: API  
**Estimated**: 18~26 小时  
**Files**:
- `apps/server/internal/server/handler/document.go`（新增）
- `apps/server/internal/service/document_service.go`（新增）
- `apps/server/internal/repository/document_repo.go`
- `apps/server/internal/repository/revision_repo.go`（新增）
- `apps/server/internal/server/router.go`

**Tasks**:
- 实现 `GET /api/docs/:docId`、`PUT /api/docs/:docId`、`GET /api/docs/:docId/revisions`、`GET /api/docs/:docId/revisions/:revisionId`。
- 保存时校验 `baseVersion`，使用条件更新实现乐观锁。
- 发生版本冲突时返回 `409`，响应体包含 `latestDocument`，与前端 `ConflictError` 结构兼容。
- 每次成功保存都写入修订记录。

**Verification Criteria**:
- [ ] 正常保存返回 `200` 且 `version` 自增。
- [ ] 过期 `baseVersion` 保存返回 `409`，并返回最新文档内容。
- [ ] 历史版本接口可按时间逆序返回。
- [ ] 文档不存在返回 `404`，未授权返回 `403`。

**Exit Criteria**: 前端编辑器保存链路在 `http` 驱动可闭环，冲突提示可被正确触发。

---

## Milestone 7: ACL 权限管理（目录继承 + 文档覆盖）
**Status**: Pending  
**Type**: API  
**Estimated**: 20~28 小时  
**Files**:
- `apps/server/internal/server/handler/permission.go`（新增）
- `apps/server/internal/service/permission_service.go`（新增）
- `apps/server/internal/repository/permission_repo.go`（新增）
- `apps/server/internal/domain/acl/evaluator.go`（新增）
- `apps/server/internal/server/router.go`

**Tasks**:
- 实现 `GET/PUT /api/nodes/:nodeId/permissions`。
- 实现 `GET/PUT /api/docs/:docId/permissions`。
- 实现权限求值：文档单独授权优先于目录继承，owner 全局最高。
- 将权限求值接入所有空间、节点、文档写读接口。

**Verification Criteria**:
- [ ] 授权更新后立即生效，无需重启。
- [ ] 目录授权可继承到子目录与文档。
- [ ] 文档单独授权可覆盖目录继承结果。
- [ ] owner 身份不会被低权限规则错误降级。

**Exit Criteria**: 权限模型与产品规则一致，核心权限矩阵测试通过。

---

## Milestone 8: 联调、测试基线与发布准备
**Status**: Pending  
**Type**: Integration / Testing  
**Estimated**: 18~30 小时  
**Files**:
- `apps/server/internal/server/*_test.go`（新增）
- `apps/server/internal/service/*_test.go`（新增）
- `apps/server/Makefile` 或脚本（新增，可选）
- `README.md`
- `docs/WORKSPACE_ACCEPTANCE.md`（补充后端联调项，可选）

**Tasks**:
- 建立 API 集成测试与关键服务单测（认证、节点递归删除、409 冲突、权限继承）。
- 与前端 HTTP 数据驱动完成主流程联调。
- 固化运行命令与回归脚本（启动、迁移、测试）。
- 准备发布前检查清单（配置、密钥、CORS、日志、备份策略）。

**Verification Criteria**:
- [ ] `go test ./...` 通过。
- [ ] 手工主流程通过：注册登录 -> 建空间 -> 建文档 -> 编辑保存 -> 冲突处理。
- [ ] 前端 `http` 模式无阻断错误。
- [ ] 发布前检查项可逐条打勾执行。

**Exit Criteria**: 后端可作为默认数据源支撑日常使用，具备可重复回归与可发布能力。

---

## 依赖关系（执行顺序）

1. Milestone 1 -> 2：先稳定基础设施，再建数据层。  
2. Milestone 2 -> 3：认证依赖用户表与 token 存储。  
3. Milestone 3 -> 4 -> 5 -> 6：先读链路，再写链路，再文档版本。  
4. Milestone 7 在 4~6 完成后接入：否则权限检查点不完整。  
5. Milestone 8 最后执行：统一回归与联调验收。

---

## 首个冲刺复盘（已完成）

1. 完成 Milestone 1（配置、错误模型、中间件骨架）。  
2. 完成 Milestone 2 的迁移骨架与 SQLite 跑通。  
3. 拉通 Milestone 3 的最小认证闭环（register/login/me）。  

复盘结论：首个冲刺目标已达成（Milestone 1~3 完成）。

## 下一冲刺建议（当前）

1. 启动 Milestone 4：先打通 `GET/POST /api/spaces` 与 `GET /api/spaces/:spaceId/tree` 最小可用链路。  
2. 并行准备 Milestone 5 所需仓储与事务封装，避免节点写入阶段返工。  
3. 为 Milestone 6 提前固化 `409 Conflict` 响应契约，确保与前端 `ConflictError` 对齐。  

---

## 关联文档

- `docs/backend-ai-handoff.md`
