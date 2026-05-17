# 后端统一开发文档（`apps/server`）

**Last Updated**: 2026-04-05  
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
| `GORM_LOG_SQL` | `false` | `config.Load()` | `storage.OpenDatabase` | 是否输出 GORM SQL 执行日志；默认关闭，排查数据库问题时显式开启。 |
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
| `onlyoffice` | `OnlyOfficeConfigService.GetConfig` | `validateOnlyOfficeConfig` | ONLYOFFICE Docs 开关、Document Server 地址、callback 外链地址、JWT 密钥。 |
| `homepage.ssr.anonymous_cache` | `HomeService.resolveAnonymousCacheControl` | `validateHomepageAnonymousCacheConfig` | 首页匿名缓存头策略。 |
| `editor`、`security` | 对应服务按需读取 | 对应 validator | 编辑器与安全策略配置项。 |

校验实现文件：`apps/server/internal/service/admin_system_config_service.go`。

`image-hosting` 额外约束：

1. 图片链路读取 `imageUploadPathTemplate`，附件链路读取 `attachmentUploadPathTemplate`。
2. 历史字段 `uploadPathTemplate` 仅作为兼容输入；归一化时会继续喂给图片模板，并把附件模板自动迁移到 `attachments/` 前缀。
3. Office 初始化、文档附件上传、Office HTML 渲染落盘都属于附件链路，不能再复用图片模板。

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
14. `GET /api/docs/:docId/onlyoffice/edit-config`
15. `GET /api/docs/:docId/onlyoffice/source`
16. `POST /api/docs/:docId/onlyoffice/callback`

ONLYOFFICE 一等文档规则：

1. `documents.format` 区分 `markdown/docx/xlsx`，`nodes.type` 仍只表示 `folder/doc`。
2. `SaveDocument` 仅适用于 Markdown；Office 文档必须走 ONLYOFFICE callback 写回。
3. Office 正文落在 `file_blobs`，版本快照落在 `document_file_revisions`。
4. `VisibilityService` 按“文档可读/可写”工作，Office 与 Markdown 共用同一套权限判断。
5. callback 成功后会更新 `documents.version/content_version/source_blob_id`，并追加 `audit_logs` 记录。

### 6.3 管理后台（`/api/admin/*`）

中间件前置：

1. `RequireAdminSession`：仅验证登录态，用于 `/api/admin/me`、`/api/admin/profile` 与成员态空间列表。
2. `AttachAdminAuditContext`
3. `RequireAdmin` / `RequirePlatformAdmin` / `RequireSpaceManagement`：仅包住真正的高权限治理路由。

模块：

1. 管理员/后台用户信息与个人资料。
2. operation token 签发。
3. 用户治理（平台管理员）。
4. 空间治理（平台全量/空间管理员按 scope；普通登录用户仅能看到自己参与的空间列表，并且只读编辑文档入口）。
5. 分享治理（所有登录用户都能进入分享中心，普通登录用户只允许查询自己的分享记录并对自己创建的分享执行改码、延期、设永久、取消等自助操作；管理员可以查询全部并执行治理动作）。
6. 文档治理。
7. 主题治理（平台管理员）。
8. 系统配置治理（平台管理员）。
9. 审计检索（平台管理员）。

个人资料接口补充约束：

1. `/api/admin/profile` 的查询、修改昵称、头像上传、修改密码都只要求登录态。
2. 普通登录用户的个人资料自助更新会继续写入 `audit_logs`，但审计写入对 `profile` / `profile_password` 这类自助场景放行，不再强制操作者必须是管理员。
3. 空间导入由业务服务先校验 `space_create` 能力；空间导出先校验 `space_manage`，普通登录用户仅可导出自己拥有的空间或明确可管理的空间。导入导出都会写 `space.import` / `space.export` 审计；审计 metadata 只记录 job、阶段、能力类型和文件名/大小等非敏感信息，禁止写入 token、本地私有路径或敏感配置值，失败错误若包含 token、私有目录或绝对路径必须泛化。
4. 空间导出下载只能消费一次性 `downloadToken`，并且服务端返回文件前必须校验文件仍位于导出私有目录内，扩展名只能是 `.zip`、`.plaindoc` 或 `.epub`；可导入空间交换包必须使用 `.plaindoc` 后缀。
5. 普通登录用户在创建空间或导入完成补默认封面时，可以先创建 `space_cover_asset`；真正绑定到空间仍必须通过空间元数据更新校验，且只有纯封面绑定允许空间 owner 自助执行，审计 targetType 使用 `space_cover_binding`。
6. 除个人资料自助、空间导入/导出、封面资产创建和纯封面绑定场景外，后台审计仍按管理员身份收口，不能把这些例外扩散到其他管理接口。

空间导入导出接口补充约束：

1. 导出入口：`POST /api/admin/spaces/:spaceId/exports` 创建任务，`GET /api/admin/spaces/:spaceId/exports/:jobId/events?token=...` 订阅 SSE，`GET /api/admin/space-exports/:jobId/download?token=...` 消费一次性下载 token。
2. 导入入口：`POST /api/admin/space-imports/inspect` 上传并解析 `.plaindoc`，`POST /api/admin/space-imports/:importId/commit` 创建新空间导入任务，`GET /api/admin/space-imports/:jobId/events?token=...` 订阅 SSE。
3. 导出临时文件位于服务端私有 `data/exports/admin-space`，导入 staging 位于 `data/imports/admin-space`；接口不能接受客户端传入的任意本地路径。
4. SSE token 绑定 actor、空间或导入任务、job，默认短期有效；任务完成、失败或弹层关闭后前端应关闭订阅。
5. 导出任务已 completed 后才订阅 SSE 时，服务端初始 `completed` 事件要重新签发一次性下载 token，不能重放旧明文 token，也不能只返回文件名。
6. 导入/导出任务处于 `queued` 或 `running` 时不能因为 SSE stream token 过期而被清理；清理循环只处理终态任务和过期 staging，避免长任务完成事件、下载 token 或回滚状态丢失。
7. 导入落地创建文档时必须显式使用默认主题 `default`；导入失败后如果回滚新空间也失败，任务仍保留 `restore` 主阶段和原始错误，并附带回滚错误供排查。
8. 导入 Office 源文件时不能依赖内容嗅探决定 MIME；`docx` 固定写入 `application/vnd.openxmlformats-officedocument.wordprocessingml.document`，`xlsx` 固定写入 `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`，避免 ZIP 容器被误存为 `application/zip`。
9. 服务端必须兜底规范化导出选项：`source_zip` 强制包含附件和 Office 源文件，`epub` 强制包含 Office 源文件用于渲染但不导出普通附件。
10. `.plaindoc` 交换包如果包含空间封面，导入时必须校验封面文件存在、`source` 只能是空值/`user_upload`/`system_generated`，恢复封面资产并绑定新空间；没有封面时由前端导入完成后复用创建空间的浏览器默认封面生成逻辑补齐。
11. 导入封面对象写入本地 `uploads/space-covers` 后，如果封面资产持久化或新空间创建失败，必须清理已写入的封面对象；若封面资产已落库但新空间创建失败，还必须删除该封面资产记录，避免孤儿文件和孤儿元数据。
12. `POST /api/admin/space-imports/inspect` 必须在 `FormFile` 或 `ParseMultipartForm` 前用 `http.MaxBytesReader` 限制请求体，避免超大 multipart 在进入 service 体积校验前消耗临时磁盘或 IO；handler 上限要与 `service.MaxAdminSpaceImportUploadBytes` 保持一致，并预留 multipart 元数据开销。

后台壳页的能力摘要由 `/api/admin/me` 返回，前端据此区分：

1. 仅个人信息 + 我的分享 + 自助分享操作视图。
2. 个人信息 + 我的分享 + 自助分享操作 + 成员态空间管理视图。
3. 完整后台治理视图。

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

### 7.5 EPUB 导出复用 SSR 的边界

1. EPUB Markdown 章节必须通过阅读页 SSR Worker 渲染，后端提取 `#plaindoc-preview-body` 写入 EPUB。
2. SSR renderer 未注入或单次渲染失败时，EPUB 导出任务应失败；禁止使用 Go Markdown fallback 静默生成与阅读页不一致的章节。
3. EPUB 章节应尽量保持阅读页/分享页正文效果一致，但必须移除代码块复制按钮等浏览器交互控件，保留 `<pre><code>` 内容本身。
4. EPUB 目录必须按空间 `tree` 递归生成，文档节点下的子文档也要导出，并通过 `go-epub.AddSubSection` 保留上下级目录。
5. `router.go` 仅在 `readerSSRDispatcher` 可用时向 `AdminSpaceExportService` 注入 `AdminSpaceExportSSRReaderHTMLRenderer`。
6. EPUB 图片资源只允许 `data:image/*` 与 `/uploads/*` 这类可信来源；写入 EPUB 前必须先落到服务端私有临时目录，再交给 `go-epub.AddImage`，任意远程 URL 或本机路径必须降级为 alt 文本。

---

## 8. 数据模型与迁移（逐表说明）

### 8.1 Schema 真正来源

1. 迁移脚本是结构真值来源：
   - `apps/server/internal/storage/migrations/sqlite/*.sql`
   - `apps/server/internal/storage/migrations/mysql/*.sql`
   - `apps/server/internal/storage/migrations/postgres/*.sql`
2. GORM 模型用于代码映射：`apps/server/internal/storage/models/*.go`。
3. PostgreSQL 迁移执行器已支持 `DO $$...$$` 这类 dollar-quoted block，可用于幂等约束检查与复杂 DDL 编排；编写这类脚本时仍需保证三套迁移一致，并补回归测试。

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
   - 例外是后台壳页的“可进入性”：`/api/admin/me`、`/api/admin/profile` 只要求登录态，具体菜单/按钮再按能力视图收口。
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

---

## 13. 全文检索可行性方案（M0-M9 对齐，适配当前项目）

本章节给出“可落地、可切换、可回滚”的全文检索方案，目标是先交付 L1 基线能力，再平滑演进到 L2。

分词抽象与自定义词典专题见：`docs/FULLTEXT_SEARCH_ANALYZER_DESIGN.md`。

### 13.1 现状与可行性结论

基于当前代码与结构（`router.go`、`workspace/access handler`、`VisibilityService`、`system_configs`、后台审计与 operation token、`main.go` 后台循环任务模型）：

1. **可行**：项目已有单体服务、统一路由注入、配置中心、后台治理与审计机制，具备新增检索子系统的基础。
2. **主要缺口**：目前无通用搜索 API、无可插拔搜索 Provider、无索引任务队列与状态机。
3. **推荐路线**：`Bleve 先行（L1） -> 补齐任务系统与状态机 -> 对接 Meilisearch/Typesense（L2）`。

### 13.2 L1/L2 范围定义（产品边界）

| 层级 | 本项目建议能力 |
| --- | --- |
| L1（首期必须） | `q` 关键词、`space_id` 过滤、权限过滤、分页、排序（`relevance` / `updated_at_desc`）、返回 `doc_id + score`（snippet 可选） |
| L2（增强） | 高亮、typo、同义词、facets、复杂排序、多字段权重可配 |

首期明确不做：

1. 跨空间一次性聚合搜索（先做单空间检索，降低权限泄露风险）。
2. 基于向量/语义检索的复杂召回（留到后续专题）。

### 13.3 权限模型映射（检索统一口径）

```mermaid
flowchart TD
  A[请求进入搜索接口] --> B{是否有登录态 viewerUserID}
  B -- 否 --> C[actor=anonymous]
  B -- 是 --> D[actor=logged-in]

  C --> E[SearchQueryService.Search]
  D --> E

  E --> F[加载 search 配置与 analyzer]
  F --> G[分词/归一化 query]
  G --> H{是否指定 space_id}
  H -- 是 --> I[scope_space_ids = [space_id]]
  H -- 否 --> J[从仓储解析可见空间 scope_space_ids]
  J --> K{scope 为空?}
  K -- 是 --> Z[直接返回空结果]
  K -- 否 --> L[继续]

  I --> M{有 space_id 且已登录?}
  L --> M
  M -- 是 --> N[解析该空间 user_role_level]
  M -- 否 --> O[user_role_level = 0]

  N --> P[构建 SearchRequest]
  O --> P

  P --> Q{Provider}
  Q -- Bleve --> R[倒排召回候选]
  Q -- Database --> S[SQL召回候选]

  R --> T[硬过滤: scope + visibility_scope + min_role + 状态]
  S --> U[硬过滤: scope + visibility_scope + min_role + 状态]

  T --> V[分页]
  U --> V
  V --> W[返回命中]

```
当前项目内容权限核心来自：

1. `spaces.visibility` 与 `documents.visibility`（`public/authenticated/member`）。
2. 空间成员角色：`owner/collaborator/reader`（见 `space_members.role`、`models.Role`）。
3. 可见性判定由 `VisibilityService` 统一执行（后端真值来源）。

为兼容检索过滤，定义 `role_level`（内容访问角色）：

1. `reader=1`
2. `collaborator=2`（对应通用方案的 Editor）
3. `owner=3`（对应通用方案的 Admin）

可见性等级定义：

1. `public=1`
2. `authenticated=2`
3. `member=3`

文档实际可见等级：

1. `effective_visibility = max(space_visibility, document_visibility)`（取更严格一侧）。

检索权限必须满足“单调性”：

1. 登录用户结果集必须包含匿名用户结果集。
2. 即 `VisibleDocs(logged_in) ⊇ VisibleDocs(anonymous)`。

空间可见性矩阵：

| 空间可见性 | 未登录 | 已登录非成员 | 已登录成员（owner/collaborator/reader） |
| --- | --- | --- | --- |
| `public` | 是 | 是 | 是 |
| `authenticated` | 否 | 是 | 是 |
| `member` | 否 | 否 | 是 |

空间/文档组合矩阵（检索是否可见）：

1. 该矩阵对“单空间检索”和“跨空间聚合检索”都成立；跨空间场景按每条文档所属空间独立判定。

| 空间可见性 | 文档可见性 | 未登录 | 已登录非成员 | 已登录成员（该空间） |
| --- | --- | --- | --- | --- |
| `public` | `public` | 是 | 是 | 是 |
| `public` | `authenticated` | 否 | 是 | 是 |
| `public` | `member` | 否 | 否 | 是 |
| `authenticated` | `public` | 否 | 是 | 是 |
| `authenticated` | `authenticated` | 否 | 是 | 是 |
| `authenticated` | `member` | 否 | 否 | 是 |
| `member` | `public` | 否 | 否 | 是 |
| `member` | `authenticated` | 否 | 否 | 是 |
| `member` | `member` | 否 | 否 | 是 |

索引/查询映射建议：

1. 保留 `visibility_scope`（`public/authenticated/member`）与 `min_role`，分别表达“登录态门槛”和“成员角色门槛”。
2. 查询过滤始终包含“空间/文档状态有效（非 `deleted/banned`）”。
3. `visibility_scope=member` 时再校验 `role_level >= min_role`。

> 说明：现阶段 `member` 下默认 `min_role=1`；后续若启用 `node_permissions/document_permissions` 细粒度策略，再提升 `min_role` 或扩展 `acl_hash/allow_list`。

### 13.4 统一索引文档与查询模型

**IndexRecord（L1）建议字段**

1. `space_id`
2. `doc_id`
3. `node_id`
4. `title`
5. `body`（必须为 Markdown 清洗后的纯文本：去除代码块、公式、mermaid 与 Markdown 语法，不允许直接使用 `content_md` 原文）
6. `visibility_scope`（`public/authenticated/member`）
7. `min_role`（`1/2/3`，首期多数为 `1`）
8. `updated_at_unix`（排序统一字段）
9. `is_deleted`
10. `space_status` / `doc_status`（可选冗余字段，便于过滤）

**SearchRequest（L1）**

1. `space_id`
2. `actor_user_id`
3. `is_authenticated`
4. `user_role_level`（在该空间内计算）
5. `q`
6. `page` / `page_size`
7. `sort`（`relevance|updated_at_desc`）
8. `need_highlight`（L1 可忽略实现但保留协议位）

**SearchResponse（L1）**

1. `total`
2. `hits[]`：`doc_id`、`score`、`snippet?`

### 13.5 Provider 抽象（可插拔框架）

`apps/server/internal/search/provider` 抽象层，契约如下（伪接口）：

1. `Health(ctx) error`
2. `Verify(ctx, cfg) error`
3. `EnsureSchema(ctx) error`
4. `Upsert(ctx, []IndexRecord) error`
5. `Delete(ctx, []docID) error`
6. `PurgeBySpace(ctx, spaceID) error`
7. `Search(ctx, SearchRequest) (SearchResponse, error)`
8. `Capabilities() ProviderCapabilities`

`ProviderCapabilities` 建议包含：

1. `supports_highlight`
2. `supports_sort_updated_at`
3. `supports_typo`
4. `supports_facets`

后台 UI（系统配置/搜索管理）基于 capability 控制开关可见性与禁用状态。

### 13.6 Provider 注册与配置治理（复用现有 system_configs）

现有 `system_configs` + `AdminSystemConfigService` 已具备版本控制、校验、审计、operation token 保护，建议新增配置键：

1. `search`

`search` 配置建议结构：

1. `enabled`: 全局开关；仅开启时前台显示检索入口并执行检索。
2. `activeProvider`: `database|bleve|meili|typesense`
3. `fallbackPolicy`: `error|degrade_to_bleve`
4. `providers.bleve`: `{ enabled, indexDir }`
5. `providers.meili`: `{ enabled, endpoint, apiKey, indexName }`
6. `providers.typesense`: `{ enabled, endpoint, apiKey, collectionName }`
7. `switch`: `{ dualWriteEnabled, dualWriteWindowMinutes }`

说明：`database` provider 为数据库 `LIKE` 方案，仅支持简单搜索（不提供倒排索引与高级相关性排序）。

状态机建议单独落表（避免高频进度更新冲突 `system_configs.version`）：

1. `configured -> verified -> building -> ready -> active`
2. 异常分支：`failed` / `degraded`

### 13.7 内置 Bleve 方案（首选）

首期采用 **单索引 + 字段过滤**（A 方案）：

1. 索引数量可控，复杂度低，便于先上线。
2. 使用 `space_id` + `visibility_scope` + `min_role` 过滤避免跨空间泄露。
3. 通过 `updated_at_unix` 支持统一排序。

索引目录建议：

1. 默认：`/app/data/search/bleve`（容器）/ `./data/search/bleve`（本地）
2. 新增配置项：`SEARCH_BLEVE_DIR`（可选）

重建策略：

1. 全量：按空间分页扫描 `documents + nodes + spaces` 流式 upsert。
2. 增量：消费索引任务表（见 13.9）。
3. 损坏恢复：`Health/Verify` 失败时标记 `failed`，允许后台一键重建。

### 13.8 外部 Provider（Meilisearch / Typesense）统一要求

字段与过滤语义统一：

1. 必须支持：`space_id = X`
2. 必须支持可见性过滤：`visibility_scope + min_role + is_authenticated`
3. 统一排序字段：`updated_at_unix`
4. 全文字段：`title/body`

外部引擎不可用策略（可配置）：

1. `error`：直接返回明确错误（可观测、可预期）
2. `degrade_to_bleve`：当且仅当 Bleve `ready` 时降级读取

### 13.9 索引任务系统（Outbox/Job）

当前项目没有通用任务队列，建议新增专用表 `search_index_jobs`：

1. `job_id`
2. `provider`
3. `job_type`：`upsert/delete/purge_space/rebuild`
4. `dedupe_key`
5. `payload_json`
6. `status`：`pending/running/success/failed`
7. `retry_count`
8. `next_run_at`
9. `last_error`
10. `created_at/updated_at`

幂等与去重规则：

1. 同 provider + 同 doc 的连续 `upsert` 合并为最后一次。
2. `delete` 优先级高于 `upsert`（删后不回写）。
3. `purge_space` 会吞并该空间尚未执行的 doc 粒度任务。

Worker 建议复用 `main.go` 现有后台循环模式（参考 `runDataRetentionCleanupLoop`）：

1. 启动独立 `runSearchIndexLoop(...)`
2. 按 `next_run_at` 拉取任务
3. 失败指数退避
4. 记录结构化日志并更新 provider 状态

### 13.10 索引更新事件范围（与现有代码映射）

应触发索引更新的动作：

1. 文档创建/保存：`workspaceHandler.CreateNode`（doc）与 `SaveDocument`
2. 文档可见性变更：`accessHandler.UpdateDocumentVisibility`
3. 文档删除/空间删除：`workspaceHandler.DeleteNode`、`adminDocumentHandler.DeleteDocument`、`adminSpaceHandler.DeleteSpace`
4. 空间可见性变更：`accessHandler.UpdateSpaceVisibility`（建议触发 `purge_space + rebuild_space`）
5. 文档/空间状态治理（封禁/删除/恢复）：后台对应 handler/service

明确不触发：

1. 用户角色变更本身不触发重建；查询时按实时角色计算过滤参数即可生效。

### 13.11 读写路径与切换状态机

切换必须遵循：

1. 配置（configured）
2. 校验（verified）
3. 全量构建（building）
4. 就绪（ready）
5. 切 active（active）

禁止“改配置立即切流量”。

切换期间策略：

1. 读路径：仅 active provider 对外；候选 provider 仅内部验证。
2. 写路径：building/ready 阶段建议双写（active + target），窗口可配置。
3. 回滚：一键切回 previous active，并保留失败原因审计。

### 13.12 API 与后台操作建议

业务检索 API（L1）：

1. `GET /api/spaces/:spaceId/search?q=&page=&pageSize=&sort=`

后台治理 API：

1. `GET /api/admin/search/status`
2. `POST /api/admin/search/providers/:provider/verify`
3. `POST /api/admin/search/providers/:provider/rebuild`
4. `POST /api/admin/search/providers/:provider/activate`
5. `POST /api/admin/search/providers/:provider/rollback`

高风险操作（activate/rollback）建议叠加 `RequireAdminOperationToken`，并记录 `AdminAuditModuleSystemConfig` 审计明细。

### 13.13 安全验收清单（必须通过）

1. Reader（1）不能检索到 `min_role > 1` 的成员内容。
2. Collaborator（2）不能检索到 `min_role = 3` 内容。
3. Owner（3）可见该空间全部可检索文档。
4. 任意查询必须强制携带 `space_id`，不得跨空间返回。
5. 文档权限变更后，短暂延迟可接受，但不得出现越权可读字段泄露。
6. 外部引擎返回结果后仍做后端兜底校验（防索引漂移），但**主过滤必须在引擎侧完成**。

### 13.14 可观测性与运维（按现阶段能力）

当前仓库暂无统一 metrics 系统，首期建议：

1. 结构化日志：搜索耗时、错误、provider、降级状态、任务重试。
2. 管理接口：返回 active/ready provider、构建进度、队列堆积。
3. 告警前置指标（可先日志聚合）：`p95/p99`、错误率、队列积压、失败重试次数。

后续再接入统一 metrics（如 Prometheus）时复用同一指标语义。

### 13.15 分阶段交付（建议）

1. **Phase 1（L1 + Bleve）**  
   完成模型/Provider 抽象、Bleve 实现、`/api/spaces/:spaceId/search`、基础权限过滤、分页排序、最小测试集。
2. **Phase 2（任务系统 + 状态机）**  
   完成 `search_index_jobs`、后台 worker、全量重建、ready/active 切换、回滚与审计。
3. **Phase 3（外部 Provider）**  
   接入 Meili/Typesense、能力探测、降级策略、后台联动操作。
4. **Phase 4（L2 增强）**  
   高亮、typo、同义词、facets、复杂排序与性能优化。

> 结论：按上述路径，全文检索可在不破坏当前分层与权限体系的前提下逐步上线，且支持可控切换与回滚。
