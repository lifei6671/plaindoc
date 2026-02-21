# 后端与主题迁移进展记录（2026-02-20）

> 更新时间：2026-02-21（内容主体为 2026-02-20 当天实现）  
> 目的：沉淀当天已落地能力与踩坑结论，降低后续迭代回归风险。
> 口径说明：本文件聚焦后端主链（基础设施/数据库/主题/认证）；管理后台后续进度以 `docs/ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md` 为准。

## 今日已实现功能

### 1) 工程基础与日志链路（Milestone 1）

- 扩展后端配置项，补齐 `.env.example` 详细注释（环境、日志、超时、数据库、JWT）。
- 建立统一错误响应结构（`code` + `message`）。
- 中间件链路完善：`request_id`、`timeout`、`recovery`、`access_log`、`cors`。
- 请求入口初始化 `context` 日志容器，容器存储 `slog.Attr`，同名 key 覆盖。
- 在请求结束时统一合并并输出日志（控制台或文件）。
- `observability` 包重命名为 `logit`，并补充常用构造器：
  - `logit.String`
  - `logit.Int`
  - `logit.Int64`
  - `logit.Uint64`
  - `logit.Float64`
  - `logit.Bool`
  - `logit.Duration`
  - `logit.Time`
  - `logit.Any[T]`（泛型）

### 2) 数据库接入与迁移基线（Milestone 2）

- Go 版本升级到 `1.26.0`。
- ORM 落地为 `GORM`（`sqlite/postgres/mysql` 三驱动）。
- 完成数据库连接与迁移框架（up/down）以及回归测试。
- 完成 `0001_init` 基线表结构与索引（`users/spaces/space_members/nodes/documents/document_revisions/node_permissions/document_permissions`）。
- 模型拆分为按表单文件维护（`entities.go` 已拆分）。
- 业务 ID 字段命名统一到实体语义（如 `UserID/SpaceID/DocumentID`），并修正 GORM column 标签与数据库字段一致（`user_id/space_id/...`）。

### 3) 主题能力（后端 + 前端统一维护链路）

- 数据库新增 `themes` 表，`documents` 新增 `theme_id` 外键引用。
- 后端新增主题 API：
  - `GET /api/themes`：获取主题列表
  - `PUT /api/docs/:docId/theme`：更新文档主题
- 前端数据访问层补齐主题网关：
  - `theme.listThemes()`
  - `document.setDocumentTheme(docId, themeId)`
- 新增前端内置主题源 `apps/web/src/theme-presets.ts`，主题菜单与预览逻辑改为数据驱动。
- 本地 IndexedDB 引入 `themes` 表，文档记录增加 `themeId`。

### 4) 原 TS 主题迁移结果

- 已迁移主题总数：`7`
- 主题 ID：
  - `default`
  - `newspaper`
  - `clean-tech`
  - `wechat-minimal`
  - `green-fresh`
  - `lanqing`
  - `orange-heart`
- 新增三库主题种子迁移：`0002_seed_builtin_themes`（SQLite/PostgreSQL/MySQL）。
- 主题种子已改为 upsert 策略，确保历史库中已有 `default` 也能被升级为完整数据。

### 5) 登录与认证（Milestone 3）

- 后端新增认证接口并挂载路由：
  - `POST /api/auth/register`
  - `POST /api/auth/login`
  - `POST /api/auth/refresh`
  - `GET /api/auth/me`
  - `POST /api/auth/logout`
- 密码策略：使用 `bcrypt` 哈希存储，禁止明文落库。
- 新增三库会话迁移：`0003_auth_sessions`，用于 refresh token 校验与会话状态维护。
- refresh token 能力升级为“旋转 + 旧 token 失效”：
  - refresh 成功后写入新 session
  - 原 session 置 `revoked_at`，并记录 `replaced_by_session_id`
- logout 能力升级：
  - 携带 access token 退出时，服务端会吊销当前 session（无 token 时幂等返回 `204`）。
- 前端新增登录/注册 UI，并在 HTTP 适配器中实现：
  - access/refresh token 本地存储
  - `401` 自动 refresh + 重试一次
  - `getSession()` 基于 `/auth/me` 恢复会话
  - `logout()` 调用后强制清理本地 token

### 6) 认证链路分层重构（可维护性修正）

- 认证实现已改为标准分层：`handler -> service -> repository`。
- `handler` 仅负责参数校验、HTTP 状态码与错误码映射。
- `service` 承担认证业务编排（注册、登录、refresh 旋转、me、logout）。
- `repository` 负责 GORM 数据访问细节（`users` / `user_sessions`）。
- 路由层统一装配依赖：`NewAuthService(NewGormUserRepository, NewGormUserSessionRepository, JWTConfig)`。

## 今日踩坑记录（问题 / 根因 / 处理）

### 坑 1：ID 字段改名后，GORM 标签未同步导致映射错位

- 问题：结构体字段改为 `UserID/SpaceID` 后，部分标签仍是旧列名（例如 `ulid`）。
- 根因：批量重命名时仅改了 Go 字段名，漏改 `gorm:"column:..."`。
- 处理：逐表核对模型与迁移列名，统一修正为 `*_id`。

### 坑 2：SQLite 下扫描时间字段报错（`TEXT` -> `time.Time`）

- 问题：主题接口查询时，扫描结果报时间类型不匹配。
- 根因：SQLite 迁移将时间列存为 `TEXT`，直接映射 `time.Time` 存在兼容问题。
- 处理：主题查询与更新响应改为显式行结构（避免依赖 `time.Time` 扫描）；更新时间字段在响应层按 RFC3339 输出。

### 坑 3：`0002` 主题种子最初用“冲突忽略”，导致旧 `default` 未升级

- 问题：历史库已经有 `default`（来自 `0001`），`0002` 使用 ignore 时不会覆盖，导致主题字段不完整。
- 根因：迁移策略只做“补缺”，未做“升级”。
- 处理：三库 `0002` 全部改成 upsert update（冲突时更新主题内容与 `updated_at`）。

### 坑 4：本地 IndexedDB 仅插入缺失主题，旧主题不会被覆盖

- 问题：本地已存在旧版 `default` 时，升级后仍保留旧数据。
- 根因：`ensureBuiltinThemes` 只做 exists-check + add。
- 处理：改为 `syncBuiltinThemes`：存在则更新、缺失则新增，并保留 `createdAt`。

### 坑 5：迁移回滚测试步数固定为 1，新增版本后无法完全回滚

- 问题：新增 `0002` 后，测试仍按 1 步回滚，残留表结构。
- 根因：回滚测试与迁移版本数耦合。
- 处理：测试改为足够步数的全量回滚（当前用 `10` 步兜底）。

### 坑 6：refresh token 没有持久化时，无法真正做到“旋转后失效”

- 问题：仅靠 JWT 自包含校验时，旧 refresh token 在过期前依然可继续换发。
- 根因：服务端缺少会话状态存储，无法识别“已被替换/吊销”的 token。
- 处理：新增 `user_sessions` 表，refresh 时进行 hash 对比与会话状态检查，并在旋转时吊销旧 session。

### 坑 7：测试中 JWT TTL 若写成整数 `1`，会被解释为 1 纳秒

- 问题：登录后立即请求 `me/refresh` 偶发 `401`。
- 根因：`time.Duration` 默认单位是纳秒，TTL 过短导致 token 几乎立即过期。
- 处理：测试配置改为显式时长（`time.Hour`、`24*time.Hour`）。

### 坑 8：在 handler 直接写 SQL，导致职责混杂与后续扩展困难

- 问题：业务逻辑、查询细节、HTTP 映射耦合在同一层，后续加鉴权策略或换存储实现成本高。
- 根因：虽然定义了仓储接口，但认证链路初版未按接口分层实现。
- 处理：将认证链路重构为 `handler -> service -> repository`，并补 GORM 仓储实现，保留统一错误语义。

### 坑 9：SQLite 扫描 `time.Time` 字段在部分模型查询上存在兼容风险

- 问题：直接 `SELECT *` 到包含 `time.Time` 字段的模型时，部分场景会触发扫描错误。
- 根因：SQLite 驱动在时间类型返回值与模型字段解析上存在差异。
- 处理：仓储查询改为显式 `Select` 业务所需列，避免不必要的时间字段扫描。

## 当前验证结论

- `apps/server`：`go test ./...` 通过。
- 仓库根目录：`npm run web:build` 通过。

## 后续建议

1. 主题源当前在前端 TS 与后端 SQL 各维护一份，建议增加自动生成脚本，避免再次漂移。
2. 主题 API 后续可增加“自定义主题 CRUD”，并区分 `builtin` 与用户主题生命周期。
