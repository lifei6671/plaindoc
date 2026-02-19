# 后端与主题迁移进展记录（2026-02-19）

> 更新时间：2026-02-19  
> 目的：沉淀当天已落地能力与踩坑结论，降低后续迭代回归风险。

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

## 当前验证结论

- `apps/server`：`go test ./...` 通过。
- 仓库根目录：`npm run web:build` 通过。

## 后续建议

1. 主题源当前在前端 TS 与后端 SQL 各维护一份，建议增加自动生成脚本，避免再次漂移。
2. 主题 API 后续可增加“自定义主题 CRUD”，并区分 `builtin` 与用户主题生命周期。
