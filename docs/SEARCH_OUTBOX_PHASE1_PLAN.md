# PlainDoc 检索 Outbox 一期实施清单（执行版）

最后更新：2026-03-03（一期开发已落地）  
适用范围：`apps/server` 全文检索增量同步链路（Outbox + Worker）

## 1. 本期范围确认

根据本轮确认，本期必须完成：

1. 空间可见性变更和空间删除，纳入 Outbox 任务流。
2. 文档/空间状态更新（含封禁与恢复）纳入 Outbox 任务流。
3. 防抖窗口（debounce window）延期到下期，不在本期实现。

## 2. 本期目标

1. 将“进程内异步队列”升级为“持久化 Outbox + 定时 Worker”。
2. 将写库与索引任务入队绑定到同一事务，避免任务丢失。
3. 复用现有后台循环模式，落地 `runSearchIndexJobLoop(...)`。
4. 保持现有 `SearchIndexService` 的索引执行口径（`SyncDocumentByID/DeleteDocumentByID/SyncSpaceByID/PurgeSpaceByID`）不回退。

## 3. 任务清单（一期）

状态说明：`待实现`、`进行中`、`已完成`、`延期`  
优先级说明：`P0`（本期必做）、`P1`（本期可选，不建议阻塞）

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| O1 | 已完成 | P0 | 新增 `search_index_jobs` 三库迁移（0024） | `internal/storage/migrations/{sqlite,mysql,postgres}/0024_search_index_jobs.*.sql` | 三库迁移可执行，索引/约束齐全 |
| O2 | 已完成 | P0 | 定义任务模型与枚举：`DOC_UPSERT/DOC_DELETE/SPACE_PURGE/REBUILD_SPACE`，`PENDING/RUNNING/SUCCESS/FAILED`，优先级字段 | `internal/storage/models/search_index_job.go` | 枚举统一、可序列化、可被服务层复用 |
| O3 | 已完成 | P0 | 新增 Outbox 仓储接口与 GORM 实现（拉取、抢占、完成、失败重试、去重合并） | `internal/storage/repository/interfaces.go` + `gorm_search_index_job_repository.go` | 单元测试覆盖核心状态机 |
| O4 | 已完成 | P0 | 新增 `SearchIndexJobService`（任务编排 + 重试退避 + 日志） | `internal/service/search_index_job_service.go` | 可单次消费一批任务并正确回写状态 |
| O5 | 已完成 | P0 | 写侧同事务入队：文档创建/保存/标题变更/可见性变更/删除；空间可见性变更/删除；状态封禁/恢复 | `workspace/document/space repository` | 主写提交后对应 Outbox 记录必存在 |
| O6 | 已完成 | P0 | 移除 Handler 内直接索引调用（`Enqueue* / SyncSpaceByID / PurgeSpaceByID`），统一走写侧事务事件 | `internal/server/handler/{workspace,access,admin_document,admin_space}.go` | 请求线程不再直接触发 provider 索引写入 |
| O7 | 已完成 | P0 | 复用后台循环模式，新增 `runSearchIndexJobLoop(...)` 并在服务启动时挂载 | `internal/server/router.go` | 进程重启后可继续消费未完成任务 |
| O8 | 已完成 | P0 | 兼容执行层：Worker 调用现有 `SearchIndexService` 执行 `upsert/delete/purge/rebuild_space` | `internal/service/search_index_service.go` + `search_index_job_service.go` | 行为与当前全量/增量口径一致 |
| O9 | 已完成 | P0 | 测试：事务原子性、去重（upsert 合并 + delete 吞并 upsert）、失败重试、worker 循环 | `internal/service/*_test.go` + `internal/storage/repository/*_test.go` | 关键路径测试通过 |
| O10 | 延期 | P1 | 指标与状态扩展（队列长度、失败数、最大任务年龄） | `service status/log` | 可观测性增强，不阻塞一期上线 |

## 4. 写侧事件映射（本期必须接入）

| 业务动作 | Outbox 任务类型 | 说明 |
|---|---|---|
| 创建文档 / 保存文档 / 改标题 / 改文档可见性 | `DOC_UPSERT` | 覆盖正文与元信息变更 |
| 删除文档（业务删除/后台删除） | `DOC_DELETE` | 删除优先，需吞并同 doc 未执行 upsert |
| 空间可见性变更 | `REBUILD_SPACE` | 本期按“整空间重建”处理，避免遗漏 |
| 空间删除 | `SPACE_PURGE` | 删除后清空该空间索引数据 |
| 文档状态变更（active/banned） | `DOC_UPSERT` 或 `DOC_DELETE` | 按当前索引策略映射：不可索引态应删除 |
| 空间状态变更（active/banned） | `REBUILD_SPACE` 或 `SPACE_PURGE` | 状态收紧优先，确保不会越权命中 |

## 5. 实施步骤（按顺序）

### Step 1：数据结构与迁移

1. 新增 `0024_search_index_jobs` 在 sqlite/mysql/postgres 三套迁移。
2. 表字段至少包含：`job_id/job_type/dedupe_key/payload_json/status/priority/retry_count/next_run_at/last_error/created_at/updated_at`。
3. 增加关键索引：`status + next_run_at + priority`、`dedupe_key`。

### Step 2：模型与仓储

1. 增加 `SearchIndexJob` 模型与任务/状态常量。
2. 在仓储接口层新增 `SearchIndexJobRepository`。
3. GORM 实现以下能力：
4. `EnqueueOrMerge(...)`：同 `dedupe_key` 合并；`DOC_DELETE` 可吞并旧 `DOC_UPSERT`。
5. `ClaimRunnableJobs(...)`：按优先级和时间拉取可运行任务并标记 `RUNNING`。
6. `MarkSuccess(...)`、`MarkRetry(...)`：失败写回 `next_run_at`（指数退避 + jitter）。

### Step 3：服务层编排

1. 新增 `SearchIndexJobService`，对外提供：
2. `Enqueue*`（文档/空间/状态事件）。
3. `RunOnce(ctx)`（消费一批任务）。
4. 任务执行时调用现有 `SearchIndexService` 方法，避免重复实现索引构建逻辑。

### Step 4：写侧同事务改造

1. 对当前已在事务中的仓储写操作（如 `CreateNode/SaveDocument/DeleteNode`）直接在事务中写 Outbox。
2. 对当前非事务或事务边界在 service 的写操作（如可见性、状态更新、后台删除）补齐事务并写 Outbox。
3. 统一保证“业务写成功 => Outbox 任务存在”。

### Step 5：移除 Handler 直接触发索引

1. 删除 Handler 中对 `searchIndexService.Enqueue* / SyncSpaceByID / PurgeSpaceByID` 的直接调用。
2. Handler 只负责业务写入，不直接关心索引执行。

### Step 6：接入定时 Worker

1. 在 `router.go` 复用后台循环模式新增 `runSearchIndexJobLoop`。
2. 循环调用 `SearchIndexJobService.RunOnce(ctx)`。
3. 根据执行结果动态等待（空队列长等待、失败短重试）。

### Step 7：回归与验收

1. 核对任务流：写操作后可在 `search_index_jobs` 看到任务。
2. 重启服务后任务可继续被消费。
3. 空间可见性变更、空间删除、状态封禁/恢复均能通过 Outbox 生效。
4. 旧进程内队列路径不再作为主路径。

## 6. 一期不做项（明确边界）

1. 防抖窗口与时间窗合并（debounce window）。
2. 多实例并发消费的数据库锁精细化（如 `SKIP LOCKED` 差异策略）。
3. 索引版本切换、双写与原子切流。
4. `content_hash/version` 级乱序保护增强。

## 7. 本轮实现结论

1. 一期 P0 任务已全部落地并通过关键测试（`go test` + `go test -race`）。
2. 本期按确认将“空间可见性变更、空间删除、状态变更（封禁/恢复）”全部纳入 Outbox。
3. `O10` 指标治理已明确延期到下一期，不阻塞本期交付。

## 8. 完成定义（DoD）

1. 迁移通过：sqlite/mysql/postgres 全部通过。
2. 功能通过：本期事件全部写入 Outbox 并被 Worker 消费。
3. 一致性通过：写库成功后不存在任务丢失。
4. 安全通过：状态收紧与空间删除后，不再返回不应可见索引结果。
5. 测试通过：新增单测与集成测试覆盖核心流程。
