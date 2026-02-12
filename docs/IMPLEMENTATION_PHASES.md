# Implementation Phases: 本地工作区（当前执行基线）

**Project Type**: 现有 Web 前端功能扩展（Local Workspace MVP）  
**Scope**: `localAdapter` + 文档树工作区 + 本地持久化 + ULID 业务 ID  
**Stack**: React + Vite + TypeScript + React Complex Tree + Dexie  
**Current Date**: 2026-02-12

---

## 范围与边界（已确认）

**本期已固定目标**：
- 工作区仅负责“当前知识本内文档树”管理。
- 目录树支持文档/目录新建、重命名、删除。
- 文档节点点击后与编辑区内容联动。
- 刷新后恢复上次激活空间和激活文档。
- 本地业务 ID 统一为小写 ULID。

**本期不做**：
- 拖拽排序（仅保留扩展点）。
- 空间删除（仅保留扩展点）。
- 空间管理 UI（例如空间切换器入口）。
- 跨标签页实时同步。

---

## 阶段总览

1. Phase 1 `工作区状态层`：已完成
2. Phase 2 `侧栏布局与交互骨架`：已完成
3. Phase 3 `目录树交互与编辑体验`：已完成
4. Phase 4 `本地数据层与 ULID 策略`：已完成
5. Phase 5 `状态恢复与持久化增强`：部分完成
6. Phase 6 `验收文档与回归基线`：进行中

---

## Phase 1: 工作区状态层
**Status**: Completed  
**Type**: UI  
**Files**:
- `apps/web/src/workspace/types.ts`
- `apps/web/src/workspace/use-workspace.ts`
- `apps/web/src/App.tsx`
- `apps/web/src/data-access/types.ts`

**当前已落地能力**:
- 工作区状态统一收敛到 `useWorkspace`。
- 启动流程封装 `bootstrapWorkspace`，并保证空间/文档兜底创建。
- 文档切换、树刷新、节点 CRUD 调用链由 Hook 提供统一入口。
- 扩展点 `moveNode` / `deleteSpace` 已预留能力位。

**后续注意点**:
- 不要把空间/文档初始化逻辑散回 `App.tsx`。
- 扩展点未实现前应保持明确错误提示，不可静默失败。

**Exit Criteria**:
- 启动、切换、保存链路稳定；`App` 不直接管理工作区初始化细节。

---

## Phase 2: 侧栏布局与交互骨架
**Status**: Completed  
**Type**: UI  
**Files**:
- `apps/web/src/components/WorkspaceSidebar.tsx`
- `apps/web/src/App.tsx`
- `apps/web/src/styles.css`

**当前已落地能力**:
- 左侧工作区已集成到主布局。
- 侧栏支持拖拽调宽。
- 侧栏支持隐藏/展开。
- 侧栏宽度与折叠状态已持久化。
- 侧栏无额头部，符合“只管文档树”的产品定位。

**后续注意点**:
- 不要在侧栏重新引入空间管理操作。
- 侧栏尺寸状态需要继续保持可恢复，不要改成会话级临时状态。

**Exit Criteria**:
- 侧栏布局稳定且不影响编辑区核心体验。

---

## Phase 3: 目录树交互与编辑体验
**Status**: Completed  
**Type**: UI  
**Files**:
- `apps/web/src/components/WorkspaceTree.tsx`
- `apps/web/src/components/WorkspaceSidebar.tsx`
- `apps/web/src/App.tsx`

**当前已落地能力**:
- 目录树基于 `react-complex-tree`。
- 不仅叶子节点，所有文档节点都可点击打开。
- 节点 hover 显示右侧 `+` 操作菜单。
- 新建文档/目录后进入树内原位输入框。
- 重命名已改为树内原位输入框，不再用 `prompt`。
- 当前激活文档节点有灰色选中背景。
- 树节点行高统一 `36px`。

**后续注意点**:
- 必须保留 `canInvokePrimaryActionOnItemContainer`，避免退化为仅叶子可点击。
- 选中背景与默认背景类要保持互斥，避免激活态被覆盖。
- 输入框交互应保持一致：`Enter` 提交、`Esc` 取消、`Blur` 提交。

**Exit Criteria**:
- 树浏览、文档打开、创建重命名交互形成完整闭环。

---

## Phase 4: 本地数据层与 ULID 策略
**Status**: Completed  
**Type**: API  
**Files**:
- `apps/web/src/data-access/local/ulid.ts`
- `apps/web/src/data-access/local/store.ts`
- `apps/web/src/data-access/local/adapter.ts`
- `apps/web/src/data-access/types.ts`
- `docs/local-indexeddb-schema.md`

**当前已落地能力**:
- 本地存储已迁移为 IndexedDB（Dexie），不再使用 `localStorage` 大对象数据库。
- 业务 ID 全面切换为小写 ULID。
- ULID 具备冲突检测与重试策略（默认 8 次）。
- 核心表采用双主键语义：`id`（技术）+ `ulid`（业务）。
- 节点递归删除与文档修订链路已联动处理。

**后续注意点**:
- 不要在 Dexie 事务内调用事务外异步流程（尤其是 ID 生成）。
- 该问题会直接触发 `Transaction committed too early`。
- 数据模型详见 `docs/local-indexeddb-schema.md`。

**Exit Criteria**:
- 本地 CRUD、版本保存、ID 生成与唯一性策略稳定可用。

---

## Phase 5: 状态恢复与持久化增强
**Status**: Partial  
**Type**: Integration  
**Files**:
- `apps/web/src/App.tsx`
- `apps/web/src/workspace/use-workspace.ts`
- `apps/web/src/components/WorkspaceTree.tsx`

**已完成**:
- 刷新后恢复 `activeSpaceId` 与 `activeDocId`。
- 恢复后自动加载目标文档内容到编辑区。
- 侧栏宽度与折叠状态持久化已稳定。

**待完成**:
- `expandedFolderIds` 的持久化与恢复。
- 删除上次激活节点后的恢复策略专项回归（含嵌套目录场景）。

**后续注意点**:
- 启动恢复顺序：先空间后文档，目标缺失时必须回退到可用文档。
- 不要让刷新恢复破坏当前自动保存与冲突检测链路。

**Exit Criteria**:
- 树状态与文档状态在刷新后可稳定恢复，且回退策略可预期。

---

## Phase 6: 验收文档与回归基线
**Status**: In Progress  
**Type**: Testing  
**Files**:
- `docs/local-workspace-ai-handoff.md`
- `docs/LOCAL_WORKSPACE_ACCEPTANCE.md`（待新增）

**已完成**:
- 已有 AI 接手上下文文档，覆盖实现边界与高风险注意点。

**待完成**:
- 补充正式手工验收清单文档（按场景逐条打勾）。
- 固化本地工作区专项回归项（创建、重命名、删除、刷新恢复、崩溃兜底）。

**后续注意点**:
- 每次继续开发后最少执行一次 `npm run web:build`。
- 回归必须包含“刷新恢复激活文档”场景，不可只测首次启动。

**Exit Criteria**:
- 验收清单可复用，后续 AI 能基于文档稳定推进而不回退既有能力。

---

## 关键约束（后续 AI 必须遵守）

1. 工作区只管理文档树，不扩展为空间管理面板。
2. 本期禁止回退 ULID 方案，也禁止恢复自增业务 ID。
3. 新建/重命名必须保持树内输入框交互，不使用 `prompt`。
4. 所有文档节点都必须可点击打开（不限是否叶子）。
5. 任何改动后若破坏激活高亮、刷新恢复、事务稳定性，视为阻断问题。

---

## 关联文档

- `docs/local-workspace-ai-handoff.md`
- `docs/local-indexeddb-schema.md`
- `docs/ai-handoff-pitfalls.md`
