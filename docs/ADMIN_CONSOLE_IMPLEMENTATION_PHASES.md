# Implementation Phases: 管理后台（Admin Console）

**Project Type**: 现有项目新增管理后台（全站管理 + 空间管理）  
**Scope**: 用户管理 + 空间管理 + 文档管理 + 系统配置 + 主题管理 + 审计  
**Stack**: Web(React + Vite + React Router) + Server(Gin + GORM + SQL Migrations)  
**Estimated Total**: 14~20 天（按 2 周冲刺可拆为两期上线）

---

## 最新进展（2026-02-20）

1. Milestone 1 已完成（RBAC + Scope + 状态迁移 + admin 鉴权中间件）。  
2. 已新增后台基础接口：
   - `GET /api/admin/me`
   - `GET /api/admin/spaces/:spaceId/check`
3. 已完成后台权限矩阵基础测试（`platform_admin`、`space_admin`、非管理员）。  

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
**Type**: UI  
**Estimated**: 1~2 天  
**Files**:
- `apps/web/src/admin/AdminApp.tsx`（新增）
- `apps/web/src/admin/routes.tsx`（新增）
- `apps/web/src/admin/layout/AdminLayout.tsx`（新增）
- `apps/web/src/App.tsx`（接入 `/admin/*` 路由）
- `apps/web/src/data-access/http/adapter.ts`（补 admin session/me）

**Tasks**:
- [ ] 新增后台入口路由：`/admin/login`、`/admin`。
- [ ] 新增后台基础布局：侧边菜单、顶部用户区、面包屑。
- [ ] 按角色动态渲染菜单（platform_admin vs space_admin）。
- [ ] 未登录/无权限时跳转与错误页处理（401/403）。

**Verification Criteria**:
- [ ] 平台管理员登录后看到全菜单。
- [ ] 空间管理员登录后只看到授权菜单。
- [ ] 普通用户访问后台路由被拒绝。
- [ ] 刷新页面后角色和菜单状态可恢复。

**Exit Criteria**: 后台页面框架可承载各业务模块，路由守卫稳定。

---

## Milestone 3: 用户管理（含封禁/删除）
**Type**: API + UI  
**Estimated**: 2~3 天  
**Files**:
- `apps/server/internal/server/handler/admin_user.go`（新增）
- `apps/server/internal/service/admin_user_service.go`（新增）
- `apps/server/internal/storage/repository/gorm_user_repository.go`（扩展）
- `apps/web/src/admin/pages/users/*.tsx`（新增）
- `apps/web/src/data-access/types.ts`、`apps/web/src/data-access/http/adapter.ts`

**Tasks**:
- [ ] 实现用户列表、搜索、分页接口。
- [ ] 实现用户封禁/解封接口（记录原因与操作者）。
- [ ] 实现用户删除接口（首版软删除，保留审计）。
- [ ] 前端实现用户管理列表页和操作确认弹窗。
- [ ] 所有关键动作写入审计日志。

**Verification Criteria**:
- [ ] 封禁用户后该用户无法登录业务端。
- [ ] 删除用户后默认列表不可见（按策略可查历史）。
- [ ] 非平台管理员不可调用用户管理写接口。
- [ ] 审计日志可看到操作者、动作、目标与时间。

**Exit Criteria**: 用户全生命周期管理能力可用并可追溯。

---

## Milestone 4: 空间与文档管理（含封禁/删除）
**Type**: API + UI  
**Estimated**: 3~4 天  
**Files**:
- `apps/server/internal/server/handler/admin_space.go`（新增）
- `apps/server/internal/server/handler/admin_document.go`（新增）
- `apps/server/internal/service/admin_space_service.go`（新增）
- `apps/server/internal/service/admin_document_service.go`（新增）
- `apps/web/src/admin/pages/spaces/*.tsx`（新增）
- `apps/web/src/admin/pages/documents/*.tsx`（新增）

**Tasks**:
- [ ] 实现空间列表/搜索/过滤（含 status、visibility）。
- [ ] 实现空间封禁/解封、删除接口（space_admin 限授权范围）。
- [ ] 实现文档列表/搜索/过滤（按空间、状态、可见性）。
- [ ] 实现文档封禁/解封、删除接口。
- [ ] 前端实现空间与文档管理页，支持批量操作。

**Verification Criteria**:
- [ ] `space_admin` 只能操作授权空间和其文档。
- [ ] `platform_admin` 可跨空间操作。
- [ ] 删除空间后符合级联策略（节点/文档状态一致）。
- [ ] 封禁中的空间/文档在业务端访问被拒绝（403）。

**Exit Criteria**: 空间/文档后台治理能力可用，权限边界正确。

---

## Milestone 5: 主题管理 + 系统配置
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
- [ ] 实现主题的增删改查与启停用（区分 builtin 与 custom）。
- [ ] 实现系统配置读写接口（JSON schema 校验 + 版本号）。
- [ ] 前端实现主题管理页和系统配置页。
- [ ] 系统配置变更写审计，并支持查看最近版本。

**Verification Criteria**:
- [ ] 主题更新后业务端可读取最新主题数据。
- [ ] 非平台管理员不可修改系统配置。
- [ ] 配置非法值被后端校验拒绝（400）。
- [ ] 关键操作均有审计记录。

**Exit Criteria**: 管理后台可独立维护主题和系统配置，不依赖手工改库。

---

## Milestone 6: 审计中心与发布前加固
**Type**: API + UI + Testing  
**Estimated**: 2~3 天  
**Files**:
- `apps/server/internal/server/handler/admin_audit.go`（新增）
- `apps/server/internal/service/audit_service.go`（新增）
- `apps/server/internal/server/middleware/*.go`（审计注入）
- `apps/web/src/admin/pages/audits/*.tsx`（新增）
- `apps/server/internal/server/*_test.go`（新增）

**Tasks**:
- [ ] 实现审计查询接口（按 actor/action/target/scope/time 过滤）。
- [ ] 建立统一审计事件写入点（中间件 + service）。
- [ ] 对高风险操作增加二次确认与防重放 token。
- [ ] 补齐权限矩阵测试与核心 E2E 流程。

**Verification Criteria**:
- [ ] 任一封禁/删除操作都能被审计检索。
- [ ] 权限绕过测试失败率为 0（无越权）。
- [ ] 后台关键路径性能满足可用基线（列表与搜索可用）。
- [ ] 发布清单完成（配置、回滚、监控、告警）。

**Exit Criteria**: 管理后台达到首期可上线标准。

---

## 五、接口清单（首期最小集合）

1. `GET /api/admin/me`  
2. `GET /api/admin/users`  
3. `PATCH /api/admin/users/:userId/status`  
4. `DELETE /api/admin/users/:userId`  
5. `GET /api/admin/spaces`  
6. `PATCH /api/admin/spaces/:spaceId/status`  
7. `DELETE /api/admin/spaces/:spaceId`  
8. `GET /api/admin/documents`  
9. `PATCH /api/admin/documents/:documentId/status`  
10. `DELETE /api/admin/documents/:documentId`  
11. `GET /api/admin/themes`  
12. `POST /api/admin/themes`  
13. `PUT /api/admin/themes/:themeId`  
14. `DELETE /api/admin/themes/:themeId`  
15. `GET /api/admin/system-configs`  
16. `PUT /api/admin/system-configs/:key`  
17. `GET /api/admin/audits`

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
