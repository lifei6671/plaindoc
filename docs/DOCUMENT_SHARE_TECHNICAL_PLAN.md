# 文档单页分享（公开/密码）技术方案

**文档状态**: Completed（已完成）  
**创建日期**: 2026-03-05  
**完成日期**: 2026-03-05  
**适用范围**: `apps/server`、`apps/web`（编辑器、阅读 SSR、后台内容管理）  
**目标**: 支持文档单页分享，支持公开分享与密码分享，支持后台“分享中心”（分享管理 + 我的分享）。

---

## 1. 需求确认（含补充）

1. 编辑文档时，可从文档右侧子菜单配置“分享文档”，已分享的文档，子菜单前面打对号。
2. 分享访问不受空间/文档 `visibility` 影响（但受文档/空间实体状态影响）。
3. 分享页是前端单页阅读，并支持：
   - 导出 Markdown
   - 导出 PDF（浏览器打印）
   - 下载/预览附件
4. 支持密码分享，密码最小长度 6 位。
5. 用户输入过密码后，再次进入同一分享页可免输入密码。
6. 分享 URL 与当前阅读 URL 兼容：
   - 文档配置了 `reader_slug`（文档标识）时，分享路径可直接用该标识访问。
7. 支持公开分享：
   - 可不设置密码
   - 可不设置有效期（永久，直到手动取消）
8. 后台内容管理新增“分享管理”：
   - `space_admin` 仅可查看其 scope 内空间的分享文档
   - `platform_admin` 可查看全站分享文档
9. 后台“分享中心”新增“我的分享”视图：
   - 与“分享管理”同权限边界（`platform_admin` 全站、`space_admin` scope）
   - 仅查看“当前登录管理员创建的分享文档”
   - 支持取消分享、延长时间、修改密码、切换公开/密码模式

---

## 2. 现状与可复用能力

1. 文档路由已支持 `docKey` 兼容解析（`document_id / node_id / reader_slug`），可复用到分享路由。
2. 编辑器文档操作入口已在 `WorkspaceTree`，适合挂“分享设置”模态弹窗。
3. 阅读页已有单页导出能力（Markdown/PDF）与附件访问链路，可复用渲染逻辑与 UI。
4. 后台已有 scope 权限模型（`platform_admin` + `space_admin(scope)`）与内容管理页框架，可按同模式扩展为“分享中心”双视图。

---

## 3. 总体设计

### 3.1 分享模式

每个文档最多一个“当前分享配置”，状态如下：

1. `disabled`：未分享/已取消分享
2. `public`：公开分享（可无密码）
3. `password`：密码分享（密码必填，最小 6 位）

有效期规则：

1. `expires_at = NULL`：永久有效（直到手动取消）
2. `expires_at != NULL`：到期失效

### 3.2 分享访问边界

分享访问不走 `VisibilityService.GetDocument`，改为：

1. 文档存在且 `active`
2. 所属空间存在且 `active`
3. 文档存在有效分享配置（未禁用、未过期）

未命中时统一返回“页面不存在/不可访问”，避免泄露文档存在性。

### 3.3 URL 兼容策略

新增分享页路由：

1. `GET /s/:spaceId/:docKey`

其中 `docKey` 解析规则与阅读页一致（`document_id / node_id / reader_slug`）。

canonical 策略：

1. 若文档存在 `reader_slug`，canonical 固定为 `/s/:spaceId/:reader_slug`
2. 访问了旧 key 时返回 `303` 到 canonical

说明：

1. 当前系统暂无 space slug，分享路径中的 `spaceId` 仍为 `space_id`。
2. 为后续 space 路由扩展预留 `resolveSpaceRouteKey` 抽象。

---

## 4. 数据模型

### 4.1 `document_shares`（新增）

字段建议：

1. `share_id`（ULID，唯一）
2. `document_id`（唯一）
3. `space_id`（冗余，便于后台 scope 查询）
4. `mode`（`public`/`password`）
5. `password_hash`（可空，`bcrypt`）
6. `password_hint`（可空）
7. `expires_at`（可空）
8. `disabled_at`（可空）
9. `access_version`（int，默认 1；密码变更/取消分享时递增，用于免密失效）
10. `created_by_user_id`、`updated_by_user_id`
11. `created_at`、`updated_at`

索引建议：

1. `uk_document_shares_document_id`
2. `idx_document_shares_space_id_disabled_expires`
3. `idx_document_shares_mode`

---

## 5. 免密方案（密码验证后再次访问免输）

### 5.1 机制

密码验证成功后下发分享访问 Cookie（HttpOnly）：

1. Cookie 名：`pd_share_access_<share_id_suffix>`
2. 值：签名 claims（含 `share_id`、`access_version`、`exp`）
3. `SameSite=Lax`、`HttpOnly`、HTTPS 下 `Secure=true`

### 5.2 生效与失效

免密通过条件：

1. Cookie 签名合法
2. 未过期
3. `access_version` 与当前分享记录一致

自动失效场景：

1. 分享取消
2. 密码修改
3. 分享模式变更（公开 <-> 密码）
4. 分享过期

实现方式：上述操作统一递增 `access_version`。

---

## 6. 后端接口设计

### 6.1 编辑器文档分享配置 API

1. `GET /api/docs/:docId/share`
   - 返回当前分享配置（不返回明文密码）
2. `PUT /api/docs/:docId/share`
   - 入参：
     - `enabled`（bool）
     - `mode`（`public|password`）
     - `password`（`mode=password` 时必填，最小 6 位；其余可空）
     - `passwordHint`（可空）
     - `expiresAt`（可空）
3. `DELETE /api/docs/:docId/share`
   - 取消分享（写 `disabled_at` + `access_version++`）

权限：复用文档可写权限（owner/collaborator/管理员）。

### 6.2 分享页访问 API

1. `GET /s/:spaceId/:docKey`
   - 未通过密码校验时：返回密码页
   - 已通过（或公开分享）：返回分享阅读页 SSR
2. `POST /s/:spaceId/:docKey/verify`
   - 校验密码成功后写免密 Cookie

### 6.3 分享态附件访问 API（新增）

1. `POST /api/shares/:spaceId/:docKey/attachments/:attachmentId/access-link`
   - 目的：下载或预览
   - 鉴权：公开分享或密码分享已通过（含免密 Cookie）
2. `GET /api/shares/:spaceId/:docKey/attachments/:attachmentId/download`
   - 便于无脚本和导出文件链接

---

## 7. 分享页能力（导出 + 附件）

分享页复用阅读页渲染体系，保留：

1. 导出 Markdown
2. 导出 PDF
3. 附件下载/预览

差异点：

1. 不显示空间树与无关导航
2. 附件访问走“分享态附件 API”
3. 页面缓存策略默认 `private, no-store`
4. 完全公开的页面缓存策略和原策略保持一致

---

## 8. 后台“分享中心”（分享管理 + 我的分享）

### 8.1 功能

在后台内容管理新增“分享中心”页，包含两个视图（Tab）：

1. `分享管理`（`view=all`）：查询并管理“当前管理员权限范围内”的所有分享文档
2. `我的分享`（`view=mine`）：查询并管理“当前登录管理员创建的分享文档”
3. 通用筛选：空间、分享模式、是否过期、关键词
4. 通用动作：取消分享、延长有效期、修改密码、切换公开/密码

### 8.2 权限

1. `platform_admin`
   - `view=all`：全站可见
   - `view=mine`：全站范围内仅本人创建
2. `space_admin`
   - `view=all`：仅 `scope` 空间可见
   - `view=mine`：在 `scope` 内仅本人创建

API 建议：

1. `GET /api/admin/document-shares?view=all|mine`
2. `PATCH /api/admin/document-shares/:shareId`
3. `DELETE /api/admin/document-shares/:shareId`

---

## 9. 视图与接口约束补充

同一后台接口支持两类视图，不再新增前台 `/api/me/*`：

1. `view=all`
   - 作用：分享管理视图
   - 过滤：按管理员权限范围返回可管理分享
2. `view=mine`
   - 作用：我的分享视图
   - 过滤：在 `view=all` 结果基础上追加 `created_by_user_id = actor_user_id`

API 建议：

1. `GET /api/admin/document-shares?view=all|mine`
2. `PATCH /api/admin/document-shares/:shareId`
3. `DELETE /api/admin/document-shares/:shareId`

权限：当前用户必须在管理员权限范围内且对目标文档具备可写权限。

---

## 10. 错误码与校验补充

新增建议错误码：

1. `DOCUMENT_SHARE_NOT_FOUND`
2. `DOCUMENT_SHARE_EXPIRED`
3. `DOCUMENT_SHARE_DISABLED`
4. `DOCUMENT_SHARE_PASSWORD_REQUIRED`
5. `DOCUMENT_SHARE_PASSWORD_INVALID`
6. `DOCUMENT_SHARE_PASSWORD_TOO_SHORT`
7. `DOCUMENT_SHARE_ACCESS_DENIED`

密码校验：

1. 最小 6 位（沿用现有密码最低策略）
2. 空密码仅允许在 `mode=public`

---

## 11. 测试计划

### 11.1 后端集成测试

1. 分享创建/更新/取消权限校验（owner/collaborator/reader/outsider）
2. 分享页访问：
   - 公开分享可直接访问
   - 密码分享需验证
   - 验证后免密再次访问
3. 密码变更后旧免密 Cookie 失效
4. 过期分享访问失败
5. `reader_slug` 路径可打开分享，旧 key 重定向到 canonical
6. 后台列表 scope 过滤正确

### 11.2 前端与 E2E

1. `WorkspaceTree` 分享弹窗流程
2. 分享页导出 Markdown/PDF 正常
3. 分享页附件下载/预览正常
4. 后台“分享中心”双视图（`all/mine`）列表与管理动作完整

---

## 12. 实施顺序

1. Phase A：迁移 + model + repository + service
2. Phase B：编辑器文档分享 API + 分享页路由
3. Phase C：免密 Cookie 机制 + 分享态附件接口
4. Phase D：前端分享页与分享设置弹窗
5. Phase E：后台“分享中心”双视图（`分享管理 + 我的分享`）
6. Phase F：回归测试与文档收敛

---

## 13. 风险与控制

1. 风险：分享绕过可见性后可能被路径枚举探测  
   控制：未分享/无效统一返回、密码接口限流、失败提示模糊化。
2. 风险：免密状态在密码修改后未及时失效  
   控制：`access_version` 统一递增并校验。
3. 风险：分享态附件链路与现有附件链路割裂  
   控制：复用现有附件服务，仅新增“分享态鉴权入口”。

---

## 14. 实施完成记录

1. 已按 Phase A-F 完成开发与联调。
2. 验证结果（2026-03-05）：
   - `apps/server`：`go test ./...` 通过
   - `apps/web`：`npm run build` 通过
   - `apps/web`：`npm run test:run -- src/components/WorkspaceTree.test.tsx` 通过
