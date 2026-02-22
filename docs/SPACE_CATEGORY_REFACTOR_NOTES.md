# 空间分类独立表改造说明

**Last Updated**: 2026-02-22  
**Scope**: `apps/server` + `apps/web` 的空间分类数据模型与管理接口改造  
**Goal**: 将“空间分类”从配置表迁移为独立实体，降低维护成本并支持稳定的关联关系

---

## 1. 背景与目标

原方案将空间分类维护在 `system_configs` 中，存在以下问题：
1. 分类属于结构化业务实体，却以配置 JSON 形式维护，读写与校验复杂。
2. 空间与分类之间无法建立稳定关联，仅能依赖分类名称，重命名/删除风险高。
3. 分类管理扩展（重命名、去重、删除迁移）难以通过数据库约束保障一致性。

改造后目标：
1. 分类使用独立表存储，空间通过 `category_id` 关联分类。
2. 分类名称唯一，禁止重名。
3. 删除分类时，空间自动迁移到默认分类“未分类”。
4. 系统初始化保证存在默认分类“未分类”。

---

## 2. 数据模型变更

## 2.1 新增表 `space_categories`

核心字段：
1. `category_id`：业务主键（ULID 小写），生成规则与 `space_id` 保持一致。
2. `name`：分类名称，唯一约束。
3. `is_default`：是否默认分类（`未分类`）。
4. `created_at` / `updated_at`：审计时间。

默认常量：
1. 默认分类 ID：`01jmf4v2x7m7f1m6qv5kh0t2mn`
2. 默认分类名称：`未分类`

## 2.2 变更表 `spaces`

1. 新增字段 `category_id`（非空，默认指向“未分类”）。
2. 现有 `category` 字段保留作为冗余展示名（由服务层同步维护）。

---

## 3. 迁移脚本

版本：`0011_space_category_relation`（sqlite / mysql / postgres 全量提供）。

Up 迁移主要动作：
1. 创建 `space_categories` 表与索引/约束。
2. 插入默认分类“未分类”。
3. 为 `spaces` 增加 `category_id` 并设默认值。
4. 将历史空分类名称回填为“未分类”。

Down 迁移主要动作：
1. 删除关联索引/外键（不同数据库按方言处理）。
2. 从 `spaces` 移除 `category_id`。
3. 删除 `space_categories` 表。

---

## 4. 后端接口与行为约束

## 4.1 分类管理接口（后台）

1. `GET /api/admin/spaces/categories`：获取分类列表。
2. `POST /api/admin/spaces/categories`：新增分类（名称唯一）。
3. `PATCH /api/admin/spaces/categories/:categoryId`：重命名分类（名称唯一）。
4. `DELETE /api/admin/spaces/categories/:categoryId`：删除分类并迁移空间到“未分类”。

行为约束：
1. 默认分类不可重命名、不可删除。
2. 分类名去除首尾空格后判重。
3. 删除与迁移在事务中执行，保证空间不会出现“悬空分类”。

## 4.2 空间接口字段调整

1. 创建空间：`POST /api/admin/spaces` 使用 `categoryId`。
2. 编辑空间元数据：`PATCH /api/admin/spaces/:spaceId/metadata` 使用 `categoryId`。
3. 空间列表响应新增：
   - `categoryId`
   - `category`（名称）
   - `categoryIsDefault`

---

## 5. 前端改造点（管理后台）

1. 分类管理 UI 从“配置文本编辑”改为“列表式维护”。
2. 支持新增、重命名、删除分类。
3. 新建/编辑空间在可见性后新增分类选择，提交 `categoryId`。
4. 默认选中“未分类”。

---

## 6. 错误码补充

新增错误码（`apps/server/internal/server/response/error_codes.go`）：
1. `SPACE_CATEGORY_NOT_FOUND`（3012）
2. `SPACE_CATEGORY_NAME_EXISTS`（4009）
3. `SPACE_CATEGORY_DEFAULT_IMMUTABLE`（5015）

说明：接口 HTTP 状态维持项目既有约束（仅 `200/403`），具体错误语义通过 `JsonResult.code` 返回。

---

## 7. 测试与回归

1. 增加并通过分类管理路由集成测试：`TestRouter_AdminSpaceCategoriesManageAndApply`。
2. 修复迁移回滚测试步数硬编码问题，改为动态按迁移总数回滚，避免后续迁移新增导致误报：
   - `apps/server/internal/storage/migrate_test.go`
3. 推荐回归命令：
   - `cd apps/server && GIN_MODE=release go test ./... -count=1`
   - `cd apps/server && go test ./internal/storage -run TestMigrateUpAndDown_SQLite -count=1`
   - `npm run web:build`
