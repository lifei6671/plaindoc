# 后端统一开发文档（`apps/server`）

**Last Updated**: 2026-02-28  
**适用对象**: 后端工程师、全栈工程师、AI Agent  
**目标**: 用一份文档覆盖后端架构、配置、接口、SSR、数据模型、发布与排障，作为唯一后端事实口径。

---

## 1. 当前状态（结论先看）

截至 2026-02-28，后端主链路已可用：

1. 单体入口（API + 页面 SSR + SPA 托管 + SSR Worker 管理）。
2. 认证体系（local + LDAP provider）与 JWT/refresh 会话轮换。
3. 工作区协作（空间/节点/文档/修订/可见性/主题）。
4. 管理后台（用户、空间、文档、主题、系统配置、审计、operation token）。
5. 双 SSR：
   - 首页/分类页：Go 模板 SSR
   - 阅读页：Go 调 Node Worker SSR

仍在持续收口：

1. 阅读页 SSR 自动化一致性（M6）与性能/发布加固（M7）。
2. LDAP 改造 Phase 6（灰度、告警、回滚演练）。

---

## 2. 技术栈与分层约束

核心栈：

1. Go 1.26
2. Gin 1.11
3. GORM 1.31
4. SQLite / MySQL / PostgreSQL
5. JWT v5 + bcrypt
6. `gg` + `x/image` + `webp`（封面处理）
7. 自研 SSR 子进程池（Go 管理 Node Worker）

分层约束（必须遵守）：

1. `handler`：参数校验、HTTP 映射、错误协议。
2. `service`：业务规则、权限判断、审计编排。
3. `repository`：数据访问与事务边界。
4. `router.go`：依赖注入与路由注册唯一入口。

统一返回协议：`JsonResult(code/message/requestId/data)`。

### 2.1 强制代码规范（后端）

1. 分层边界必须清晰：
   - `handler` 只做参数校验、响应映射、错误模板选择
   - `service` 承载业务规则、权限判定、审计编排
   - `repository` 承载 SQL/GORM 细节与事务编排
2. 路由注册与依赖注入只能在 `apps/server/internal/server/router.go` 集中维护；禁止新增并行入口。
3. 业务代码禁止直接读取环境变量；统一通过 `config.Load()` 注入后的配置对象传递。
4. 错误语义必须落到 `JsonResult.code`，禁止在业务层散落“魔法字符串/魔法数字”错误码。
5. 错误映射必须统一走 `response`：
   - `service` 返回 `MappedError/AppError` 时，`handler` 只调用 `response.FromError(c, err)`。
   - 遗留 sentinel 错误需要在 `handler` 定义 `[]response.ErrorTemplateMapping`，并调用 `response.WriteMappedError(c, err, mappings...)`。
   - 禁止在 `handler` 散落重复 `switch errors.Is` 映射块；统一保留 `response.InternalError(c)` 兜底。
6. 高风险后台写操作（封禁/删除/角色变更/系统配置变更等）必须叠加 `RequireAdminOperationToken`。
7. 后台请求链路必须保留 `AttachAdminAuditContext`，并在审计记录中带 `request_id`。
8. 权限模型必须保持：
   - 管理端：`platform_admin` 与 `space_admin` 边界不可穿透
   - 协作端：`owner > collaborator > reader`
9. 认证链路必须保持：
   - 密码仅允许 `bcrypt` 哈希存储
   - refresh token 必须走服务端会话状态并支持旋转后旧 token 失效
10. Go 代码提交前必须 `gofmt`；复杂流程（鉴权、并发、回滚、兼容分支）需补充简洁函数和行内中文注释。
11. 错误日志强制规范（MUST）：
   - 只要进入 `error` 分支（`if err != nil`），必须在返回前将该错误写入请求上下文日志容器。
   - 统一 key：`errmsg`。
   - 统一方式：`logit.SetRequestAttrs(c.Request.Context(), logit.Error("errmsg", err))`。
   - 需要补充语义时，必须使用 wrap（示例：`fmt.Errorf("查询附件失败: %w", err)`）后再写入 `errmsg`。
   - 禁止仅写 `fmt.Sprintf(...%v, err)` 字符串而不保留原始 error 链。

### 2.2 强制代码规范（数据与迁移）

1. 业务对外 ID 统一使用语义化字段名（如 `UserID/SpaceID/DocumentID`），并保持 GORM `column` 标签与实际列名一致（`*_id`）。
2. SQLite 场景禁止无差别 `SELECT *` 扫描复杂时间字段模型；优先显式 `Select` 所需列并做归一化。
3. 多表写入（例如状态变更 + 审计 + 会话吊销）必须放在同一事务内，保证原子性。
4. 结构变更必须同步补齐三套迁移脚本（sqlite/mysql/postgres），并补迁移回归测试。
5. 涉及唯一约束、外键、排序语义的变更必须在迁移层显式表达，不能只依赖应用层校验。

---

## 3. 目录与入口速查

优先阅读文件：

1. `apps/server/cmd/server/main.go`
2. `apps/server/internal/config/config.go`
3. `apps/server/internal/server/router.go`
4. `apps/server/internal/server/web_spa.go`
5. `apps/server/internal/service/*`
6. `apps/server/internal/storage/*`
7. `apps/server/internal/ssr/*`

目录职责：

1. `cmd/server`：启动、日志、DB、SSR Worker 生命周期。
2. `internal/config`：环境变量解析与校验。
3. `internal/server`：router/handler/middleware/响应协议/模板。
4. `internal/service`：业务编排与策略。
5. `internal/storage`：迁移、模型、仓储。
6. `internal/ssr`：协议、进程、进程池、分发。

---

## 4. 运行时链路（从启动到请求）

### 4.1 启动链路

入口：`apps/server/cmd/server/main.go`

1. `loadDotEnvCandidates()` 尝试加载 `.env` 候选路径。
2. 调用 `config.Load()` 解析与校验配置。
3. 初始化日志 writer（stdout/file/both）。
4. 若 `SSR_WORKER_ENABLED=true`，先 `validateSSRWorkerRuntime()` 再启动 WorkerPool。
5. 初始化数据库与迁移。
6. 构建 router 并启动 HTTP Server。

### 4.2 `.env` 与系统环境变量优先级

实现位置：`main.go` 的 `loadDotEnvCandidates()` + `loadDotEnvFile()`

加载候选顺序：

1. `.env`
2. `apps/server/.env`
3. `cmd/server/.env`
4. `apps/server/cmd/server/.env`

优先级规则：

1. 系统环境变量优先（已存在键不会被 `.env` 覆盖）。
2. `.env` 仅用于补缺。

### 4.3 请求链路

1. Gin 中间件（request_id、timeout、recovery、access_log、cors）。
2. handler -> service -> repository。
3. 统一 `JsonResult` 输出。

---

## 5. 配置系统（来源、默认值、消费点）

### 5.1 配置定义与校验来源

1. 配置定义与默认值：`apps/server/internal/config/config.go` 的 `Load()`。
2. 配置合法性校验：`config.go` 的 `Validate()`。
3. 环境变量示例：`apps/server/.env.example`。
4. 启动实际生效摘要：`main.go` 的 `server starting` 日志。

### 5.2 环境变量配置矩阵（含消费位置）

| 配置项 | 默认值 | 解析位置 | 主要消费位置 | 说明 |
| --- | --- | --- | --- | --- |
| `APP_ENV` | `development` | `config.Load()` | `main.go` | 生产环境触发更严格校验（如 JWT 密钥）。 |
| `APP_ADDR` | `:8080` | `config.Load()` | `http.Server.Addr` | 服务监听地址。 |
| `WEB_ORIGIN` | `http://localhost:3001` | `config.Load()` | `router`/handler | CORS 与页面链接构造。 |
| `WEB_DIST_DIR` | `apps/web/dist` | `config.Load()` | `web_spa.go` | SPA 托管目录。 |
| `DB_DRIVER` | `sqlite` | `config.Load()` | `storage.OpenDatabase` | 支持 sqlite/postgres/mysql。 |
| `DB_DSN` | sqlite 本地 DSN | `config.Load()` | `storage.OpenDatabase` | 数据源连接串。 |
| `DB_AUTO_MIGRATE` | `true` | `config.Load()` | `main.go` | 启动时是否执行迁移。 |
| `JWT_SECRET` | `plaindoc-dev-secret` | `config.Load()` | AuthService/JWT | 生产必须替换。 |
| `JWT_ACCESS_TOKEN_TTL` | `15m` | `config.Load()` | `authHandler`/AuthService | access token TTL。 |
| `JWT_REFRESH_TOKEN_TTL` | `168h` | `config.Load()` | `authHandler`/AuthService | refresh token TTL。 |
| `AUTH_DEFAULT_PROVIDER` | `local` | `config.Load()` | `AuthLoginOrchestrator` | local/ldap 入口路由默认 provider。 |
| `AUTH_LDAP_*` | 见 `config.go` | `config.Load()` | LDAP provider 构建 | LDAP 主机、TLS、DN、过滤器、超时等。 |
| `SSR_WORKER_ENABLED` | `false` | `config.Load()` | `main.go` | 是否启用阅读 SSR 子进程链路。 |
| `SSR_WORKER_EXEC` | `node` | `config.Load()` | `validateSSRWorkerRuntime`/worker process | Node 可执行命令。 |
| `SSR_WORKER_ENTRY` | 空字符串 | `config.Load()` | `validateSSRWorkerRuntime`/worker process | Worker 入口 JS 文件路径。 |
| `SSR_WORKER_COUNT` | `2` | `config.Load()` | `pool.New().Start()` | 常驻 worker 数量。 |
| `SSR_RENDER_TIMEOUT` | `1500ms` | `config.Load()` | `worker.Process.Render()` | 单次渲染超时。 |
| `SSR_WORKER_START_TIMEOUT` | `5s` | `config.Load()` | `pool.Start` context | 启动与握手超时。 |
| `SSR_WORKER_MAX_PAYLOAD_BYTES` | `1048576` | `config.Load()` | `stdioCodec`/`Render()` | 单次 payload 限制。 |
| `SSR_PROTOCOL_VERSION` | `v1` | `config.Load()` | handshake/request version | Go 与 Worker 协议版本。 |

### 5.3 数据库系统配置（`system_configs`）键与消费点

这些配置不是环境变量，而是后台可在线编辑配置（`/api/admin/system-configs`）：

| 配置键 | 读取服务 | 校验服务 | 作用 |
| --- | --- | --- | --- |
| `auth` | `AuthRegistrationPolicyService`、`AuthRiskPolicyService`、LDAP 相关服务 | `AdminSystemConfigService.validateAuthConfig`（含 `validateAuthRiskControlConfig`） | 登录模式、provider 列表、break-glass、认证风控（验证码/封禁）策略。 |
| `site` | `AuthRegistrationPolicyService` | `validateSiteConfig` | 站点级开关（如注册策略叠加）。 |
| `image-hosting` | `ImageHostingService.GetConfig` | `validateImageHostingConfig` | 图床 provider 与参数。 |
| `sitemap` | `SitemapService.GetConfig` | `validateSitemapConfig` | sitemap 生成策略。 |
| `homepage.ssr.anonymous_cache` | `HomeService.resolveAnonymousCacheControl` | `validateHomepageAnonymousCacheConfig` | 首页匿名缓存头策略。 |
| `editor`、`security` | 对应服务按需读取 | 对应 validator | 编辑器与安全策略配置项。 |

校验实现文件：`apps/server/internal/service/admin_system_config_service.go`。

---

## 6. API 功能域总览

### 6.1 认证与会话

1. `POST /api/auth/register`
2. `POST /api/auth/login`
3. `POST /api/auth/refresh`
4. `GET /api/auth/me`
5. `POST /api/auth/logout`
6. `GET /api/auth/options`

说明：

1. 登录支持 `identifier + provider`，兼容旧 `email`。
2. refresh token 使用服务端会话状态，支持旋转后旧 token 失效。

### 6.1.1 认证风控（验证码 + 临时封禁）

入口与核心文件：

1. HTTP 接入：`apps/server/internal/server/handler/auth.go`
   - 预检：`checkAuthRisk(...)`
   - 结果回写：`recordAuthRisk(...)`
   - 错误映射：`authRegisterErrorMappings` / `authLoginErrorMappings` / `authRefreshErrorMappings` / `authMeErrorMappings` + `response.WriteMappedError(...)`
2. 策略解析：`apps/server/internal/service/auth_risk_policy_service.go`
3. 状态机执行：`apps/server/internal/service/auth_risk_control_service.go`
4. 配置校验：`apps/server/internal/service/admin_system_config_service.go`（`validateAuthRiskControlConfig`）

请求/响应协议扩展：

1. 登录/注册请求体新增可选字段：`captchaId`、`captchaAnswer`。
2. 新增错误码：
   - `1008` `CAPTCHA_REQUIRED`
   - `1009` `CAPTCHA_INVALID`
   - `1010` `AUTH_TEMPORARILY_LOCKED`
3. 错误 `data` 字段：
   - 验证码会话：`captchaId`、`captchaImageDataUrl`、`level`、`expiresInSeconds`
   - 其中 `level` 为验证码字符数量（位数），例如 `4/5/6`
   - 封禁反馈：`lockedUntil`、`retryAfterSeconds`

风险主体维度：

1. 登录：`IP`、`identifier`、`IP+identifier` 并行计数，取最高风险级别。
2. 注册：`email`、`IP+email` 计数，避免同出口 IP 的不同新用户相互污染。
3. 所有主体键写库前会做 HMAC-SHA256，不落明文。

默认策略（可被系统配置覆盖）：

1. 登录阈值：`3/6/9` 触发 L1/L2/L3 验证码，`12` 触发 24h 封禁。
2. 注册阈值：`2/5/8` 触发 L1/L2/L3 验证码，`10` 触发 24h 封禁。
3. 统计窗口：`15m`；验证码 TTL：`120s`；封禁时长：`24h`。

验证码实现：

1. 使用 `github.com/mojocn/base64Captcha` 的 `DriverDigit` 生成图片验证码。
2. 落库字段 `auth_captcha_challenges.level` 记录验证码字符数量，不记录风险等级编号。

`system_configs.auth.riskControl` 配置来源：

1. 写入入口：`PUT /api/admin/system-configs/auth`。
2. 读取消费：`AuthRiskPolicyService.Resolve(...)`。
3. 支持字段：
   - `enabled`
   - `windowSeconds`
   - `lockSeconds`
   - `captcha.ttlSeconds`
   - `loginThresholds.{l1,l2,l3,lock}`
   - `registerThresholds.{l1,l2,l3,lock}`

关联核心表（本功能新增）：

1. `auth_risk_states`
   - 主用途：按 `scene + subject_type + subject_hash` 维护风险窗口状态。
   - 关键列：`attempt_count`、`failed_count`、`captcha_fail_count`、`lock_until`、`window_started_at`。
2. `auth_captcha_challenges`
   - 主用途：管理验证码会话生命周期与一次性消费。
   - 关键列：`captcha_id`、`scene`、`subject_hash`、`level`、`answer_hash`、`answer_salt`、`expires_at`、`consumed_at`、`issued_ip_hash`。

### 6.2 工作区与文档协作

1. `GET/POST /api/spaces`
2. `GET /api/spaces/:spaceId`
3. `GET /api/spaces/:spaceId/tree`
4. `POST /api/spaces/:spaceId/nodes`
5. `PATCH /api/nodes/:nodeId`
6. `POST /api/nodes/:nodeId/move`
7. `DELETE /api/nodes/:nodeId`
8. `GET/PUT /api/docs/:docId`
9. `GET /api/docs/:docId/revisions`
10. `PUT /api/spaces/:spaceId/visibility`
11. `PUT /api/docs/:docId/visibility`
12. `PUT /api/docs/:docId/theme`
13. `POST /api/docs/:docId/remote-images/localize`

### 6.3 管理后台（`/api/admin/*`）

中间件前置：

1. `RequireAdmin`
2. `AttachAdminAuditContext`

模块：

1. 管理员信息与个人资料。
2. operation token 签发。
3. 用户治理（平台管理员）。
4. 空间治理（平台全量/空间管理员按 scope）。
5. 文档治理。
6. 主题治理（平台管理员）。
7. 系统配置治理（平台管理员）。
8. 审计检索（平台管理员）。

### 6.4 页面与静态资源

1. `GET /`、`GET /explore/:categoryId`（Go Template SSR）
2. `GET /r/:spaceId`、`GET /r/:spaceId/:docId`（阅读 SSR）
3. `GET /uploads/*path`
4. `/login`、`/register`、`/editor/*`、`/admin/*`（SPA 托管）

---

## 7. 阅读页 SSR（Go 子进程）深度说明

### 7.1 运行模型

1. `reader_page_handler` 聚合 payload 并鉴权。
2. 调用 `Dispatcher.Render` 分发到 WorkerPool。
3. Worker 通过 JSONL 协议返回 `RenderResponse`。
4. 渲染失败时走降级页，避免全链路 500。

关键代码：

1. 协议定义：`apps/server/internal/ssr/protocol/messages.go`
2. 单进程管理：`apps/server/internal/ssr/worker/process.go`
3. 进程池：`apps/server/internal/ssr/pool/pool.go`
4. 分发器：`apps/server/internal/ssr/pool/dispatcher.go`
5. 页面调用与降级：`apps/server/internal/server/handler/reader_page.go`

### 7.2 SSR 核心配置逐项含义

| 配置项 | 含义 | 生效代码 | 失败行为 |
| --- | --- | --- | --- |
| `SSR_WORKER_ENABLED` | 开关；关闭时不启动 worker 池 | `main.go` | 阅读页走无 worker 分支（降级页面）。 |
| `SSR_WORKER_EXEC` | Node 可执行命令 | `validateSSRWorkerRuntime` + `exec.Command` | 启动前失败并退出。 |
| `SSR_WORKER_ENTRY` | worker 入口 JS 文件 | `validateSSRWorkerRuntime` + `exec.Command` | 文件不存在/是目录会启动失败。 |
| `SSR_WORKER_COUNT` | 常驻子进程数 | `pool.Start` | 非法值在 `Validate()` 即拒绝。 |
| `SSR_RENDER_TIMEOUT` | 单次渲染超时（也写入 `deadlineMs`） | `worker.Process.Render` | 超时杀进程并返回错误，页面降级。 |
| `SSR_WORKER_START_TIMEOUT` | worker 启动+握手超时 | `context.WithTimeout` in `main.go` | 启动失败并退出。 |
| `SSR_WORKER_MAX_PAYLOAD_BYTES` | payload 上限 | `stdioCodec` + `Render()` | 超限直接报错，不发请求。 |
| `SSR_PROTOCOL_VERSION` | 握手与请求版本号 | `performHandshakeLocked` + request version | 版本不匹配，worker 握手失败。 |

### 7.3 路径与产物要求

1. Worker 构建产物默认在 `apps/web/dist-ssr/worker-entry.js`。
2. `SSR_WORKER_ENTRY` 支持相对路径；相对路径相对于服务启动工作目录解析。
3. 推荐在开发脚本中使用绝对路径（`Makefile` 的 `server-dev-ssr` 已这样处理）。

### 7.4 缓存与可用性

1. 阅读渲染缓存实现：`apps/server/internal/pkg/rendercache/rendercache.go`。
2. 失败策略：`pool.Render` 对“worker 不可用错误”先尝试一次重启再重试。
3. `reader_page_handler` 里有 fallback HTML，避免读页面完全不可用。

---

## 8. 数据模型与迁移（逐表说明）

### 8.1 Schema 真正来源

1. 迁移脚本是结构真值来源：
   - `apps/server/internal/storage/migrations/sqlite/*.sql`
   - `apps/server/internal/storage/migrations/mysql/*.sql`
   - `apps/server/internal/storage/migrations/postgres/*.sql`
2. GORM 模型用于代码映射：`apps/server/internal/storage/models/*.go`。

当前迁移版本到 `0014_user_identities`。

### 8.2 业务主链表

| 表名 | 用途 | 关键字段 | 主要读写链路 |
| --- | --- | --- | --- |
| `users` | 用户主体与状态 | `user_id`、`email`、`password_hash`、`status` | AuthService、AdminUserService |
| `user_sessions` | refresh 会话状态与轮换 | `session_id`、`refresh_token_hash`、`expires_at`、`revoked_at` | AuthService（login/refresh/logout） |
| `user_identities` | 外部身份映射（LDAP 等） | `provider_id`、`external_id`、`login_name` | LDAP provider、Auth 映射逻辑 |
| `spaces` | 空间元数据与治理状态 | `space_id`、`owner_user_id`、`visibility`、`status`、`category_id` | Workspace、AdminSpace、Home/Reader |
| `space_members` | 空间成员与角色 | `space_id`、`user_id`、`role` | Workspace、AdminSpace |
| `space_categories` | 空间分类实体 | `category_id`、`name`、`is_default` | AdminSpace、HomeService |
| `space_cover_assets` | 空间封面资产元信息 | `asset_id`、`object_key/url`、`source`、`normalized` | AdminSpace 封面上传/生成 |
| `nodes` | 目录树节点（目录/文档） | `node_id`、`space_id`、`parent_node_id`、`type`、`sort` | Workspace（树、拖拽、删除） |
| `documents` | 文档正文与状态 | `document_id`、`node_id`、`content_md`、`version`、`visibility`、`status` | Workspace、Reader、AdminDocument |
| `document_revisions` | 文档修订历史 | `document_revision_id`、`document_id`、`version`、`base_version` | Workspace 保存历史 |
| `node_permissions` | 节点级授权（ACL） | `node_id`、`user_id`、`role` | 权限求值与治理链路 |
| `document_permissions` | 文档级授权（覆盖/补充） | `document_id`、`user_id`、`role` | 权限求值与治理链路 |
| `themes` | 文档主题库 | `theme_id`、`is_builtin`、`is_enabled` | Theme API、AdminTheme |

补充：默认分类常量在 `models/space_category.go`：

1. `DefaultSpaceCategoryID = 01jmf4v2x7m7f1m6qv5kh0t2mn`
2. `DefaultSpaceCategoryName = 未分类`

### 8.3 后台治理与系统表

| 表名 | 用途 | 关键字段 | 主要读写链路 |
| --- | --- | --- | --- |
| `user_admin_roles` | 管理员角色绑定 | `user_id`、`role` | AdminAccessService |
| `space_admin_scopes` | 空间管理员管理范围 | `user_id`、`space_id` | AdminAccessService |
| `system_configs` | 系统配置中心（JSON + version） | `config_key`、`config_value_json`、`version` | AdminSystemConfigService、各配置消费者 |
| `audit_logs` | 后台审计轨迹 | `actor_user_id`、`action`、`target_type/id`、`request_id` | AdminAuditService |

### 8.4 状态与枚举（模型常量）

定义文件：`apps/server/internal/storage/models/types.go`

1. `Role`: `owner/collaborator/reader`
2. `NodeType`: `folder/doc`
3. `Visibility`: `public/authenticated/member`
4. `AdminRole`: `platform_admin/space_admin`
5. `EntityStatus`: `active/banned/deleted`

---

## 9. 本地开发、测试、构建、打包

优先使用根目录 `Makefile`：

1. `make server-dev`
2. `make server-dev-ssr`
3. `make test-server`
4. `make server-build`
5. `make build`
6. `make package`

最小回归：

1. `cd apps/server && go test ./... -count=1`
2. `npm run web:build`

高风险改动追加回归（按需执行）：

1. 后台权限/operation token/审计链路：`cd apps/server && go test ./internal/server -run TestRouter_Admin -count=1`
2. 迁移与数据库兼容：`cd apps/server && go test ./internal/storage/... -count=1`
3. 阅读页 SSR 改动：手工验证 `/r/:spaceId/:docId` 正常渲染、无权限分流、SSR 失败降级链路

发布产物：

1. `release/plaindoc-server-linux-amd64`
2. `release/plaindoc-server-linux-amd64-<version>.tar.gz`
3. `release/plaindoc-web-<version>.tar.gz`
4. `release/checksums-<version>.txt`

---

## 10. 安全与权限基线

1. 权限必须后端强校验，前端只做可见性优化。
2. `platform_admin` 与 `space_admin` 边界不可互相穿透。
3. 封禁/删除/配置变更等高风险写操作必须走 operation token。
4. 审计必须覆盖关键后台写操作。
5. 生产必须替换 `JWT_SECRET` 与关键环境变量。
6. LDAP 优先使用 `ldaps/starttls`；`plain` 仅建议在受控内网环境下使用。

---

## 11. 高频坑与排障

1. `go.mod` 不在仓库根：应在 `apps/server` 下执行 `go run ./cmd/server`。
2. SQLite 时间扫描异常：避免 `SELECT *` 直接扫 `time.Time`。
3. JWT TTL 单位误用：`time.Duration(1)` 是 1ns。
4. `.env` 未生效：检查启动 cwd 与“系统环境优先”规则。
5. SSR worker 启动失败：先查 `SSR_WORKER_EXEC`、`SSR_WORKER_ENTRY`、`SSR_PROTOCOL_VERSION`。
6. `webp` 构建错误：后端构建需 `CGO_ENABLED=1`。
7. operation token 校验失败：核对 actor/operation/target 是否一致，token 是否已消费。

---

## 12. 待继续推进

1. 阅读页 SSR M6/M7（一致性自动化、性能与发布加固）未完全收口。
2. LDAP Phase 6（灰度、监控、回滚演练）待完成。
3. 后续文档维护只更新本文件，不再新增并行“后端主文档”。
