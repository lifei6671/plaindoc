# Implementation Phases: 管理后台（Admin Console）

**Project Type**: 现有项目新增管理后台（全站管理 + 空间管理）  
**Scope**: 用户管理 + 空间管理 + 文档管理 + 系统配置 + 主题管理 + 审计  
**Stack**: Web(React + Vite + React Router) + Server(Gin + GORM + SQL Migrations)  
**Estimated Total**: 14~20 天（按 2 周冲刺可拆为两期上线）

---

## 最新进展（2026-02-21）

1. Milestone 1 已完成（RBAC + Scope + 状态迁移 + admin 鉴权中间件）。  
2. Milestone 2 已完成（后台路由壳 + 角色菜单显示 + admin 身份联调）。  
3. 已新增后台基础接口：
   - `GET /api/admin/me`
   - `GET /api/admin/spaces/:spaceId/check`
4. 已完成后台权限矩阵基础测试（`platform_admin`、`space_admin`、非管理员）。  
5. Milestone 3 核心能力已完成，整体已收口（用户列表/封禁/解封/软删除 + 管理端用户页联调 + 审计写入）。  
6. Milestone 4 已完成（空间与文档管理能力 + 批量操作 + scope 权限校验 + 空间删除后文档级联软删除 + 业务端封禁/删除拦截）。  
7. Milestone 5 主题与系统配置功能已完成，剩余发布前加固项并入 Milestone 6。  
8. Milestone 6 已进入进行中（审计查询接口与关键动作写入已落地，发布前加固待完成）。  
9. Milestone 6 已补齐 operation token 权限矩阵与核心 E2E 覆盖（签发权限、跨管理员复用拦截、request_id 审计透传）。  
10. 已新增发布清单文档 `docs/ADMIN_CONSOLE_RELEASE_CHECKLIST.md`（配置、回滚、监控、告警、验收流程）。  
11. 根据当前决策，后台性能压测与性能基线项豁免，不作为首期上线阻塞条件。  

---

## 一、已确认范围（来自产品确认）

1. 支持两类后台能力：全站管理、空间管理。  
2. 根据用户角色动态展示菜单。  
3. 复用现有 `users` 体系，不新建独立 admin 账号系统。  
4. 首期必须覆盖：用户管理、空间管理、文档管理、系统配置、主题管理、审计。  
5. 首期必须支持删除与封禁能力。  

---

## 二、角色与菜单矩阵（首版建议）

### 角色定义

1. `platform_admin`（平台管理员）
2. `space_admin`（空间管理员，按空间范围授权）
3. `normal_user`（普通用户，无管理后台入口）

### 菜单可见性

| 菜单 | platform_admin | space_admin | normal_user |
|---|---|---|---|
| Dashboard 概览 | 可见 | 可见（仅自己管理空间） | 不可见 |
| 用户管理 | 可见 | 不可见 | 不可见 |
| 空间管理 | 可见（全站） | 可见（仅授权空间） | 不可见 |
| 文档管理 | 可见（全站） | 可见（仅授权空间） | 不可见 |
| 主题管理 | 可见 | 可见（仅可应用范围） | 不可见 |
| 系统配置 | 可见 | 不可见 | 不可见 |
| 审计日志 | 可见（全站） | 可见（仅授权空间） | 不可见 |

### 权限动作矩阵（关键动作）

| 动作 | platform_admin | space_admin |
|---|---|---|
| 封禁/解封用户 | 可执行 | 不可执行 |
| 删除用户 | 可执行 | 不可执行 |
| 封禁/解封空间 | 可执行 | 可执行（仅授权空间） |
| 删除空间 | 可执行 | 可执行（仅授权空间） |
| 封禁/解封文档 | 可执行 | 可执行（仅授权空间） |
| 删除文档 | 可执行 | 可执行（仅授权空间） |
| 修改系统配置 | 可执行 | 不可执行 |
| 主题创建/修改/删除 | 可执行 | 可执行（受策略限制） |

---

## 三、数据模型变更计划（迁移拆分）

### Migration A: 管理角色与管理范围

1. 新增 `user_admin_roles`  
字段：`id`, `user_id`, `role`, `created_at`, `updated_at`  
约束：`UNIQUE(user_id, role)`，`role IN ('platform_admin','space_admin')`
2. 新增 `space_admin_scopes`  
字段：`id`, `user_id`, `space_id`, `created_at`, `updated_at`  
约束：`UNIQUE(user_id, space_id)`

### Migration B: 封禁/删除状态（复用现有实体）

1. `users` 新增：`status`, `banned_reason`, `banned_at`, `deleted_at`  
2. `spaces` 新增：`status`, `banned_reason`, `banned_at`, `deleted_at`  
3. `documents` 新增：`status`, `banned_reason`, `banned_at`, `deleted_at`  
约束建议：`status IN ('active','banned','deleted')`

### Migration C: 系统配置与审计

1. 新增 `system_configs`  
字段：`config_key`, `config_value_json`, `updated_by_user_id`, `created_at`, `updated_at`
2. 新增 `audit_logs`  
字段：`audit_log_id`, `actor_user_id`, `action`, `target_type`, `target_id`, `scope_type`, `scope_id`, `request_id`, `detail_json`, `created_at`  
索引：`actor_user_id`, `target_type+target_id`, `scope_type+scope_id`, `created_at`

---

## 四、分阶段里程碑与任务清单

## Milestone 1: 管理后台权限基线（RBAC + Scope）
**Status**: Completed（2026-02-20）  
**Type**: Database + API  
**Estimated**: 2~3 天  
**Files**:
- `apps/server/internal/storage/migrations/*/0005_admin_rbac*.sql`
- `apps/server/internal/storage/models/*.go`（新增 admin role/scope/status 模型）
- `apps/server/internal/storage/repository/interfaces.go`
- `apps/server/internal/storage/repository/gorm_*_repository.go`
- `apps/server/internal/service/*`（admin 权限判定服务）

**Tasks**:
- [x] 新增管理角色、空间管理范围、状态字段迁移（sqlite/postgres/mysql）。
- [x] 封装统一管理权限求值：`IsPlatformAdmin`、`CanManageSpace(spaceID)`。
- [x] 建立 `admin` 路由组基础中间件（401/403 统一错误响应）。
- [x] 新增“软删除/封禁”通用状态检查函数（避免各 handler 重复逻辑）。

**Verification Criteria**:
- [x] 非管理员访问 `/api/admin/*` 返回 `403`。
- [x] `space_admin` 仅能访问授权空间资源。
- [x] `platform_admin` 可访问全站资源。
- [x] 数据库约束生效，非法 role/status 无法入库。

**Exit Criteria**: 管理权限判定在服务层可复用，后续模块只接业务逻辑。

---

## Milestone 2: 管理后台前端壳与路由
**Status**: Completed（2026-02-20）  
**Type**: UI  
**Estimated**: 1~2 天  
**Files**:
- `apps/web/src/admin/AdminApp.tsx`（新增）
- `apps/web/src/admin/routes.tsx`（新增）
- `apps/web/src/admin/layout/AdminLayout.tsx`（新增）
- `apps/web/src/App.tsx`（接入 `/admin/*` 路由）
- `apps/web/src/data-access/http/adapter.ts`（补 admin session/me）

**Tasks**:
- [x] 新增后台入口路由：`/admin/login`、`/admin`。
- [x] 新增后台基础布局：侧边菜单、顶部用户区、面包屑。
- [x] 按角色动态渲染菜单（platform_admin vs space_admin）。
- [x] 未登录/无权限时跳转与错误页处理（401/403）。

**Verification Criteria**:
- [x] 平台管理员登录后看到全菜单。
- [x] 空间管理员登录后只看到授权菜单。
- [x] 普通用户访问后台路由被拒绝。
- [x] 刷新页面后角色和菜单状态可恢复。

**Exit Criteria**: 后台页面框架可承载各业务模块，路由守卫稳定。

---

## Milestone 3: 用户管理（含封禁/删除）
**Status**: Completed（2026-02-20，含审计写入与用户会话吊销）  
**Type**: API + UI  
**Estimated**: 2~3 天  
**Files**:
- `apps/server/internal/server/handler/admin_user.go`（新增）
- `apps/server/internal/service/admin_user_service.go`（新增）
- `apps/server/internal/server/middleware/admin_auth.go`（扩展 `RequirePlatformAdmin`）
- `apps/server/internal/storage/repository/interfaces.go`（扩展 User / Session 仓储契约）
- `apps/server/internal/storage/repository/gorm_user_repository.go`（扩展列表/状态更新/软删除）
- `apps/server/internal/storage/repository/gorm_user_session_repository.go`（扩展按用户批量吊销会话）
- `apps/server/internal/server/router.go`（接入 `/api/admin/users*` 路由）
- `apps/server/internal/server/admin_handler_test.go`（新增用户管理集成测试）
- `apps/web/src/admin/pages/AdminUsersPage.tsx`（新增）
- `apps/web/src/admin/AdminApp.tsx`（接入用户管理页面）
- `apps/web/src/data-access/types.ts`、`apps/web/src/data-access/http/adapter.ts`

**Tasks**:
- [x] 实现用户列表、搜索、分页接口。
- [x] 实现用户封禁/解封接口（记录原因与操作者）。
- [x] 实现用户删除接口（首版软删除，保留审计）。
- [x] 前端实现用户管理列表页和操作确认弹窗。
- [x] 所有关键动作写入审计日志。

**Verification Criteria**:
- [x] 封禁用户后该用户无法登录业务端。
- [x] 删除用户后默认列表不可见（按策略可查历史）。
- [x] 非平台管理员不可调用用户管理写接口。
- [x] 审计日志可看到操作者、动作、目标与时间。

**Exit Criteria**: 用户全生命周期管理能力可用并可追溯。

### Milestone 3 实现补充（2026-02-20）

1. 已新增接口：
   - `GET /api/admin/users`
   - `PATCH /api/admin/users/:userId/status`
   - `DELETE /api/admin/users/:userId`
2. 权限策略：
   - 用户管理全部接口仅允许 `platform_admin` 访问。
   - 拦截管理员自封禁、自删除操作。
3. 会话策略：
   - 封禁、删除用户时立即调用 `RevokeAllByUserID` 吊销全部未吊销会话。
4. 前端能力：
   - `/admin/users` 已支持查询、状态过滤、分页、封禁/解封、删除。
   - 默认筛选不展示已删除用户，可切换到 `deleted` 或 `all` 查看历史记录。

### Milestone 3 踩坑记录（2026-02-20）

1. SQLite `users.created_at/updated_at` 在当前迁移中是 `TEXT`，直接扫描到 `time.Time` 会触发 `unsupported Scan`。  
2. 解决方式：用户列表改为先按字符串读取，再在仓储层统一解析时间，避免仅在 SQLite 下出现 500。  
3. 结论：后续新迁移中建议统一时间字段类型声明（优先 `TIMESTAMP/DATETIME`），并在跨数据库仓储层避免对时间格式做隐式假设。  

---

## Milestone 4: 空间与文档管理（含封禁/删除）
**Status**: Completed（2026-02-20，空间/文档管理含批量、权限边界、删除级联与业务端访问拦截均已验收）  
**Type**: API + UI  
**Estimated**: 3~4 天  
**Files**:
- `apps/server/internal/server/handler/admin_space.go`（新增）
- `apps/server/internal/server/handler/admin_document.go`（新增）
- `apps/server/internal/service/admin_space_service.go`（新增）
- `apps/server/internal/service/admin_document_service.go`（新增）
- `apps/web/src/admin/pages/AdminSpacesPage.tsx`（新增）
- `apps/web/src/admin/pages/documents/*.tsx`（新增）

**Tasks**:
- [x] 实现空间列表/搜索/过滤（含 status、visibility）。
- [x] 实现空间封禁/解封接口（space_admin 限授权范围）。
- [x] 实现空间删除接口（space_admin 限授权范围）。
- [x] 实现空间元数据设置（名称、可见性）。
- [x] 实现文档列表/搜索/过滤（按空间、状态、可见性）。
- [x] 实现文档封禁/解封、删除接口。
- [x] 前端实现空间与文档管理页，支持批量操作。

**Verification Criteria**:
- [x] `space_admin` 只能操作授权空间和其文档。
- [x] `platform_admin` 可跨空间操作。
- [x] 删除空间后符合级联策略（空间软删除时，其下文档同步软删除）。
- [x] 封禁中的空间/文档在业务端访问被拒绝（403）。

**Exit Criteria**: 空间/文档后台治理能力可用，权限边界正确。

### Milestone 4 空间管理补充（2026-02-20）

1. 已新增接口：
   - `GET /api/admin/spaces`
   - `PATCH /api/admin/spaces/:spaceId/status`
   - `PATCH /api/admin/spaces/:spaceId/metadata`
   - `DELETE /api/admin/spaces/:spaceId`
2. 权限策略：
   - `platform_admin` 可管理全站空间。
   - `space_admin` 仅可管理 `space_admin_scopes` 授权空间。
3. 页面能力：
   - `/admin/spaces` 已支持空间列表、创建者（姓名/邮箱）展示、创建时间/更新时间展示。
   - 支持空间封禁/解封（封禁原因必填）与状态原因展示。
   - 支持空间元数据设置（名称、可见性）与软删除。
   - 支持空间多选与批量封禁/解封/删除。
4. 测试覆盖：
   - 已新增后台集成测试覆盖空间封禁/解封、scope 越权拦截、封禁原因校验。
   - 已补充空间删除后的文档级联软删除验证，以及删除后业务端文档访问 `403` 验证。

### Milestone 4 文档管理补充（2026-02-20）

1. 已新增接口：
   - `GET /api/admin/documents`
   - `PATCH /api/admin/documents/:documentId/status`
   - `DELETE /api/admin/documents/:documentId`
2. 权限策略：
   - `platform_admin` 可管理全站文档。
   - `space_admin` 仅可管理授权空间内文档。
3. 页面能力：
   - `/admin/documents` 已支持文档列表、关键词检索、按空间/状态/可见性筛选。
   - 支持文档封禁/解封（封禁原因必填）与软删除。
   - 支持文档多选与批量封禁/解封/删除。
4. 测试覆盖：
   - 已新增后台集成测试覆盖文档列表 scope 过滤、封禁原因校验、越权拦截、软删除状态落库。
   - 已补充文档封禁后业务端访问 `403` 验证。

---

## Milestone 5: 主题管理 + 系统配置
**Status**: Completed（2026-02-20，主题与系统配置前后端已完成并接入审计）  
**Type**: API + UI  
**Estimated**: 2~3 天  
**Files**:
- `apps/server/internal/server/handler/admin_theme.go`（新增）
- `apps/server/internal/server/handler/admin_system_config.go`（新增）
- `apps/server/internal/service/admin_theme_service.go`（新增）
- `apps/server/internal/service/admin_system_config_service.go`（新增）
- `apps/web/src/admin/pages/themes/*.tsx`（新增）
- `apps/web/src/admin/pages/system-config/*.tsx`（新增）

**Tasks**:
- [x] 实现主题的增删改查与启停用（区分 builtin 与 custom）。
- [x] 实现系统配置读写接口（JSON schema 校验 + 版本号）。
- [x] 前端实现主题管理页和系统配置页。
- [x] 系统配置变更写审计，并支持基于 version 的并发控制。

**Verification Criteria**:
- [x] 主题更新后业务端可读取最新主题数据。
- [x] 非平台管理员不可修改系统配置。
- [x] 配置非法值被后端校验拒绝（400）。
- [x] 关键操作均有审计记录。

**Exit Criteria**: 管理后台可独立维护主题和系统配置，不依赖手工改库。

### Milestone 5 后端实现补充（2026-02-20）

1. 迁移与数据结构：
   - 新增 `0006_admin_theme_system_configs`（sqlite/postgres/mysql）。
   - `themes` 增加 `is_enabled` 字段，支持主题启停。
   - 新增 `system_configs` 表（`config_key`、`config_value_json`、`version`、`updated_by_user_id`）。
2. 新增后台主题接口：
   - `GET /api/admin/themes`
   - `POST /api/admin/themes`
   - `PUT /api/admin/themes/:themeId`
   - `DELETE /api/admin/themes/:themeId`
3. 新增后台系统配置接口：
   - `GET /api/admin/system-configs`
   - `PUT /api/admin/system-configs/:key`
4. 策略与约束：
   - `themes`：区分 `builtin/custom`，内置主题不可修改、不可删除；`enabled=false` 不在业务端 `GET /api/themes` 返回结果中。
   - `system-configs`：仅 `platform_admin` 可写；按 `expectedVersion` 做乐观锁版本控制，冲突返回 `409`。
   - 当前预置配置键：`site`、`editor`、`security`，均执行服务端 schema 校验。

### Milestone 5 前端实现补充（2026-02-20）

1. 已接入后台主题管理页 `/admin/themes`：
   - 支持主题列表、关键词过滤、新建、编辑、启停、删除。
   - 对内置主题在 UI 层限制修改/删除/启停，避免误操作。
2. 已接入系统配置页 `/admin/system-configs`：
   - 支持 `site/editor/security` 三类配置键切换。
   - 提供 JSON 编辑器、模板填充、载入线上值、保存配置。
   - 保存时携带 `expectedVersion`，对后端版本冲突（409）直接反馈错误。

---

## Milestone 6: 审计中心与发布前加固
**Status**: Completed（2026-02-20，已完成审计查询、统一审计写入点、防重放 token、权限矩阵与核心 E2E；性能基线按当前决策豁免）  
**Type**: API + UI + Testing  
**Estimated**: 2~3 天  
**Files**:
- `apps/server/internal/server/handler/admin_audit.go`（新增）
- `apps/server/internal/service/audit_service.go`（新增）
- `apps/server/internal/server/middleware/*.go`（审计注入）
- `apps/web/src/admin/pages/audits/*.tsx`（新增）
- `apps/server/internal/server/*_test.go`（新增）

**Tasks**:
- [x] 实现审计查询接口（按 actor/action/target/scope/time 过滤）。
- [x] 建立统一审计事件写入点（中间件 + service）。
- [x] 对高风险操作增加二次确认与防重放 token。
- [x] 补齐权限矩阵测试与核心 E2E 流程。

**Verification Criteria**:
- [x] 任一封禁/删除操作都能被审计检索。
- [x] 权限绕过测试失败率为 0（无越权）。
- [x] 后台关键路径性能满足可用基线（列表与搜索可用，首期按决策豁免压测阻塞）。
- [x] 发布清单完成（配置、回滚、监控、告警）。

**Exit Criteria**: 管理后台达到首期可上线标准。

### Milestone 6 已落地补充（2026-02-20）

1. 新增一次性高风险操作 token 能力：
   - `POST /api/admin/operation-tokens` 用于签发短时有效（默认 2 分钟）的一次性 token。
   - token 绑定 `actor + operation + targetType + targetId`，消费后立即失效，重复使用返回冲突错误。
2. 高风险路由已接入 `RequireAdminOperationToken` 校验中间件：
   - 用户：封禁/解封、删除
   - 空间：封禁/解封、删除
   - 文档：封禁/解封、删除
   - 主题：更新、删除
   - 系统配置：更新
3. 前端 HTTP 网关已自动在高风险请求前申请 token 并附带 `X-Admin-Operation-Token` 请求头。
4. 已新增后端测试覆盖：
   - 无 token 请求被拒绝（400）
   - token 与目标不匹配被拒绝（409）
   - token 仅可使用一次，重放被拒绝（409）
5. 已新增统一审计写入点：
   - 新增 `AttachAdminAuditContext` 中间件，在 `RequireAdmin` 后统一把 `actor_user_id` 和 `request_id` 注入请求 context。
   - `AdminAuditService.Record` 支持在 `RecordAdminAuditInput` 未显式传入 `ActorUserID/RequestID` 时自动从 context 补齐。
   - 用户/空间/文档/主题/系统配置服务的审计调用已去除重复传参，统一走 context 写入。
6. 已补齐权限矩阵与核心 E2E 覆盖：
   - 新增 operation token 签发权限矩阵测试（未登录/普通用户/space_admin/platform_admin）。
   - 新增 operation token 跨管理员复用拦截测试（actor 绑定）。
   - 新增审计 request_id 上下文透传测试（验证统一写入点生效）。
7. 已新增发布清单文档：`docs/ADMIN_CONSOLE_RELEASE_CHECKLIST.md`（配置、回滚、监控、告警、验收与发布记录）。

---

## 五、接口清单（首期最小集合）

1. `GET /api/admin/me`  
2. `POST /api/admin/operation-tokens`  
3. `GET /api/admin/users`  
4. `PATCH /api/admin/users/:userId/status`  
5. `DELETE /api/admin/users/:userId`  
6. `GET /api/admin/spaces`  
7. `PATCH /api/admin/spaces/:spaceId/status`  
8. `DELETE /api/admin/spaces/:spaceId`  
9. `GET /api/admin/documents`  
10. `PATCH /api/admin/documents/:documentId/status`  
11. `DELETE /api/admin/documents/:documentId`  
12. `GET /api/admin/themes`  
13. `POST /api/admin/themes`  
14. `PUT /api/admin/themes/:themeId`  
15. `DELETE /api/admin/themes/:themeId`  
16. `GET /api/admin/system-configs`  
17. `PUT /api/admin/system-configs/:key`  
18. `GET /api/admin/audits`

---

## 六、任务执行顺序（建议）

1. 先做 Milestone 1（RBAC + scope + 状态迁移），这是所有模块的前置。  
2. 再做 Milestone 2（后台路由壳），保证可并行开发页面。  
3. Milestone 3/4 并行推进（用户管理 与 空间文档管理）。  
4. 完成后接 Milestone 5（主题与系统配置）。  
5. 最后 Milestone 6（审计中心与上线加固）。  

---

## 七、你可继续补充的决策项

1. 删除策略：仅软删除。
2. 封禁策略：封禁立即踢出在线会话。  
3. 审计保留时长：例如 180 天。  
4. 主题管理权限：`space_admin` 允许创建全局主题。  
5. 系统配置版本回滚：不需要“一键回滚到上一个版本”。

---

## 八、Definition of Done（首期）

1. 两类管理员登录后均可进入后台，菜单按权限准确显示。  
2. 六个模块均可用：用户、空间、文档、主题、系统配置、审计。  
3. 封禁与删除能力在线上可执行，且具备完整审计追踪。  
4. 所有越权访问被拦截，关键接口覆盖集成测试。  
5. 具备上线与回滚预案，可在生产环境灰度发布。  

---

## 九、前端确认交互规范（2026-02-20）

1. 禁止在前端代码中使用浏览器原生确认/输入弹框（`window.confirm`、`window.prompt`）。  
2. 确认类交互必须使用项目模态窗能力：
   - 通用编辑器与工作区：`apps/web/src/components/ConfirmDialog.tsx`（`useConfirmDialog`）。
   - 管理后台：`apps/web/src/admin/components/AdminDialogs.tsx`（`useAdminDialogs`）。  
3. 高风险动作（删除、封禁）必须使用 `danger` 视觉语义，并写明影响范围（例如“会删除子节点”）。  
4. 业务流程中必须以 `await confirm(...)` 控制后续动作，未确认直接 `return`，避免误触发写操作。  
5. 新增功能若涉及确认交互，默认复用上述组件，不得引入新的浏览器原生弹框实现。  
