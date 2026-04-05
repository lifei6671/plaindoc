# 后台普通用户访问控制 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让已登录但不是 `platform_admin` / `space_admin` 的普通用户也能进入后台页，并按“个人信息 / 分享中心 / 空间成员 / 空间管理员”四种视图展示不同能力。

**Architecture:** 后端先把“是否已登录”和“是否具备管理角色”拆开，`/api/admin/me` 与个人资料接口只要求登录态，分享中心对所有登录用户开放但只允许普通用户操作自己创建的分享，空间列表接口则根据角色返回管理员视图或成员视图。前端再基于 `/api/admin/me` 返回的能力摘要构建后台菜单，并在分享中心和空间管理页分别按自助模式 / 成员模式 / 管理员模式切换右侧操作区。

**Tech Stack:** Go 1.26、Gin、GORM、React 19、TypeScript、Vite 7、Vitest、Testing Library

---

### Task 1: 后端登录态与能力摘要拆分

**Files:**
- Modify: `apps/server/internal/server/middleware/admin_auth.go`
- Modify: `apps/server/internal/server/handler/admin.go`
- Modify: `apps/server/internal/server/handler/admin_profile.go`
- Modify: `apps/server/internal/server/handler/admin_space.go`
- Modify: `apps/server/internal/server/router.go`
- Modify: `apps/server/internal/service/admin_space_service.go`
- Modify: `apps/server/internal/service/admin_access_service.go`
- Test: `apps/server/internal/server/admin_handler_test.go`

- [ ] **Step 1: 写失败测试**

```go
// 1) 未登录请求 /api/admin/me 与 /api/admin/profile 仍应被拒绝。
// 2) 已登录普通用户请求 /api/admin/me 应返回 capabilities，而不是权限错误。
// 3) 已登录普通用户请求 /api/admin/document-shares?view=mine 应返回自己的分享。
// 4) 已登录普通用户请求 /api/admin/spaces?page=1&pageSize=20 应返回其成员空间列表。
// 5) 普通用户请求 /api/admin/users 仍应被拒绝。
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -timeout 60s ./internal/server -run 'TestRouter_AdminShellAccessControl|TestAdminHandler_MeReturnsCapabilities|TestAdminSpaceList_AllowsLoggedInMembers'`

Expected: FAIL，失败点会集中在 `RequireAdmin` 仍然拦住普通用户，或 `me` 响应缺少能力字段。

- [ ] **Step 3: 写最小实现**

```go
// 1) 新增“仅要求登录”的后台中间件，给 /api/admin/me、/api/admin/profile 和成员态空间列表复用。
// 2) /api/admin/me 返回 roles + capabilities，普通用户可通过 hasSpaceMembership 判断是否展示空间管理。
// 3) 分享中心列表按视图分流：普通用户仅允许 mine，且只允许创建者本人改码/延期/设永久/取消。
// 4) 空间列表服务按角色分流：管理员走现有管理视图，普通用户走 ListByUserID 成员视图。
// 5) 保留所有敏感路由的管理员级别守卫，不放宽高风险操作。
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -timeout 60s ./internal/server -run 'TestRouter_AdminShellAccessControl|TestAdminHandler_MeReturnsCapabilities|TestAdminSpaceList_AllowsLoggedInMembers'`

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add apps/server/internal/server/middleware/admin_auth.go apps/server/internal/server/handler/admin.go apps/server/internal/server/handler/admin_profile.go apps/server/internal/server/handler/admin_space.go apps/server/internal/server/router.go apps/server/internal/service/admin_space_service.go apps/server/internal/service/admin_access_service.go apps/server/internal/server/admin_handler_test.go
git commit -m "feat: loosen admin shell access for logged-in users"
```

### Task 2: 后台壳页菜单与身份模型

**Files:**
- Modify: `apps/web/src/data-access/types.ts`
- Modify: `apps/web/src/data-access/http/adapter.ts`
- Modify: `apps/web/src/admin/AdminApp.tsx`
- Test: `apps/web/src/admin/AdminApp.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
// 1) 普通登录用户的 /admin 页面至少显示“个人信息 + 分享中心/我的分享”。
// 2) 空间成员用户的 /admin 页面显示“个人信息 + 分享中心/我的分享 + 空间管理”。
// 3) `platform_admin` / `space_admin` 仍显示现有完整菜单。
```

- [ ] **Step 2: 运行测试确认失败**

Run: `npm run test:run -- AdminApp.test.tsx AdminApp.share-menu.test.tsx`

Expected: FAIL，当前实现会把非管理员直接拦到“无管理后台权限”。

- [ ] **Step 3: 写最小实现**

```tsx
// 1) 给 AdminIdentity 增加后端返回的能力摘要字段。
// 2) buildAdminMenu 改成基于“角色 + 能力视图”生成菜单，而不是只看管理角色。
// 3) 普通用户不再进入错误页，而是自动落到个人信息页和分享中心的“我的分享”。
// 4) 菜单与页面切换逻辑保持单一来源，避免左侧菜单、移动端下拉与默认路由不一致。
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npm run test:run -- AdminApp.test.tsx`

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add apps/web/src/data-access/types.ts apps/web/src/data-access/http/adapter.ts apps/web/src/admin/AdminApp.tsx apps/web/src/admin/AdminApp.test.tsx
git commit -m "feat: allow logged-in users into admin shell"
```

### Task 3: 分享中心与空间管理页成员模式

**Files:**
- Modify: `apps/web/src/admin/pages/AdminSpacesPage.tsx`
- Test: `apps/web/src/admin/pages/AdminSpacesPage.test.tsx`

- [ ] **Step 1: 写失败测试**

```tsx
// 1) 普通用户的分享中心仅展示“我的分享”。
// 2) 普通用户在“我的分享”里只对自己创建的分享显示行内操作。
// 3) 成员模式下，空间列表右侧只显示“编辑文档”。
// 4) 成员模式下，不显示“新建空间 / 分类管理 / 批量封禁 / 批量删除”等高风险入口。
// 5) 管理员模式下，现有完整按钮仍存在。
// 6) “编辑文档”被折叠进“设置”子菜单后，菜单结构测试要通过。
```

- [ ] **Step 2: 运行测试确认失败**

Run: `npm run test:run -- AdminDocumentSharesPage.test.tsx AdminSpacesPage.test.tsx`

Expected: FAIL，当前空间页仍会展示完整操作区。

- [ ] **Step 3: 写最小实现**

```tsx
// 1) 让分享中心和空间页接收后台能力模式，按 self/member/admin 三种模式渲染右侧操作区。
// 2) 分享中心的 self 模式只保留“我的分享”的自助操作，所有非本人分享统一隐藏。
// 3) 成员模式只保留编辑文档入口，所有管理动作统一隐藏，不做禁用态露出。
// 4) 管理员模式维持现有设置、成员、状态、转让、删除能力。
// 5) 将“编辑文档”并入“设置”子菜单，保持入口语义和视觉结构一致。
```

- [ ] **Step 4: 运行测试确认通过**

Run: `npm run test:run -- AdminDocumentSharesPage.test.tsx AdminSpacesPage.test.tsx`

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add apps/web/src/admin/pages/AdminSpacesPage.tsx apps/web/src/admin/pages/AdminSpacesPage.test.tsx
git commit -m "feat: simplify admin spaces actions for members"
```

### Task 4: 文档与全量验证

**Files:**
- Modify: `docs/BACKEND_DEVELOPER_GUIDE.md`
- Modify: `docs/FRONTEND_DEVELOPER_GUIDE.md`

- [ ] **Step 1: 补充文档**

```md
说明后台壳页新增“登录用户可进入、按能力视图展示”的规则。
说明分享中心对普通用户开放但仅支持自助分享操作，以及空间成员态与管理员态的菜单差异。
说明后端仍然会兜底拒绝高风险接口。
```

- [ ] **Step 2: 运行格式化与测试**

Run: `gofmt -w .`

Run: `go test -timeout 60s ./...`

Run: `npm exec -- tsc -b`

Run: `npm run web:build`

Expected: 全部通过；如果仓库已有无关失败，需要记录失败文件与原因，不回滚本次改动。

- [ ] **Step 3: 提交**

```bash
git add docs/BACKEND_DEVELOPER_GUIDE.md docs/FRONTEND_DEVELOPER_GUIDE.md
git commit -m "docs: describe admin backend access control"
```
