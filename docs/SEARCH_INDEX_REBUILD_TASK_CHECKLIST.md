# PlainDoc 索引重建任务清单（项目定制版）

最后更新：2026-03-03  
适用范围：`apps/server` 全文检索链路（`SearchIndexService`、Provider、后台系统配置、读写 Handler）

## 1. 当前项目现状（基于代码）

1. 已有 Provider 抽象与统一模型：`IndexRecord/SearchRequest/SearchResponse` 已落地（`internal/search/provider/types.go`）。
2. 已有 Bleve + Database provider；Meili/Typesense 仍是配置占位，尚未实现 provider。
3. 已有全量重建与增量同步服务：`RebuildActiveProvider/SyncDocumentByID/DeleteDocumentByID/SyncSpaceByID/PurgeSpaceByID`。
4. 已有索引状态面板字段：`RebuildInProgress/LastRebuildSource/LastRebuildIndexedDocuments`。
5. 已接入持久化 Outbox（`search_index_jobs`）+ 定时 worker；`EnqueueSyncDocumentByID/EnqueueDeleteDocumentByID` 作为兼容入口，优先落 Outbox。
6. 权限过滤目前是“索引粗过滤 + DB 二次过滤”混合模式；`min_role` 字段已写入，查询路径已在单空间检索计算 `user_role_level`，但尚未全 provider 强制 `min_role` 过滤。
7. 已具备 `search_index_jobs` 持久化任务表、失败重试退避、基础去重合并；断点续跑与索引版本切换仍待实现。

## 2. 目标架构（本清单默认）

1. 最终一致：允许秒级延迟，但不允许越权结果。
2. 写库与任务同事务：文档主写入成功时，索引任务必须落库。
3. 索引任务持久化：Outbox 表 + worker 轮询消费 + 幂等重试。
4. 统一权限过滤：强制 `space_id` + `min_role <= user_role_level`，同时需要兼顾用户未登录时可访问完全开放的空间和文档。
5. 支持全量重建：重建可观测、可暂停、可恢复、可切换。

## 3. 任务清单（按模块，含优先级与落地）

状态说明：`已具备`、`部分`、`待实现`、`已完成`  
优先级说明：`P0`（必须先做）、`P1`（MVP内）、`P2`（增强）

### A. 规格与契约

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| A1 | 部分 | P0 | 收敛 L1/L2 能力清单并固化降级行为文档（以当前 Bleve/Database 为基线） | `docs/BACKEND_DEVELOPER_GUIDE.md` + 本文 | 能力矩阵与降级条件可直接指导实现 |
| A2 | 部分 | P0 | 扩展 `IndexRecord`：补 `content_hash/source_version/index_version`（可选） | `internal/search/provider/types.go` | 结构覆盖幂等与版本切换所需字段 |
| A3 | 已完成 | P0 | 收敛 `SearchRequest` 的角色语义：明确 `user_role_level` 来源与取值映射 | `internal/service/search_query_service.go` | 查询不再固定传 `UserRoleLevel=0` |
| A4 | 部分 | P0 | 定义“硬约束过滤”：`space_id` 必带，`min_role` 必过滤，不允许业务层后置过滤替代 | `internal/search/provider/*` | 所有 provider 的读路径都执行硬过滤 |
| A5 | 待实现 | P1 | 定义一致性 SLO：延迟阈值、越权零容忍、权限收紧优先级最高 | `docs/` + `admin status` 返回结构 | 后台可见索引延迟与积压状态 |

### B. Outbox / Job 系统

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| B1 | 已完成 | P0 | 新增 `search_index_jobs`（Outbox）表 | `internal/storage/migrations/*/0024_search_index_jobs.*.sql` | 三库迁移通过，索引与约束齐全 |
| B2 | 部分 | P0 | 定义任务类型：`DOC_UPSERT/DOC_DELETE/SPACE_PURGE/REBUILD_SPACE/REBUILD_ALL/DICT_VERSION_BUMP` | `models` + `service` 常量 | 一期已落地前四类，`REBUILD_ALL/DICT_VERSION_BUMP` 待补 |
| B3 | 已完成 | P0 | 状态机与重试：`PENDING->RUNNING->SUCCESS/FAILED(RETRY)` + 指数退避+jitter | `search_index_job_service` | 失败任务自动延期重试且可观测 |
| B4 | 已完成 | P0 | 去重合并规则：同 doc upsert 合并，delete 吞并旧 upsert | `repository` 去重 SQL + service | 队列不被高频保存刷爆 |
| B5 | 已完成 | P0 | 优先级策略：权限收紧/删除高于普通 upsert | `priority` 字段 + 拉取 SQL | 高优任务先消费 |
| B6 | 已完成 | P0 | 同事务写入：文档写入与 outbox 入队在一个 DB 事务中提交 | `workspaceRepo/documentRepo/admin service` | 不再出现“文档已变更但任务丢失” |
| B7 | 部分 | P1 | worker 拉取并发控制：批量拉取、抢占锁、避免多实例重复消费 | `job repo + worker` | 已支持批量 claim，多实例强一致抢占仍可继续增强 |

### C. 写库侧触发器（事件生成）

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| C1 | 已完成 | P0 | 文档新增/编辑/标题改/可见性改统一写 `DOC_UPSERT` 到 Outbox | `workspace/access handlers -> service/repo` | 所有入口一致走 Outbox |
| C2 | 已完成 | P0 | 文档删除写 `DOC_DELETE`（保留当前策略：delete 为主） | `workspace/admin_document` | 删除后不再被旧 upsert 恢复 |
| C3 | 已完成 | P1 | 空间删除/清空写 `SPACE_PURGE`，不在请求线程同步清理索引 | `admin_space/workspace/access` | 删除空间接口不被索引耗时阻塞 |
| C4 | 待实现 | P1 | 预留高优发布任务：发布态触发 `DOC_UPSERT(high)` | 发布功能上线时接入 | 发布后可快速搜到 |
| C5 | 待实现 | P1 | 防抖策略：高频保存合并为一次低优 upsert | `job dedupe + debounce window` | 连续输入不产生线性任务增长 |

### D. Worker 执行器（消费侧）

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| D1 | 已完成 | P0 | 落地独立 Outbox worker（替代进程内临时队列） | `internal/server/router.go` + `service` | 进程重启不丢任务 |
| D2 | 已具备 | P0 | `DOC_UPSERT` 执行流程标准化：读最新 doc -> 分词/清洗 -> provider.Upsert | `search_index_service` | 与当前全量重建口径一致 |
| D3 | 已具备 | P0 | `DOC_DELETE` 标准化：provider.Delete + 状态更新 | `search_index_service` | 删除任务幂等 |
| D4 | 已具备 | P1 | `SPACE_PURGE` 批量路径标准化 | `search_index_service` | 大空间删除可控完成 |
| D5 | 待实现 | P1 | 幂等保护：对比 `version/content_hash/updated_at` 防旧任务覆盖新内容 | `IndexRecord + worker` | 乱序任务不污染索引 |
| D6 | 部分 | P1 | 批量优化：worker 聚合 batch upsert/delete | `provider.Upsert/Delete` 调用层 | 吞吐提升、写放大下降 |
| D7 | 待实现 | P2 | 熔断与降速：provider 不可用时暂停/限速并告警 | worker + admin status | 故障期间系统稳定 |

### E. Provider 可插拔与能力探测

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| E1 | 部分 | P1 | 扩展 capabilities：`batch/purge_by_space/filter_min_role/highlight/sort_updated_at` | `provider/types.go` | 后台可见能力矩阵 |
| E2 | 部分 | P1 | 统一 `Verify/Health/EnsureSchema` 规范；补 Meili/Typesense provider 框架 | `provider/*` | provider 可被后台校验 |
| E3 | 部分 | P0 | 文档化字段映射：`space_id/min_role/title/body/terms/title_terms` | `docs/` | 三类 provider 行为一致可追踪 |
| E4 | 待实现 | P2 | provider 索引版本号机制：索引别名或 meta 版本 | provider + config | 重建切换可原子化 |

### F. 全量重建与版本切换

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| F1 | 部分 | P1 | 全量重建任务化：`REBUILD_SPACE/REBUILD_ALL` 走 Outbox，而非纯同步调用 | `search_index_service + jobs` | 后台触发后可异步观测进度 |
| F2 | 待实现 | P1 | 断点续跑：记录游标（`last_doc_id` 或 `updated_at`） | `jobs payload/progress` | 进程中断后可续跑 |
| F3 | 待实现 | P2 | 重建期写入策略：双写旧索引+新索引（可配置） | `index writer` | 切换窗口无写丢失 |
| F4 | 待实现 | P2 | 原子切换 active index version | `search config + provider` | 切换不影响在线查询 |
| F5 | 待实现 | P1 | 重建失败恢复：保留旧索引、清理半成品、支持重试 | `worker + admin op` | 重建失败不影响线上 |
| F6 | 部分 | P1 | 明确重建触发条件：词典版本变更、索引损坏、漂移、空间级治理 | `admin_search_analyzer + status` | 触发规则可配置可审计 |

### G. 搜索读路径与安全约束

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| G1 | 部分 | P0 | 强制注入过滤：所有 provider 查询都必须包含 `space_id + min_role<=user_role_level` | `search_query_service + provider` | 单空间已做 `min_role` 硬拒绝，跨空间与 DB provider 仍需继续收口 |
| G2 | 已完成 | P0 | 查询前计算 `user_role_level`（owner/collaborator/reader/guest） | `search_query_service + repo` | `UserRoleLevel` 不再恒为 0 |
| G3 | 部分 | P0 | 结果字段安全：仅返回已过滤命中的标题/片段，不做先召回后返回 | `provider.Search` | 不出现越权片段泄露 |
| G4 | 部分 | P1 | 分页稳定性：`relevance`/`updated_at` 排序规则统一并写测试 | `provider.Search` | 分页翻页稳定 |

### H. 观测、后台治理与修复

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| H1 | 待实现 | P1 | Outbox 指标：队列长度、成功率、失败率、重试次数、任务年龄 | `worker + metrics/log` | 后台可见积压与健康度 |
| H2 | 部分 | P1 | provider 健康指标：health、写入耗时、搜索耗时、错误率 | `search index status + logs` | 发现故障有依据 |
| H3 | 部分 | P1 | 后台运维动作：单文档重建、空间重建、全量重建、暂停/恢复 worker | `admin handler/service` | 管理动作可审计 |
| H4 | 待实现 | P2 | 一致性巡检任务：抽样对比 DB 与索引（`updated_at/version/hash`） | `periodic checker` | 漂移可被自动发现 |

### I. 测试与验收（必须）

| 编号 | 状态 | 优先级 | 任务 | 主要落地位置 | 验收标准 |
|---|---|---|---|---|---|
| I1 | 已完成 | P0 | 幂等性测试：upsert 合并、delete 吞并旧 upsert | `search_index_service_test` + job tests | 乱序与重复任务不出错 |
| I2 | 待实现 | P0 | 权限安全测试：guest/reader/collaborator/owner 边界一致 | provider + query tests | 无角色越权命中 |
| I3 | 待实现 | P1 | 防抖测试：连续保存期间任务合并，停止后触发一次 | outbox dedupe tests | 防抖窗口生效 |
| I4 | 部分 | P1 | 重建测试：可断点续跑、完成切换、失败回滚 | rebuild tests | 重建可恢复且不影响线上 |
| I5 | 待实现 | P1 | 容错测试：provider 短故障可重试恢复，长故障可降速/暂停 | worker tests | 系统可持续服务 |
| I6 | 待实现 | P2 | 性能基准：批量导入/重建与在线检索延迟影响评估 | benchmark/report | 达到预设 SLO |

## 4. 两阶段交付建议（可直接排期）

### 4.1 MVP（2 周可上线）

1. 完成 A1-A4（契约冻结与硬约束明确）。
2. 完成 B1-B6（Outbox 表、事务写入、去重与优先级最小集）。
3. 完成 C1-C3（所有写路径改为写 Outbox，不在请求线程做重索引）。
4. 完成 D1-D4（worker 消费与基础重试）。
5. 完成 G1-G3（读路径权限硬过滤）。
6. 完成 H3 最小操作（单文档/空间/全量重建）。
7. 完成 I1-I2（幂等与权限安全必须先过）。

### 4.2 增强版（随后 2~4 周）

1. 完成 B7、D5-D7（并发消费、幂等保护、熔断降速）。
2. 完成 F2-F6（断点续跑、版本切换、失败恢复、触发策略完善）。
3. 完成 E2-E4（Meili/Typesense 接入与版本机制）。
4. 完成 H1-H2-H4（指标与一致性巡检）。
5. 完成 I3-I6（防抖、容错、性能全量验收）。

## 5. 实施顺序建议（避免返工）

1. 先做事务 Outbox（B1-B6），再做 worker（D1-D4），最后做读路径安全收口（G1-G3）。
2. `SyncSpaceByID/PurgeSpaceByID` 请求线程直调索引已在一期移除，后续入口新增需继续保持 Outbox-first。
3. `EnqueueSyncDocumentByID/EnqueueDeleteDocumentByID` 保留为过渡 API，内部已优先写 Outbox。
4. 索引版本切换（F3-F4）不要提前做，等 Outbox 与权限硬约束稳定后再上。
