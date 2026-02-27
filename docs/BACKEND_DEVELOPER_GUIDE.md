# 后端开发指南（`apps/server`）

**Last Updated**: 2026-02-27  
**适用对象**: 新加入项目的后端工程师、全栈工程师、AI Agent  
**目标**: 统一后端认知口径，快速进入可开发、可联调、可发布状态

---

## 1. 后端定位与现状

`apps/server` 是 PlainDoc 的单体入口，当前承担：

1. 业务 API（认证、空间、文档、主题、管理后台）。
2. 页面 SSR（首页、分类页、阅读页网关）。
3. Node SSR Worker 生命周期管理（Go 拉起子进程）。
4. 统一权限、审计、错误协议、配置加载与日志链路。

截至 2026-02-26，后端主链路已可用，阅读页 SSR 的测试/性能加固仍在持续完善。

---

## 2. 技术栈

核心栈（来自 `apps/server/go.mod`）：

1. Go `1.26`
2. Gin `1.11`
3. GORM `1.31`
4. SQLite / MySQL / PostgreSQL 三驱动
5. JWT v5 + bcrypt（认证会话）
6. `gg` + `x/image` + `webp`（封面生成与图片规范化）
7. 自研 SSR 子进程池（`internal/ssr/*`）

---

## 3. 架构与代码结构

### 3.1 分层规则

1. `handler`：参数校验、HTTP 映射、调用 service。
2. `service`：业务编排、权限规则、审计落点。
3. `repository`：GORM 数据访问实现与事务边界。
4. `router`：统一依赖注入与中间件顺序控制。

### 3.2 关键目录

1. `apps/server/cmd/server/main.go`：服务启动、配置加载、SSR Worker 初始化。
2. `apps/server/internal/config/`：配置模型与校验。
3. `apps/server/internal/server/`：路由、handler、中间件、响应协议、视图模板。
4. `apps/server/internal/service/`：业务服务层。
5. `apps/server/internal/storage/`：数据库连接、迁移、模型、仓储。
6. `apps/server/internal/ssr/`：Worker 协议、进程、进程池、调度器。

---

## 4. 已实现功能（按模块）

### 4.1 认证与会话

1. `POST /api/auth/register`：注册新用户账号，并返回可用会话信息。
2. `POST /api/auth/login`：账号登录，签发 access/refresh token。
3. `POST /api/auth/refresh`：刷新 access token，同时执行 refresh token 旋转与旧 token 失效。
4. `GET /api/auth/me`：获取当前登录用户信息（会话校验入口）。
5. `POST /api/auth/logout`：退出当前会话并吊销对应服务端会话状态。

LDAP/统一认证改造任务请优先阅读：`docs/LDAP_DIRECT_AUTH_IMPLEMENTATION_PLAN.md`。

### 4.2 空间与文档协作

1. `GET/POST /api/spaces`：`GET` 用于获取当前用户可见空间列表；`POST` 用于创建新空间并建立 owner 关系。
2. `GET /api/spaces/:spaceId/tree`：获取空间目录树（编辑器左侧树数据源）。
3. `POST /api/spaces/:spaceId/nodes`：在指定空间下创建目录或文档节点（文档节点会联动创建文档记录）。
4. `PATCH/DELETE /api/nodes/:nodeId`：`PATCH` 用于节点重命名/移动/排序；`DELETE` 用于删除节点及其子树数据。
5. `GET/PUT /api/docs/:docId`：`GET` 读取文档正文与元信息；`PUT` 保存文档并执行版本冲突检测。
6. `GET /api/docs/:docId/revisions`：获取文档修订历史列表，用于版本追溯。
7. `PUT /api/spaces/:spaceId/visibility`：更新空间可见性（`public/authenticated/member`）。
8. `PUT /api/docs/:docId/visibility`：更新文档可见性策略（文档级访问控制）。
9. `PUT /api/docs/:docId/theme`：为文档设置主题（theme_id 绑定）。

### 4.3 管理后台

当前后台接口已覆盖：

1. `GET /api/admin/me` 与个人资料管理。
2. 用户治理（列表、创建、角色、封禁/解封、删除）。
3. 空间治理（列表、新建、封面资产、分类管理、成员管理、状态、元数据、转移、删除）。
4. 文档治理（列表、状态、删除）。
5. 主题治理（仅 `platform_admin`）。
6. 系统配置治理（仅 `platform_admin`）。
7. 审计日志检索（仅 `platform_admin`）。
8. 高风险操作一次性 token（防重放、绑定 actor/operation/target）。

### 4.4 SSR 与页面

1. 首页 SSR：`GET /`
2. 分类页 SSR：`GET /explore/:categoryId`
3. 阅读入口：`GET /r/:spaceId`
4. 阅读页 SSR：`GET /r/:spaceId/:docId`（Go -> Node Worker）
5. 降级策略：Worker 失败时回退基础壳页，避免 500 雪崩

---

## 5. 本地运行与编译打包教程

### 5.1 环境要求

1. Go `1.26+`
2. Node.js `>=20`（启用 SSR Worker 时必需）
3. C 编译环境（编译时使用 `github.com/chai2010/webp` 需要 CGO）

### 5.2 本地启动

```bash
cd apps/server
cp .env.example .env
go mod tidy
go run ./cmd/server
```

默认地址：`http://localhost:8080`

### 5.3 本地测试

```bash
cd apps/server
go test ./... -count=1
```

常用专项：

```bash
cd apps/server
go test ./internal/server -run TestRouter_Admin -count=1
go test ./internal/storage -run TestMigrateUpAndDown_SQLite -count=1
```

### 5.4 后端二进制编译

```bash
cd apps/server
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o ../../release/plaindoc-server-linux-amd64 ./cmd/server
```

说明：`CGO_ENABLED=1` 是当前必需条件（`webp` 依赖）。

### 5.5 发布打包（Release 流程）

GitHub Actions `release.yml` 会自动产出：

1. `plaindoc-server-linux-amd64`
2. `plaindoc-server-linux-amd64-<tag>.tar.gz`（含后端 + `dist` + `dist-ssr`）
3. `plaindoc-web-<tag>.tar.gz`
4. `checksums-<tag>.txt`

### 5.6 Docker 打包

仓库根目录：

```bash
docker build -t plaindoc:latest .
docker run --rm -p 8080:8080 plaindoc:latest
```

---

## 6. 常见踩坑指南（高频）

1. SQLite 时间字段扫描失败：避免盲目 `SELECT *` 直接扫 `time.Time`，必要时先按字符串解析归一化。
2. JWT TTL 单位误用：`time.Duration(1)` 是 1 纳秒，必须显式 `time.Hour` 等时长。
3. SSR Worker 启动失败：优先检查 `SSR_WORKER_EXEC`、`SSR_WORKER_ENTRY`、`SSR_PROTOCOL_VERSION`。
4. `.env` 不生效：后端会按多候选路径加载，但系统环境变量优先级更高。
5. 编译失败出现 `webp` 符号问题：确认 CGO 已开启且构建环境具备 C 工具链。
6. 阅读页/图片本地不可访问：前端开发代理必须包含 `/r` 与 `/uploads`。
7. operation token 校验失败：确认目标类型/目标 ID/操作者绑定一致，且 token 未消费过。
8. 分类数据异常：默认分类“未分类”不可删不可改名，删除普通分类要事务迁移空间。

---

## 7. 运维与配置要点

1. 生产必须替换 `JWT_SECRET`。
2. 使用 SQLite 时建议开启 WAL 并配置数据备份策略。
3. 阅读 SSR 依赖 Node 运行时，缺失时可临时 `SSR_WORKER_ENABLED=false` 降级。
4. 管理后台高风险操作依赖审计与 operation token，不建议绕开。

---

## 8. 上手建议（新成员 / AI Agent）

1. 先读 `cmd/server/main.go`、`internal/server/router.go`、`internal/config/config.go`。
2. 再读目标业务的 `handler/service/repository` 三层实现。
3. 改动 API 契约前先核对 `apps/web/src/data-access/types.ts` 与 `http/adapter.ts`。
4. 提交前至少执行：
   - `go test ./... -count=1`
   - `npm run web:build`（跨端契约回归）

---

## 9. 关联文档（历史与专项）

1. `docs/BACKEND_IMPLEMENTATION_PHASES.md`
2. `docs/backend-ai-handoff.md`
3. `docs/ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`
4. `docs/ADMIN_CONSOLE_RELEASE_CHECKLIST.md`
5. `docs/HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`
6. `docs/SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`
7. `docs/SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`
8. `docs/SPACE_CATEGORY_REFACTOR_NOTES.md`
9. `docs/LDAP_DIRECT_AUTH_IMPLEMENTATION_PLAN.md`
