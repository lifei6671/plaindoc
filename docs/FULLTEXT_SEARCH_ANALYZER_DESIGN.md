# 全文检索分词抽象方案（Jieba + 自定义词典）

**Last Updated**: 2026-03-02  
**适用范围**: `apps/server` 检索子系统（L1/L2 演进）  
**目标**: 在可插拔搜索 Provider 之上再抽象一层分词能力，支持 `jieba` 与用户自定义词典。

---

## 1. 设计目标

本方案解决三个问题：

1. Provider（Bleve/Meili/Typesense）分词行为不一致，导致召回与排序不可控。
2. 业务希望支持中文分词优化，并允许管理员维护自定义词条。
3. 分词策略变化后，需要可回滚、可观测、可重建，不引入权限泄露路径。

---

## 2. 总体架构（新增 Analyzer Layer）

在现有 `SearchService -> Provider` 之间新增抽象层：

1. `SearchService`
2. `AnalyzerLayer`（新增）
3. `SearchProvider`（Bleve/Meili/Typesense）

核心原则：

1. 查询与建索引都必须经过同一 Analyzer。
2. Provider 不直接决定“业务分词语义”，只负责检索执行。
3. 分词词典版本必须可追踪，并与索引构建状态关联。

---

## 3. 统一契约（建议接口）

### 3.1 AnalyzerProvider

```go
type AnalyzerProvider interface {
    Name() string
    Health(ctx context.Context) error
    AnalyzeForIndex(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error)
    AnalyzeForQuery(ctx context.Context, input AnalyzeInput) (AnalyzeOutput, error)
    Reload(ctx context.Context, dictVersion string) error
    Capabilities() AnalyzerCapabilities
}
```

### 3.2 AnalyzerCapabilities

1. `supports_user_dict`
2. `supports_hot_reload`
3. `supports_phrase_hint`
4. `supports_stopwords`
5. `supports_synonyms`（L2）

### 3.3 AnalyzeInput/Output

1. `AnalyzeInput`: `text`, `mode(index|query)`, `language`, `space_id`（可选）
2. `AnalyzeOutput`: `tokens[]`, `normalized_text`, `token_count`, `dict_version`

---

## 4. 索引模型扩展（与 Provider 解耦）

在原 `IndexRecord` 基础上新增字段：

1. `terms`：Analyzer 输出 token 以空格拼接后的检索字段（统一主召回字段）。
2. `title_terms`：标题 token 字段（可选，用于提升标题命中权重）。
3. `analyzer_name`：如 `jieba`。
4. `analyzer_version`：词典版本或策略版本。
5. `body_plain`：Markdown 清洗后的纯文本（用于分词与索引）。

约束：

1. Provider 统一检索 `terms/title_terms`，弱化底层引擎自带分词差异。
2. `title/body` 原文保留用于展示、高亮与兜底。

### 4.1 Markdown 清洗输入规范（强制）

全文检索输入内容必须是“去语法后的纯文本”，不允许直接使用原始 `content_md` 建索引。

必须剔除的内容：

1. 代码块（fenced/indented code block）。
2. 行内代码（`` `code` ``）。
3. 公式（行内 `$...$`、块级 `$$...$$`）。
4. `mermaid` 语法块（```mermaid ... ```）。
5. 其他不应参与全文召回的结构化块（如 `math`/`plantuml` 等 fenced block，可按白名单扩展）。
6. Markdown 语法本身（标题标记、列表符号、链接标记、强调符号、引用符号、表格分隔符等）。

保留原则：

1. 仅保留用户可阅读的自然语言文本。
2. 链接保留“可见文本”，去掉 URL 与 Markdown 包装语法。
3. 图片保留 alt 文本（若有），去掉 URL 与语法包装。

推荐处理流水线：

1. 解析 Markdown AST（避免仅靠正则误删）。
2. 删除需忽略节点（code/math/mermaid 等）。
3. 提取文本节点并做空白归一化。
4. 输出 `body_plain` 供 Analyzer 分词与 Provider 索引。

一致性要求：

1. 建索引与查询高亮摘要使用同一清洗规则版本。
2. 清洗规则变更需触发索引重建（可复用 `dict_version` 流程，增加 `content_normalizer_version`）。

---

## 5. 查询流程（L1）

1. 接收 `/api/spaces/:spaceId/search` 请求。
2. 先计算权限参数：`is_authenticated`、`role_level`、`space_id`。
3. 调用 `AnalyzeForQuery` 得到 `query_terms`。
4. 将 `query_terms` 传给 active provider 执行检索。
5. 严格应用过滤：`space_id` + 可见性 + `min_role <= role_level`。
6. 返回 `doc_id/score/snippet`。

注意：

1. 不允许仅在业务层“先全量召回再过滤”后返回可读字段。
2. 外部 provider 结果可做后置兜底校验，但主过滤必须在引擎侧表达。

---

## 6. Jieba 方案（首选实现）

### 6.1 推荐能力

1. 分词模式：`search`（首期默认）。
2. 支持停用词（可选）。
3. 支持用户词典（必须）。
4. 支持词典热重载（建议）。

### 6.2 词典来源

1. 基础词典：随程序发布（只读）。
2. 用户词典：后台维护（可增删改）。
3. 合并词典：运行时装载到 Jieba 实例。

### 6.3 回退策略

当 Jieba 初始化或加载失败：

1. 不中断服务启动（可配置）。
2. 回退到 `simple analyzer`（按空白/标点切分）。
3. 后台状态标记 `degraded`，并提示管理员修复。

---

## 7. 用户自定义分词能力（后台）

### 7.1 数据结构（建议）

新增表：`search_analyzer_dict_entries`

1. `id`
2. `analyzer`（当前为 `jieba`）
3. `term`
4. `weight`（可选）
5. `tag`（可选）
6. `status`（`active/deleted`）
7. `created_by_user_id`
8. `updated_by_user_id`
9. `created_at/updated_at`

新增表：`search_analyzer_dict_versions`

1. `version_id`
2. `analyzer`
3. `checksum`
4. `entry_count`
5. `created_by_user_id`
6. `created_at`

### 7.2 管理接口（建议）

1. `GET /api/admin/search/analyzers`
2. `GET /api/admin/search/analyzers/jieba/dict`
3. `POST /api/admin/search/analyzers/jieba/dict`（新增词条）
4. `PATCH /api/admin/search/analyzers/jieba/dict/:id`（更新词条）
5. `DELETE /api/admin/search/analyzers/jieba/dict/:id`
6. `POST /api/admin/search/analyzers/jieba/reload`
7. `POST /api/admin/search/analyzers/jieba/analyze-preview`

高风险操作建议：

1. `reload`、`批量导入`、`生效切换` 必须叠加 `RequireAdminOperationToken`。
2. 审计模块复用 `AdminAuditModuleSystemConfig`，记录词典版本、变更数、操作者。

---

## 8. 分词版本与索引一致性

词典变更会影响 token 结果，必须纳入索引状态机：

1. 词典更新 -> `dict_version` 变化。
2. 标记 active provider 为 `building_required`。
3. 触发 `rebuild` 任务（全量或按空间分批）。
4. 重建完成后标记 `ready`，再允许切换 `active`。

推荐约束：

1. 查询侧可立即使用新词典，但后台要明确提示“索引待重建，召回可能暂不完整”。
2. 生产默认策略：词典版本变化后要求重建完成再标记 fully-ready。

---

## 9. 配置模型（system_configs.search 扩展）

建议在 `search` 配置中增加 `analysis` 节点：

```json
{
  "activeProvider": "bleve",
  "fallbackPolicy": "degrade_to_bleve",
  "analysis": {
    "activeAnalyzer": "jieba",
    "analyzers": {
      "jieba": {
        "enabled": true,
        "mode": "search",
        "hmm": true,
        "stopwordsEnabled": false,
        "dictSource": "db",
        "dictVersion": "v2026-03-02-001"
      },
      "simple": {
        "enabled": true
      }
    }
  }
}
```

校验要求：

1. `activeAnalyzer` 必须存在且启用。
2. `dictVersion` 格式受控（避免任意字符串污染）。
3. 配置变更沿用当前 `system_configs.version` 乐观锁机制。

---

## 10. 测试清单（新增）

### 10.1 功能测试

1. 自定义词条新增后，`analyze-preview` 能立即看到 token 变化。
2. 重载成功后，新请求使用新词典。
3. 分词服务故障时正确回退 `simple analyzer`。
4. `body_plain` 不包含代码块、公式、mermaid 与 Markdown 语法残留。

### 10.2 权限与隔离测试

1. 分词变化不影响 `space_id` 与 `min_role` 过滤正确性。
2. Reader/Collaborator/Owner 的结果边界不被分词策略突破。
3. 跨空间查询仍被强制阻断。

### 10.3 一致性测试

1. `dict_version` 变化触发 rebuild 任务。
2. rebuild 前后命中结果符合预期（新词召回提升）。
3. 回滚到旧词典版本后可恢复旧召回行为。
4. `content_normalizer_version` 变化触发 rebuild，且清洗结果可回归验证。

---

## 11. 分阶段实施建议

1. **Phase A（抽象先行）**  
   引入 `AnalyzerProvider`、`simple analyzer`、接口与配置骨架，不改现有业务 API。
2. **Phase B（Jieba 接入）**  
   接入 `jieba analyzer`，完成 `analyze-preview` 与 `Health/Reload`。
3. **Phase C（自定义词典）**  
   增加词典表、后台 CRUD、审计、operation token 保护。
4. **Phase D（版本联动）**  
   打通 `dict_version -> rebuild -> ready/active` 状态机。
5. **Phase E（L2 增强）**  
   停用词、同义词、短语提示、分词质量指标与压测优化。

---

## 12. 与现有方案关系

本文件是 `docs/BACKEND_DEVELOPER_GUIDE.md` 第 13 章的增强专题，重点补齐“分词可插拔与自定义能力”。

建议执行顺序：

1. 先按后端总方案交付 `L1 + Provider + 索引任务系统`。
2. 再按本专题接入 `Analyzer Layer`，避免一次性改动面过大。
