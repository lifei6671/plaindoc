# 数据库初始化与迁移中间页技术方案

**文档状态**: In Progress
**创建日期**: 2026-05-17
**适用范围**: `apps/server`、`apps/web`、`docs`
**目标**: 首次启动或版本升级执行数据库初始化/迁移时，服务先打开 HTTP 端口并展示启动中间页，向用户展示迁移进度；启动完成后自动恢复正常页面与 API 逻辑。

---

## 1. 背景与结论

当前启动链路在 `apps/server/cmd/server/main.go` 中按顺序执行：

```text
加载配置
  → 初始化日志
  → 启动 SSR Worker
  → 打开数据库
  → 执行 DB_AUTO_MIGRATE
  → 构建 Gin router
  → ListenAndServe
```

问题在于：数据库迁移完成前，HTTP 服务还没有监听端口。用户打开浏览器时只能看到连接失败或网关超时，前端无法显示“正在迁移”的页面。

本方案采用 **Bootstrap HTTP + StartupState + 可切换 Handler + 迁移进度回调**：

```text
加载配置/日志
  → 创建 StartupState
  → 先绑定端口并启动 Bootstrap HTTP
  → 打开数据库
  → 执行迁移并持续更新 StartupState
  → 构建正式 Gin router
  → 原子切换到正式 router
  → 中间页自动刷新并进入正常应用
```

中间页由后端直接输出内联 HTML/CSS/JS，不依赖数据库、不依赖 SPA 构建产物、不依赖用户登录态。

---

## 2. 目标与非目标

### 2.1 目标

1. 数据库初始化或迁移期间，HTTP 端口已经可访问。
2. 浏览器访问任意页面路由时展示启动中间页。
3. 中间页展示当前阶段、迁移总数、已完成数量、当前迁移版本和名称。
4. 启动成功后，中间页自动刷新回原始 URL。
5. 启动失败时，中间页展示失败状态，并提示查看服务日志。
6. 健康检查能区分 `starting`、`ready`、`failed`。
7. 正式业务 router 构建完成后无重启切换，不更换端口。
8. 迁移进度收集不改变现有迁移 SQL 兼容性，不引入数据库结构变更。

### 2.2 非目标

1. 不做数据库迁移暂停、恢复、取消。
2. 不做 SQL 语句级或数据行级精确进度。
3. 不做迁移历史管理页面。
4. 不做多实例启动锁或分布式迁移协调。
5. 不把启动中间页做成 React SPA。
6. 不在失败页暴露 DSN、完整 SQL、堆栈或敏感配置。

---

## 3. 方案选型

### 3.1 方案 A：前端 App 内增加 Loading 页

做法：前端启动后请求 `/api/healthz` 或新状态接口，不 ready 时显示 Loading。

缺点：

1. 当前迁移期间 HTTP 端口未监听，前端无法加载。
2. 如果 SPA 产物不存在或资源加载失败，仍然没有兜底页面。
3. 只能覆盖前端路由，无法覆盖 SSR 页面或 API 调用。

结论：不推荐作为主方案。

### 3.2 方案 B：先启动 Bootstrap HTTP，ready 后切换正式 Router

做法：后端先启动一个轻量 bootstrap handler，负责中间页、启动状态 API 和健康检查；迁移完成后构建正式 Gin router，并用原子 Handler 切换。

优点：

1. 迁移期间端口可访问，用户有明确反馈。
2. 不依赖数据库和 SPA 产物。
3. 能覆盖 `/admin`、`/login`、`/editor`、`/r`、`/api/*` 等所有请求。
4. 正式启动后不重启进程、不释放端口。

结论：推荐。

### 3.3 方案 C：独立启动页端口

做法：迁移期间启动另一个临时端口或 sidecar 页面。

缺点：

1. 用户访问的主端口仍不可用。
2. 部署、反向代理和健康检查复杂度提高。

结论：不推荐。

---

## 4. 核心架构

### 4.1 组件划分

```text
cmd/server/main.go
  ├── 创建 StartupState
  ├── 创建 SwitchHandler
  ├── 先 Serve BootstrapRouter
  ├── 执行数据库打开与迁移
  ├── 构建正式 Router
  └── SwitchHandler.Set(正式 Router)

internal/server/startup_state.go
  └── 线程安全记录启动阶段、迁移进度、失败信息

internal/server/startup_handler.go
  ├── GET /startup
  ├── GET /api/startup/status
  ├── GET /api/healthz
  └── 未 ready 前的页面/API 兜底

internal/server/switch_handler.go
  └── 原子切换 http.Handler

internal/storage/migrate.go
  └── MigrateOptions.OnProgress 回调
```

### 4.2 启动状态模型

建议新增只驻留内存的启动状态：

```go
type StartupPhase string

const (
	StartupPhaseBooting         StartupPhase = "booting"
	StartupPhaseOpeningDatabase StartupPhase = "opening_database"
	StartupPhaseMigrating       StartupPhase = "migrating"
	StartupPhaseBuildingRouter  StartupPhase = "building_router"
	StartupPhaseReady           StartupPhase = "ready"
	StartupPhaseFailed          StartupPhase = "failed"
)

type StartupSnapshot struct {
	Phase          StartupPhase `json:"phase"`
	Ready          bool         `json:"ready"`
	Failed         bool         `json:"failed"`
	Message        string       `json:"message"`
	CurrentVersion int          `json:"currentVersion,omitempty"`
	CurrentName    string       `json:"currentName,omitempty"`
	AppliedCount   int          `json:"appliedCount"`
	PendingCount   int          `json:"pendingCount"`
	TotalCount     int          `json:"totalCount"`
	StartedAt      string       `json:"startedAt"`
	UpdatedAt      string       `json:"updatedAt"`
}
```

实现要求：

1. 使用 `sync.RWMutex` 或 `atomic.Value` 保证并发安全。
2. `Message` 面向用户，不写 DSN、SQL、堆栈。
3. 完整错误仍写结构化日志。

### 4.3 可切换 Handler

启动初期 `http.Server.Handler` 使用 `SwitchHandler`：

```go
type SwitchHandler struct {
	current atomic.Value // http.Handler
}

func NewSwitchHandler(initial http.Handler) *SwitchHandler {
	handler := &SwitchHandler{}
	handler.current.Store(initial)
	return handler
}

func (handler *SwitchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.current.Load().(http.Handler).ServeHTTP(w, r)
}

func (handler *SwitchHandler) Set(next http.Handler) {
	handler.current.Store(next)
}
```

启动成功后：

```go
startupState.MarkReady()
switchHandler.Set(router)
```

建议先 `net.Listen(cfg.Addr)`，监听成功后再 `httpServer.Serve(listener)`。这样如果端口被占用，可以在迁移前失败退出，避免迁移跑完才发现端口不可用。

---

## 5. Bootstrap 路由行为

### 5.1 页面路由

未 ready 前，以下路径返回中间页：

1. `/`
2. `/login`
3. `/register`
4. `/forgot-password`
5. `/reset-password`
6. `/admin`、`/admin/*`
7. `/editor`、`/editor/*`
8. `/r/*`
9. 其他浏览器页面请求

建议统一返回 `200 text/html`，避免浏览器或反向代理把启动中间态当成错误页。

### 5.2 API 路由

未 ready 前：

1. `GET /api/startup/status` 返回 `200` + 状态 JSON。
2. `GET /api/healthz`：
   - starting：`503`
   - failed：`500`
   - ready：理论上已切换到正式 router。
3. 其他 `/api/*` 返回 `503` + `JsonResult` 风格错误：

```json
{
  "code": 50300,
  "message": "服务正在初始化，请稍后重试",
  "requestId": "",
  "data": {
    "phase": "migrating",
    "ready": false
  }
}
```

启动中间态不是业务错误，因此错误码只需要前端可识别即可，不进入业务 handler。

### 5.3 静态资源

中间页必须使用内联样式和内联脚本，不依赖 `/web-assets/*`。

原因：

1. `WEB_DIST_DIR` 可能不存在。
2. 首次启动时前端产物不应成为数据库迁移状态页的前置条件。
3. 内联页面更适合作为部署兜底页。

---

## 6. 中间页交互设计

页面内容建议：

1. 标题：`PlainDoc 正在初始化`
2. 主状态：如 `正在迁移数据库`
3. 当前迁移：如 `0034 admin_space_transfer_jobs`
4. 进度条：`已完成 12 / 共 34`
5. 提示：`首次启动或版本升级需要完成数据库初始化，请勿关闭服务。`
6. 失败态：`初始化失败，请查看服务日志。`

前端轮询逻辑：

```text
页面加载
  → 每 1 秒 GET /api/startup/status
  → ready=true 时 location.reload()
  → failed=true 时停止轮询并展示失败态
```

浏览器刷新应保留原始 URL。用户访问 `/admin/spaces` 时，迁移完成后自动回到 `/admin/spaces`。

---

## 7. 迁移进度接入

### 7.1 扩展 MigrateOptions

当前 `MigrateUpWithOptions` 已经逐条加载并执行迁移。可以在不改变 SQL 执行语义的前提下扩展：

```go
type MigrateOptions struct {
	Logger     *slog.Logger
	OnProgress func(MigrationProgress)
}

type MigrationProgress struct {
	Phase          string
	Driver         string
	TotalCount     int
	PendingCount   int
	AppliedCount   int
	CurrentVersion int
	CurrentName    string
}
```

回调点：

1. 加载完成迁移列表后：`phase=loaded`
2. 每条迁移执行前：`phase=applying`
3. 每条迁移执行后：`phase=applied`
4. 全部完成后：`phase=complete`
5. 失败时：`phase=failed`

### 7.2 进度粒度

进度以迁移文件为单位，不以 SQL 语句或数据行数为单位。

理由：

1. 三种数据库执行计划不同，无法稳定估算单条 SQL 时间。
2. PostgreSQL 的 dollar-quoted block、MySQL DDL、SQLite DDL 都不适合强行拆小。
3. 文件级进度足以解释“当前卡在哪个迁移”。

---

## 8. 启动失败策略

### 8.1 迁移失败

迁移失败后：

1. `StartupState.MarkFailed(safeMessage, err)`。
2. 中间页进入失败态。
3. `/api/healthz` 返回 `500`。
4. 日志记录完整错误链。
5. 进程默认不立即退出，便于部署环境和用户浏览器看到失败页。

是否保留“失败后立即退出”可作为后续配置项，例如 `STARTUP_FAIL_FAST=true`，本期不做。

### 8.2 数据库连接失败

数据库连接失败与迁移失败同样进入失败态。页面文案只显示：

```text
数据库连接失败，请检查服务日志和数据库配置。
```

不得展示 DSN、用户名、密码、连接串参数。

### 8.3 正式 Router 构建失败

正式 router 构建失败通常是配置、依赖或资源问题，也进入失败态。失败前不要切换 Handler。

---

## 9. 兼容性与风险

### 9.1 兼容性

1. 不修改数据库结构。
2. 不修改现有业务 API 契约。
3. `DB_AUTO_MIGRATE=false` 时仍可展示短暂 `building_router`，随后 ready。
4. SPA 托管逻辑仍由正式 router 的 `registerWebSPARoutes` 负责。
5. 启动状态 API 不要求登录态。

### 9.2 风险点

1. **端口监听提前**：需要确保 bootstrap handler 不访问未初始化数据库。
2. **健康检查语义变化**：ready 前 `/api/healthz` 返回 `503`，部署平台应按启动探针而不是存活探针处理。
3. **Handler 切换竞态**：必须使用原子替换，不在请求中修改 Gin router。
4. **失败信息泄露**：页面只展示安全文案，详细错误只写日志。
5. **SSR Worker 启动顺序**：若 SSR Worker 启动慢，也应纳入启动阶段展示；本期优先覆盖数据库迁移。

---

## 10. 测试方案

### 10.1 单元测试

1. `StartupState` 并发读写 snapshot。（已覆盖）
2. `SwitchHandler` 初始 handler、切换 handler 后请求命中新 handler。（已覆盖）
3. Bootstrap handler：（已覆盖）
   - 页面路由返回 HTML。
   - `/api/startup/status` 返回状态 JSON。
   - `/api/healthz` starting 返回 `503`。
   - 其他 `/api/*` 返回 `503`。
4. 迁移器 `OnProgress`：（已覆盖）
   - 加载后能报告总数。
   - 每条迁移前后回调顺序正确。
   - 失败时回调 `failed`。

### 10.2 集成测试

1. 使用临时 SQLite，模拟慢迁移或注入阻塞回调。
2. HTTP server 启动后，迁移未完成时访问 `/admin` 返回中间页。
3. 迁移完成后访问 `/api/healthz` 返回正式 router 的成功响应。
4. 访问原始页面路径，ready 后刷新可进入正常页面。

### 10.3 验证命令

```bash
cd apps/server && go test -timeout 60s ./...
npm run web:build
git diff --check
```

如改动前端构建入口，则补充：

```bash
npm run test:run -w @plaindoc/web
```

---

## 11. 实施任务拆分

### Task 1：启动状态与可切换 Handler

**文件**

1. [x] 创建：`apps/server/internal/server/startup_state.go`
2. [x] 创建：`apps/server/internal/server/switch_handler.go`
3. [x] 创建：`apps/server/internal/server/startup_state_test.go`
4. [x] 创建：`apps/server/internal/server/switch_handler_test.go`

**验收**

1. [x] 状态 snapshot 并发安全。
2. [x] Handler 可在运行中原子切换。

### Task 2：Bootstrap 中间页与启动状态 API

**文件**

1. [x] 创建：`apps/server/internal/server/startup_handler.go`
2. [x] 创建：`apps/server/internal/server/startup_handler_test.go`

**验收**

1. [x] 页面路由返回中间页 HTML。
2. [x] `/api/startup/status` 返回 JSON。
3. [x] `/api/healthz` 能区分 starting/failed。
4. [x] 未 ready 的普通 API 返回 `503`。

### Task 3：迁移进度回调

**文件**

1. [x] 修改：`apps/server/internal/storage/migrate.go`
2. [x] 修改：`apps/server/internal/storage/migrate_test.go`

**验收**

1. [x] `MigrateOptions.OnProgress` 覆盖 loaded/applying/applied/complete/failed。
2. [x] 不改变 SQLite/MySQL/PostgreSQL 迁移 SQL 执行方式。
3. [x] 现有迁移测试全部通过。

### Task 4：重排 `main.go` 启动流程

**文件**

1. [x] 修改：`apps/server/cmd/server/main.go`
2. [ ] 可选创建：`apps/server/cmd/server/startup.go`
3. [ ] 修改或新增：`apps/server/cmd/server/main_test.go`

**验收**

1. [x] 先监听端口，再执行数据库打开和迁移。
2. [x] 迁移期间请求命中 bootstrap handler。
3. [x] ready 后切换到正式 router。
4. [x] 迁移失败后中间页进入 failed。

### Task 5：文档与部署说明

**文件**

1. [x] 修改：`docs/BACKEND_DEVELOPER_GUIDE.md`
2. [x] 修改：`docs/README.md`

**验收**

1. [x] 后端启动链路说明同步。
2. [x] 健康检查 ready 前 `503` 的部署影响说明清楚。

---

## 12. 推荐落地顺序

1. 先实现 `StartupState` 和 `SwitchHandler`，单测验证边界。
2. 再实现 bootstrap handler，确保不依赖数据库。
3. 扩展迁移器进度回调，保持当前迁移执行语义不变。
4. 最后重排 `main.go`，用集成测试验证迁移期间页面可访问。
5. 完成后同步后端主文档，并执行全量后端测试。

---

## 13. 方案状态

本方案当前为 In Progress。基础启动状态、Bootstrap 中间页、迁移成功路径进度回调和 `main.go` 启动顺序已实现；剩余待补失败路径注入测试和可选的 `main.go` 集成测试。
