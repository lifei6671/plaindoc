# LDAP 直连认证实施任务清单（可落地）

**Last Updated**: 2026-02-27  
**适用范围**: `apps/server` 认证链路、`apps/web` 登录页与后台系统配置  
**目标**: 在不破坏现有 JWT/Session 体系的前提下，引入可扩展的 LDAP 直连登录能力，并保留应急回退能力。

---

## 1. 背景与当前约束

当前系统登录是“本地账号 + bcrypt 密码校验”模型：

1. 登录请求固定为 `email + password`。
2. `AuthService.Login` 只查本地 `users` 表并做 `bcrypt.CompareHashAndPassword`。
3. `users.password_hash` 为 `NOT NULL`，不适合直接存放“仅 LDAP”账号。
4. 后台系统配置当前没有 `auth` 配置项，仅有 `site/editor/security/image-hosting/sitemap`。

结论：LDAP 接入应采用“认证源解耦 + 身份映射”方案，而不是在现有 `Login` 里硬编码 LDAP 分支。

---

## 2. 总体方案（冻结版）

### 2.1 关键原则

1. **会话体系不变**：LDAP 只负责身份校验，JWT 签发与 refresh 轮换继续复用现有 `AuthService`。
2. **认证源可扩展**：抽象 `IdentityProvider`，LDAP 只是第一个 provider，后续可加 OIDC/SAML。
3. **管理员主导策略**：登录方式由后台配置驱动（`ldap_only/mixed/local_only`），不是由普通用户决定。
4. **默认自动路由**：按 `defaultProvider + 匹配规则` 自动选择 provider，前端手动选择仅做兜底。
5. **安全优先**：LDAP 密钥不明文返回前端；登录失败统一错误文案；保留 break-glass 本地管理员入口。

### 2.2 模式定义

1. `local_only`：仅本地账号登录（当前行为）。
2. `ldap_only`：仅 LDAP 登录，关闭普通注册入口。
3. `mixed`：本地 + LDAP 并存，按规则自动路由，必要时允许手动指定 provider。

---

## 3. 目标架构

### 3.1 服务拆分

新增/调整服务（建议文件名）：

1. `internal/service/auth_provider.go`  
定义接口：
   - `Authenticate(ctx, input) (Principal, error)`
2. `internal/service/auth_provider_local.go`  
本地账号 provider（迁移现有 bcrypt 校验逻辑）。
3. `internal/service/auth_provider_ldap.go`  
LDAP provider（连接、search、bind、属性提取）。
4. `internal/service/auth_login_orchestrator.go`  
根据系统配置选择 provider，完成“认证 -> 本地用户映射 -> 签发会话”编排。
5. `internal/service/auth_config_service.go`  
读取并缓存 `auth` 系统配置，统一解析/校验。

### 3.2 身份映射模型

新增表：`user_identities`（一个用户可绑定多个外部身份）。

建议字段：

1. `id`（自增主键）
2. `user_id`（FK -> users.user_id）
3. `provider_type`（如 `local` / `ldap`）
4. `provider_id`（如 `corp-ldap`）
5. `external_id`（LDAP 的稳定唯一 ID，如 `entryUUID` 或 `objectGUID`）
6. `login_name`（LDAP 登录名或邮箱，便于审计）
7. `created_at` / `updated_at` / `last_login_at`

约束建议：

1. 唯一键：`(provider_id, external_id)`
2. 索引：`user_id`

说明：

1. 对“仅 LDAP 用户”，`users.password_hash` 存不可登录占位值（如 `!ldap!`），满足现有非空约束。
2. 本地账号可逐步补齐 `user_identities` 记录，不要求一次性回填。

---

## 4. 配置设计（系统配置 key: `auth`）

建议配置结构：

```json
{
  "loginMode": "ldap_only",
  "defaultProviderId": "corp-ldap",
  "allowUserRegister": false,
  "providers": [
    {
      "id": "corp-ldap",
      "name": "Corp LDAP",
      "type": "ldap",
      "enabled": true,
      "priority": 100,
      "matchRules": {
        "emailDomains": ["corp.example.com"],
        "usernameRegex": "^[a-z0-9._-]+$"
      },
      "ldap": {
        "host": "ldap.corp.example.com",
        "port": 636,
        "tlsMode": "ldaps",
        "baseDN": "dc=corp,dc=example,dc=com",
        "bindDN": "cn=readonly,ou=svc,dc=corp,dc=example,dc=com",
        "bindPasswordCiphertext": "ENC(...)",
        "userFilter": "(uid=%s)",
        "idAttribute": "entryUUID",
        "emailAttribute": "mail",
        "nameAttribute": "cn",
        "groupAttribute": "memberOf",
        "connectTimeoutMs": 3000,
        "readTimeoutMs": 3000
      }
    }
  ],
  "breakGlass": {
    "enabled": true,
    "localAdminEmails": ["platform-admin@example.com"]
  }
}
```

注意：

1. `bindPassword` 只在服务端解密使用；后台读取接口必须掩码输出，不回明文。
2. `auth` 配置应加入 `admin_system_config_service` 白名单校验与版本并发控制。

---

## 5. API 与前端契约变更

### 5.1 登录接口

`POST /api/auth/login` 请求体改造（兼容旧参数）：

1. 新字段：`identifier`（邮箱或用户名）
2. 新字段：`provider`（可选，仅 `mixed` 兜底使用）
3. 兼容字段：`email`（旧客户端继续可用）

返回体不变：继续返回 `{ user, token, refreshToken }`。

### 5.2 前端登录页

1. 默认只展示“账号 + 密码”。
2. 当后端返回“需要选择 provider”时，再展示 provider 下拉。
3. `ldap_only` 模式下隐藏注册入口；`mixed/local_only` 再按策略显示注册入口。

---

## 6. 分阶段任务清单（执行顺序）

### Phase 0：冻结方案与风险清单

- [ ] `P0-1` 明确 LDAP 字段映射（`external_id` 取值优先级、邮箱是否必填）。  
- [ ] `P0-2` 明确登录模式切换规则（尤其 `ldap_only` 的 break-glass 机制）。  
- [ ] `P0-3` 明确安全基线（TLS 必开、错误文案统一、审计字段范围）。

**验收标准**：

1. 本文档评审通过并冻结。
2. 形成可直接开发的字段/接口清单。

### Phase 1：后端骨架（不改变线上行为）

- [x] `P1-1` 增加 `IdentityProvider` 抽象与 `local provider` 实现。  
- [x] `P1-2` 新增 `AuthLoginOrchestrator`，先只接入 local provider。  
- [x] `P1-3` 将 `authHandler.Login` 调整为调用 orchestrator。  
- [x] `P1-4` 保持现有测试全部通过（行为零变化）。

**验收标准**：

1. `go test ./... -count=1` 全绿。
2. 登录行为与当前版本一致。

### Phase 2：数据模型与迁移

- [x] `P2-1` 为 mysql/postgres/sqlite 增加 `0014_user_identities` 迁移（up/down）。  
- [x] `P2-2` 新增 `models.UserIdentity` 与 repository 接口/实现。  
- [x] `P2-3` 补充迁移测试与 repository 单元测试。  
- [x] `P2-4` 增加 `!ldap!` 密码占位常量与约束说明。

**验收标准**：

1. 三种驱动迁移可正常 up/down。
2. 身份映射表 CRUD 与唯一键约束可验证。

### Phase 3：LDAP Provider MVP

- [x] `P3-1` 接入 LDAP 客户端（建议 `github.com/go-ldap/ldap/v3`）。  
- [x] `P3-2` 实现 `search + bind` 认证流程。  
- [x] `P3-3` 提取 `external_id/email/name`，落地本地映射与 JIT 建号。  
- [x] `P3-4` 输出审计日志（provider、结果、原因分类）。  
- [x] `P3-5` 增加 provider 健康探测与超时控制。

**验收标准**：

1. 连接测试通过时可成功登录并拿到现有会话 token。
2. LDAP 异常时不会泄漏内部错误细节。

**Phase 3 当前实现入口（2026-02-27）**：

1. 启用 LDAP：`AUTH_LDAP_ENABLED=true`。  
2. provider 默认路由：`AUTH_DEFAULT_PROVIDER=local|ldap`（默认 `local`）。  
3. LDAP 基础参数：`AUTH_LDAP_HOST`、`AUTH_LDAP_PORT`、`AUTH_LDAP_TLS_MODE(ldaps|starttls)`、`AUTH_LDAP_BASE_DN`。  
4. 账号检索与映射：`AUTH_LDAP_USER_FILTER`、`AUTH_LDAP_ID_ATTRIBUTE`、`AUTH_LDAP_EMAIL_ATTRIBUTE`、`AUTH_LDAP_NAME_ATTRIBUTE`。  
5. 连接与读取超时：`AUTH_LDAP_CONNECT_TIMEOUT`、`AUTH_LDAP_READ_TIMEOUT`。  
6. 服务账号（可选）：`AUTH_LDAP_BIND_DN`、`AUTH_LDAP_BIND_PASSWORD`。  
7. 登录接口兼容：`POST /api/auth/login` 支持 `identifier/provider`（旧 `email` 字段仍兼容）。

### Phase 4：系统配置与后台治理

- [x] `P4-1` 扩展系统配置白名单，新增 `auth`。  
- [x] `P4-2` 增加 `auth` 配置校验器（字段完整性、模式合法性、provider 唯一性）。  
- [x] `P4-3` 后台系统配置页增加“认证设置”面板。  
- [x] `P4-4` 增加“测试连接”接口（仅管理员可用，不落库密码明文）。

**验收标准**：

1. 管理员可在后台维护 LDAP 配置并完成连接验证。
2. 读取配置接口不会返回明文密钥。

**Phase 4 当前实现入口（2026-02-27）**：

1. 后端系统配置新增 `auth` key，校验 `loginMode/defaultProviderId/providers/breakGlass`。  
2. LDAP 密钥字段 `bindPasswordCiphertext` 仅在服务端保存，`/api/admin/system-configs` 返回掩码 `********`。  
3. LDAP 测试接口：`POST /api/admin/system-configs/auth/providers/ldap/test`（仅 `platform_admin`）。  
4. 后台页面：`/admin/system-configs` 新增“认证设置”标签页，可编辑 auth 配置并触发连接测试。

### Phase 5：前端登录体验与兼容

- [x] `P5-1` 登录表单支持 `identifier`。  
- [x] `P5-2` 在 `ldap_only` 隐藏注册入口。  
- [x] `P5-3` `mixed` 模式支持 provider 选择兜底。  
- [x] `P5-4` 保持旧客户端 `email` 字段兼容。

**验收标准**：

1. 三种登录模式都可端到端验证。
2. 不破坏当前 token 存储与刷新逻辑。

**Phase 5 当前实现入口（2026-02-27）**：

1. 登录策略公开接口：`GET /api/auth/options`，返回 `loginMode/defaultProviderId/allowUserRegister/providers`（不含 LDAP 敏感字段）。  
2. 注册策略统一：`site.allowRegistration` 与 `auth.loginMode/auth.allowUserRegister` 联合判定，`ldap_only` 强制关闭注册。  
3. 登录页输入改造：`identifier + password`，并在 `mixed/ldap_only` 下支持可选 provider 下拉兜底。  
4. 旧客户端兼容：`POST /api/auth/login` 继续兼容 `email` 字段，服务端优先读取 `identifier`，为空回退 `email`。  

### Phase 6：联调、灰度、发布

- [ ] `P6-1` 增加 LDAP 集成测试（本地可用容器或 mock server）。  
- [ ] `P6-2` 生产灰度顺序：`local_only -> mixed -> ldap_only`。  
- [ ] `P6-3` 配置监控告警（登录失败率、LDAP 超时、连接失败）。  
- [ ] `P6-4` 发布回滚预案演练。

**验收标准**：

1. 灰度期间无大面积登录失败。
2. 回滚到 `local_only` 可在分钟级完成。

---

## 7. 安全与合规清单（必须满足）

1. 只允许 `ldaps/starttls`，禁止明文 LDAP。
2. LDAP 过滤器参数必须 escape，防注入。
3. 错误响应统一，不暴露用户存在性。
4. 密钥只加密存储，读取接口仅返回掩码。
5. 审计日志不记录密码、完整 LDAP 响应体。
6. 支持 break-glass 本地管理员账号应急登录。

---

## 8. 测试矩阵（最低要求）

1. 单元测试：provider 选择、配置解析、身份映射、错误映射。
2. 集成测试：LDAP 成功登录、密码错误、用户不存在、连接超时、TLS 错误。
3. 回归测试：现有本地登录/刷新/登出全通过。
4. 权限测试：LDAP 新用户登录后对空间读写权限遵循现有规则。

---

## 9. 回滚与应急策略

1. 配置级回滚：将 `auth.loginMode` 切回 `local_only`。
2. 服务级降级：临时禁用 LDAP provider，仅保留本地 provider。
3. 数据级回滚：`user_identities` 可保留，不影响本地登录。
4. 应急账号：预置 `platform_admin` 本地账号，定期轮换密码并离线保管。

---

## 10. 下一步执行建议（从本文档开始）

推荐先执行 **Phase 1**（后端骨架、零行为变化），完成后再进入 Phase 2。

Phase 1 完成标志：

1. `authHandler` 已切到 orchestrator。
2. 本地登录行为与当前一致。
3. 全量测试通过。
