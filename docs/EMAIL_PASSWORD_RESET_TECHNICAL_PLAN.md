# 邮箱配置与密码重置技术方案

**文档状态**: Implemented（已实现）  
**创建日期**: 2026-03-03  
**适用范围**: `apps/server`、`apps/web`  
**目标**: 支持后台邮箱配置、用户邮箱找回密码、管理员发起重置密码邮件。

---

## 1. 需求范围

1. 后台新增邮箱配置能力（SMTP）。
2. 前端登录页支持“忘记密码（邮箱找回）”。
3. 后台用户管理支持管理员发起“发送重置密码邮件”。
4. 用户通过邮件链接进入重置页，设置新密码后生效。

非目标（本期不做）：

1. 通用通知中心（仅覆盖密码重置邮件场景）。
2. 短信找回、多因子重置流程。
3. 邮件模板可视化编辑器。（需要预留扩展点）

---

## 2. 方案总览

采用“系统配置 + 令牌表 + 邮件发送器 + 双入口重置流程”实现：

1. 配置层：复用 `system_configs`，新增配置键 `email`。
2. 存储层：新增 `password_reset_tokens` 表管理一次性重置令牌。
3. 服务层：新增 `PasswordResetService`、`MailSender` 抽象、`EmailConfigService`。
4. API 层：
   - 公开认证接口：申请重置、校验令牌、确认重置。
   - 管理接口：管理员触发重置邮件发送。
5. 前端：新增 `forgot-password`、`reset-password` 页面；登录页与后台用户页接入入口。
6. 安全：令牌哈希存储、TTL、一次性消费、会话失效、防枚举、防重放。

---

## 3. 后端设计

### 3.1 数据库设计

新增表：`password_reset_tokens`（sqlite/mysql/postgres 三套迁移）。

建议字段：

1. `id`：自增主键。
2. `token_id`：业务 ID（ULID，唯一）。
3. `token_secret_hash`：令牌密文哈希（HMAC-SHA256）。
4. `user_id`：目标用户 `user_id`。
5. `source`：`self_service` / `admin_initiated`。
6. `requested_by_user_id`：管理员触发时记录操作者，否则为空。
7. `request_ip_hash`：请求 IP 哈希（可为空）。
8. `expires_at`：过期时间。
9. `consumed_at`：已消费时间（可空）。
10. `invalidated_at`：失效时间（可空，生成新 token 时批量失效旧 token）。
11. `created_at`、`updated_at`。

建议索引：

1. `uk_password_reset_tokens_token_id`（唯一）。
2. `idx_password_reset_tokens_user_active`：`(user_id, consumed_at, invalidated_at, expires_at)`。
3. `idx_password_reset_tokens_expires_at`：`(expires_at)`。
4. `idx_password_reset_tokens_request_ip_created`：`(request_ip_hash, created_at)`。

### 3.2 系统配置设计

在 `AdminSystemConfigService` 中新增 `email` 配置校验器与掩码逻辑。

建议配置结构：

```json
{
  "enabled": true,
  "fromName": "PlainDoc",
  "fromEmail": "no-reply@example.com",
  "replyTo": "",
  "appBaseUrl": "https://docs.example.com",
  "passwordReset": {
    "tokenTTLMinutes": 30,
    "minRequestIntervalSeconds": 60,
    "maxRequestsPerHourPerEmail": 5,
    "maxRequestsPerHourPerIP": 20
  },
  "smtp": {
    "host": "smtp.example.com",
    "port": 587,
    "username": "smtp-user",
    "passwordCiphertext": "********",
    "security": "starttls",
    "connectTimeoutMs": 3000,
    "sendTimeoutMs": 5000
  }
}
```

密钥字段处理：

1. `smtp.passwordCiphertext` 列表返回时统一掩码 `********`。
2. 更新时若传 `********`，复用库内旧值；若传新值，覆盖旧值。
3. 与 `auth.providers[].ldap.bindPasswordCiphertext` 的现有策略保持一致。

### 3.3 邮件发送抽象

新增接口：

1. `MailSender.Send(ctx, MailMessage) error`
2. 默认实现：`SMTPMailSender`（支持 `plain` / `starttls` / `tls`）。
3. 失败仅返回业务错误，不泄露底层认证细节到前端。

建议补充测试接口：

1. `POST /api/admin/system-configs/email/test-send`
2. 支持“草稿配置 + 收件人邮箱”测试，便于上线前验证。

### 3.4 密码重置核心服务

新增 `PasswordResetService`，职责：

1. 发起重置（自助/管理员两类入口）。
2. 生成并存储一次性令牌。
3. 发送重置邮件。
4. 校验令牌。
5. 完成重置（更新密码、消费令牌、吊销用户会话）。

关键规则：

1. 令牌有效期默认 30 分钟（可配置）。
2. 同用户生成新令牌时，失效旧未消费令牌。
3. 令牌一次性消费，重复提交返回“无效或已过期”。
4. 重置成功后调用 `userSessionRepo.RevokeAllByUserID(...)`。
5. 密码强度沿用当前策略（至少 6 位），可扩展更严格规则。

### 3.5 认证与管理员 API 设计

公开认证接口（无需登录）：

1. `POST /api/auth/password-reset/request`
   - 入参：`{ "email": "user@example.com" }`
   - 返回：统一成功文案（防邮箱枚举）。
2. `POST /api/auth/password-reset/verify`
   - 入参：`{ "token": "raw_token" }`
   - 返回：`{ "valid": true, "expiresAt": "..." }` 或统一失败码。
3. `POST /api/auth/password-reset/confirm`
   - 入参：`{ "token": "...", "newPassword": "...", "confirmPassword": "..." }`
   - 返回：成功后提示重新登录。

管理员接口（仅平台管理员）：

1. `POST /api/admin/users/:userId/password-reset-email`
   - 触发发送重置密码邮件。
   - 中间件：`RequirePlatformAdmin + RequireAdminOperationToken`。
   - operation 建议：`user.password_reset_email`。

### 3.6 错误码与映射

新增错误码建议：

1. `PASSWORD_RESET_TOKEN_INVALID`
2. `PASSWORD_RESET_TOKEN_EXPIRED`
3. `PASSWORD_RESET_TOKEN_CONSUMED`
4. `PASSWORD_RESET_EMAIL_NOT_ENABLED`
5. `PASSWORD_RESET_EMAIL_SEND_FAILED`
6. `PASSWORD_RESET_RATE_LIMITED`

映射方式：

1. 在 `response/error_codes.go` 注册编号。
2. 在 `response/error_auth.go` 与 `errcode/*` 增加模板与映射。
3. 保持现有 `JsonResult` 协议与 `response.FromError` 处理链路。

### 3.7 审计日志

应记录的审计行为：

1. 管理员发起重置邮件（`module=user`, `action=update`, `targetType=user_password_reset_email`）。
2. 系统配置 `email` 新增/更新/测试发送（`module=system_config`）。

说明：自助找回流程不记录管理员审计，但服务日志应保留 request_id 与去敏感字段。

---

## 4. 前端设计

### 4.1 路由与页面

新增公开页面：

1. `/forgot-password`：输入邮箱后提交重置申请。
2. `/reset-password`：输入新密码并提交。

重置令牌传递方式：

1. 邮件链接使用 `#token=...`（放在 hash，避免 token 落到服务端访问日志）。
2. 前端从 `location.hash` 解析 token。

路由守卫调整：

1. 未登录允许访问 `/login`、`/register`、`/forgot-password`、`/reset-password`。
2. 已登录访问重置页面时可跳回首页或编辑页（与现有登录态策略保持一致）。

### 4.2 登录页改造

`AuthPanel` 在登录模式展示“忘记密码”入口，跳转 `/forgot-password`。

### 4.3 后台用户管理改造

`AdminUsersPage` 每行新增按钮“发送重置邮件”：

1. 二次确认后触发 `dataGateway.admin.sendPasswordResetEmail(userId)`。
2. 接入后台 operation token 机制。
3. 成功后 toast 提示，失败展示错误原因。

### 4.4 Data Gateway 扩展

新增类型与接口：

1. `AuthGateway.requestPasswordReset(...)`
2. `AuthGateway.verifyPasswordResetToken(...)`
3. `AuthGateway.confirmPasswordReset(...)`
4. `AdminGateway.sendUserPasswordResetEmail(...)`

并在 `adapter.ts` 完成请求实现与错误处理。

---

## 5. 安全策略

1. 防枚举：`/auth/password-reset/request` 对不存在邮箱也返回成功。
2. 防重放：令牌仅可消费一次，服务端强校验。
3. 防泄漏：
   - 数据库存储 `token_secret_hash`，不存明文 token。
   - 邮件链接 token 放 hash（`#token=`）。
4. 防滥用：按邮箱/IP 限流（可先按配置软限制，后续可对接统一风控）。
5. 会话安全：重置成功后立即吊销用户全部会话。
6. 最小暴露：错误消息前台去细节、后台保留可运维信息。
7. LDAP 兼容：`password_hash == "!ldap!"` 的账号默认不走本地重置。

---

## 6. 兼容性与约束

1. 兼容现有认证模型（local/ldap/mixed）。
2. 兼容现有后台高风险操作令牌体系。
3. 兼容现有 `system_configs` 版本控制与并发冲突机制。
4. 兼容现有错误响应规范（`JsonResult` + 统一错误码）。

---

## 7. 测试计划

### 7.1 后端单元测试

1. `email` 配置校验、secret 掩码与回填。
2. token 生成/校验/过期/消费/重复消费。
3. 重置成功后密码更新与会话吊销。
4. LDAP 占位密码用户触发重置的拒绝路径。

### 7.2 后端集成测试

1. 三个认证重置接口的完整流程。
2. 管理员发起重置邮件接口权限校验与 operation token 校验。
3. 错误码与审计日志写入验证。

### 7.3 前端联调测试

1. 登录页入口可达性与跳转正确性。
2. 忘记密码 -> 收到邮件 -> 重置成功 -> 新密码登录。
3. 管理后台发起重置邮件流程与提示反馈。

---

## 8. 实施顺序

1. 新增迁移与 repository（`password_reset_tokens`）。
2. 新增 `email` 系统配置校验、掩码、读取服务。
3. 新增 `MailSender` 与 `PasswordResetService`。
4. 接入 auth/admin handler 与 router。
5. 扩展前端 data-access 与新页面。
6. 改造 `AuthPanel` 与 `AdminUsersPage` 入口。
7. 完成单测/集成测试与文档更新。

---

## 9. 待评审决策项

1. LDAP 用户是否永久禁止本地密码重置（建议：是）。
2. 管理员是否允许对自己发重置邮件（建议：否）。
3. 重置成功后是否自动登录（建议：否，强制重新登录）。
4. 首期邮件发送策略是否同步发送（建议：是；后续可演进异步队列）。

