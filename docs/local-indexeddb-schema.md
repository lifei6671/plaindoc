# 本地工作区 IndexedDB 设计（树结构 + 文档）

## 设计目标
- 浏览器本地持久化使用 IndexedDB（Dexie）。
- 业务主键统一为小写 `ulid`，不使用 UUID。
- 保留自增 `id` 作为技术主键，便于后续对齐数据库模型。

## 表结构

### `spaces`
- `id` `number`：自增主键（`++id`）
- `ulid` `string`：业务主键（唯一 `&ulid`）
- `name` `string`
- `createdAt` `string`
- `updatedAt` `string`

索引：
- `&ulid`
- `updatedAt`
- `createdAt`

### `nodes`（目录树节点）
- `id` `number`：自增主键
- `ulid` `string`：业务主键（节点 ID）
- `spaceUlid` `string`：所属空间
- `parentUlid` `string | null`：父节点 ID，`null` 表示根节点
- `type` `"folder" | "doc"`
- `title` `string`
- `sort` `number`：同级排序值
- `createdAt` `string`
- `updatedAt` `string`

索引：
- `&ulid`
- `spaceUlid`
- `parentUlid`
- `[spaceUlid+parentUlid]`
- `[spaceUlid+parentUlid+sort]`

### `documents`
- `id` `number`：自增主键
- `ulid` `string`：业务主键（文档 ID）
- `nodeUlid` `string`：关联节点 ID（唯一 `&nodeUlid`）
- `title` `string`
- `contentMd` `string`
- `version` `number`
- `createdAt` `string`
- `updatedAt` `string`

索引：
- `&ulid`
- `&nodeUlid`
- `version`
- `updatedAt`

### `revisions`
- `id` `number`：自增主键
- `ulid` `string`：业务主键
- `documentUlid` `string`
- `version` `number`
- `contentMd` `string`
- `baseVersion` `number`
- `createdAt` `string`
- `source` `"local" | "remote"`

索引：
- `&ulid`
- `documentUlid`
- `[documentUlid+version]`
- `[documentUlid+createdAt]`

### `users` / `meta`
- `users` 用于本地登录态用户数据（同样 `id + ulid` 双主键语义）。
- `meta` 保存会话等元信息（例如 `session_user_ulid`）。

## 关系约束
- `nodes.type = "doc"` 时，必须存在一条 `documents.nodeUlid = nodes.ulid`。
- 当前实现保持 `document.ulid === node.ulid`，兼容现有前端文档打开链路。
- 删除节点时递归删除子树；若为文档节点，联动删除 `documents` 与 `revisions`。

## ID 策略
- 使用 `ulid` 库生成业务 ID，并统一转为小写。
- 生成流程包含冲突检测（按实体表校验）和重试（默认最多 8 次）。
- 表层通过 `&ulid` 唯一索引再次兜底冲突。

## 抽象分层
- `local/store.ts`：
  - 负责 Dexie 建表、初始化 seed、事务入口、ULID 生成、实体映射与树构建。
- `local/adapter.ts`：
  - 负责业务编排（空间、节点、文档、版本）和错误语义，不关心底层存储细节。
