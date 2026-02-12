# Implementation Phases: 本地工作区（空间 + 目录树）

**Project Type**: 现有 Web 前端功能扩展（Local Workspace MVP）  
**Scope**: 在 `localAdapter` 下支持“本地空间 + 目录树 + 文档切换 + 基础 CRUD + ULID 业务 ID 策略”  
**Stack**: React + Vite + TypeScript + 现有 DataGateway 抽象  
**Estimated Total**: 17 小时（约 1020 分钟人工时间）

---

## 范围与边界

**本次目标（MVP）**：
- 左侧工作区侧边栏（空间切换 + 目录树）
- 本地空间新建与切换
- 目录/文档树形展示（展开/收起）
- 文档节点打开与编辑区联动
- 本地目录树基础 CRUD（新建、重命名、删除）
- 本地状态持久化（最近空间/文档、展开状态）
- 业务 ID 统一使用小写 ULID（非自增）

**本次不做**：
- 拖拽排序（Drag & Drop）
- 多人协作与权限 UI
- 跨标签页实时同步
- 完整回收站/撤销删除流程

---

## Phase 1: 工作区状态层落地
**Type**: UI  
**Estimated**: 3 小时（约 180 分钟）  
**Files**:
- `apps/web/src/workspace/types.ts`（新增）
- `apps/web/src/workspace/use-workspace.ts`（新增）
- `apps/web/src/App.tsx`
- `apps/web/src/editor/status-utils.ts`
- `apps/web/src/data-access/types.ts`（扩展点声明）

**Tasks**:
- [ ] 新增 `workspace` 领域状态类型：当前空间、当前文档、目录树、加载状态、错误状态。
- [ ] 将 `App.tsx` 内“空间/文档启动加载逻辑”抽离到 `useWorkspace`，避免 `App` 继续膨胀。
- [ ] 在 Hook 中封装 `bootstrapWorkspace`、`openDocument`、`reloadTree` 等动作。
- [ ] 保持现有保存链路（自动保存/冲突处理）不变，只替换文档来源与切换入口。
- [ ] 预留扩展点：在工作区动作层保留 `moveNode`（拖拽排序）与 `deleteSpace`（空间删除）能力位，当前版本可返回 `NotImplemented`。
- [ ] 新增/调整中文注释，说明状态边界与关键流程。

**Verification Criteria**:
- [ ] 首次启动时，若无空间则自动创建默认空间与默认文档。
- [ ] 已有空间场景下可稳定加载首个文档并进入可编辑状态。
- [ ] 文档切换后标题、版本号、保存状态正确更新。
- [ ] 扩展点能力位不影响现有流程（即使未启用也不报错）。
- [ ] `npm run web:build` 通过。

**Exit Criteria**: `App` 不再直接管理工作区初始化细节，工作区状态读写统一由 `useWorkspace` 提供，且不引入保存能力回归。

---

## Phase 2: 空间侧边栏骨架
**Type**: UI  
**Estimated**: 2.5 小时（约 150 分钟）  
**Files**:
- `apps/web/src/components/WorkspaceSidebar.tsx`（新增）
- `apps/web/src/components/SpaceSwitcher.tsx`（新增）
- `apps/web/src/App.tsx`
- `apps/web/src/styles.css`

**Tasks**:
- [ ] 在主工作区中增加左侧栏布局（侧栏 + 编辑区 + 预览区）。
- [ ] 实现空间切换器（空间列表、当前空间高亮、创建空间入口）。
- [ ] 侧边栏微交互状态放在子组件内部，避免触发 `App` 根组件重渲染。
- [ ] 保持预览区关键选择器不变：`#plaindoc-preview-pane`、`.plaindoc-preview-pane`、`.plaindoc-preview-body`。
- [ ] 增加必要中文注释，说明性能边界和状态归属。

**Verification Criteria**:
- [ ] 可创建新空间并自动切换到新空间。
- [ ] 空间切换后，当前文档和状态栏“文档位置”正确更新。
- [ ] 展开/收起空间菜单时，编辑输入无明显卡顿。
- [ ] 预览滚动同步与主题切换不受布局改动影响。

**Exit Criteria**: 侧边栏结构稳定、空间切换可用，且未破坏现有编辑/预览核心链路。

---

## Phase 3: 目录树渲染与文档切换
**Type**: UI  
**Estimated**: 3 小时（约 180 分钟）  
**Files**:
- `apps/web/src/components/WorkspaceTree.tsx`（新增）
- `apps/web/src/components/WorkspaceTreeNode.tsx`（新增）
- `apps/web/src/components/WorkspaceSidebar.tsx`
- `apps/web/src/workspace/use-workspace.ts`
- `apps/web/src/styles.css`

**Tasks**:
- [ ] 递归渲染目录树（`folder/doc` 图标、层级缩进、选中态）。
- [ ] 支持目录展开/收起与当前文档高亮。
- [ ] 点击文档节点触发 `openDocument`，更新编辑区内容与版本基线。
- [ ] 为空空间提供“新建首篇文档”引导态。
- [ ] 关键渲染/递归逻辑补充中文注释。

**Verification Criteria**:
- [ ] 3 层以上目录结构可正确渲染与展开。
- [ ] 文档切换后内容、标题、版本、最后保存时间同步更新。
- [ ] 切换文档后继续编辑，自动保存仍正常。
- [ ] 长文档 + 长图场景下双向滚动同步无明显漂移。

**Exit Criteria**: 目录树浏览与文档切换形成完整闭环，可支持日常多文档编辑。

---

## Phase 4: 目录树本地 CRUD 闭环
**Type**: API  
**Estimated**: 4.5 小时（约 270 分钟）  
**Files**:
- `apps/web/src/workspace/use-workspace.ts`
- `apps/web/src/components/WorkspaceTreeNode.tsx`
- `apps/web/src/data-access/local/adapter.ts`
- `apps/web/src/data-access/local/store.ts`
- `apps/web/src/data-access/local/ulid.ts`（新增）
- `apps/web/src/data-access/types.ts`（如需扩展输入类型）
- `apps/web/src/data-access/http/adapter.ts`（接口对齐）

**Tasks**:
- [ ] 支持在任意目录下新建子目录/子文档。
- [ ] 支持节点重命名（目录与文档）。
- [ ] 支持节点删除（目录递归删除、文档删除）。
- [ ] 删除当前激活文档时，自动选择下一个可用文档；无文档时自动创建占位文档。
- [ ] 将本地 ID 生成切换为小写 ULID（业务主键），不使用自增 ID。
- [ ] ULID 去重策略：生成后先校验当前内存/存储是否已存在，冲突则重试（建议最多 8 次），仍冲突则抛错。
- [ ] 统一 ID 生成入口，避免空间/节点/文档/修订各自散落实现。
- [ ] 为本地适配器补充必要校验（不存在节点、非法父节点等）并返回可读错误。
- [ ] 复杂分支补充中文注释，避免后续误改。

**Verification Criteria**:
- [ ] 新建目录/文档后，树与编辑区实时更新。
- [ ] 重命名文档后，状态栏标题与文档标题同步。
- [ ] 删除目录时，其子树被正确移除且无孤儿节点。
- [ ] 删除激活文档后不会出现空白崩溃状态。
- [ ] 所有新建实体 ID 均为小写 ULID，且无重复。
- [ ] `npm run web:build` 通过。

**Exit Criteria**: 目录树本地 CRUD 可用且数据一致性可控，用户不会被操作流程卡死。

---

## Phase 5: 本地持久化与性能回归
**Type**: Integration  
**Estimated**: 2.5 小时（约 150 分钟）  
**Files**:
- `apps/web/src/workspace/use-workspace.ts`
- `apps/web/src/components/WorkspaceSidebar.tsx`
- `apps/web/src/data-access/user-config/indexeddb-gateway.ts`（如需小幅增强）
- `apps/web/src/App.tsx`
- `apps/web/src/styles.css`

**Tasks**:
- [ ] 持久化 `activeSpaceId`、`activeDocId`、`expandedFolderIds`（优先复用 `userConfigGateway`）。
- [ ] 启动时恢复上述状态，若目标不存在则自动回退到最近可用文档。
- [ ] 复查组件重渲染边界，确保侧栏交互不拖慢编辑区输入。
- [ ] 保持主题菜单与样式抽屉性能隔离策略不被破坏。
- [ ] 增补中文注释，记录持久化键与恢复回退策略。

**Verification Criteria**:
- [ ] 刷新页面后保留上次空间、文档和目录展开状态。
- [ ] 删除上次激活文档后再次刷新，能正确回退到可用文档。
- [ ] 主题切换 + 外部样式覆盖后滚动映射仍正确重建。
- [ ] 编辑区连续输入 30 秒无明显卡顿。

**Exit Criteria**: 工作区状态可恢复、体验稳定，核心性能与同步滚动链路无回归。

---

## Phase 6: 验收清单与交付门槛
**Type**: Testing  
**Estimated**: 1.5 小时（约 90 分钟）  
**Files**:
- `docs/LOCAL_WORKSPACE_ACCEPTANCE.md`（新增）

**Tasks**:
- [ ] 编写本地工作区手工回归清单（空间、目录树、文档切换、删除边界、刷新恢复）。
- [ ] 增加 ULID 验收项（小写格式、唯一性、冲突重试）。
- [ ] 固化最小构建验收命令：`npm run web:build`。
- [ ] 记录已知限制与后续迭代项（拖拽排序、跨标签页同步等）。

**Verification Criteria**:
- [ ] 构建命令通过。
- [ ] 验收清单全部通过并留存结果。
- [ ] 无阻断级问题（无法打开文档、删除后崩溃、滚动同步失效等）。
- [ ] 扩展点确认：`moveNode` / `deleteSpace` 的接口位置与调用链清晰可扩展。

**Exit Criteria**: 本地工作区 MVP 达到“可日常使用”标准，具备进入后续后端接入阶段的质量基线。

---

## Notes

**Testing Strategy**: 以阶段内手工验收 + 最终回归清单为主（当前仓库未接入前端自动化测试框架）。  
**Deployment Strategy**: 每个 Phase 结束执行本地构建，阶段 3/4/5 结束后进行一次完整手工回归。  
**Context Management**: 每个 Phase 控制在 4-6 个文件、2-4 小时，确保单次会话可完整收敛。

---

## 已确认策略（本轮评审）

1. 文档删除后兜底：自动选择同层下一个文档，若不存在则自动创建“未命名文档”。
2. 本期不做拖拽排序，但必须预留扩展点（动作层与数据层入口）。
3. 本期不做空间删除，但必须预留扩展点（空间操作位与 UI 承载位）。
4. 允许目录/文档重名（即允许重命名为已存在名称）。
5. 业务 ID 策略：统一使用小写 ULID，禁止使用自增 ID 作为业务主键。
6. 未来数据库策略：数据库可保留自增 `id`（技术主键），但业务关联与外部引用使用 `ulid` 字段。

---

## 数据模型前瞻（数据库接入阶段）

- 每张核心表建议双主键语义：
- `id BIGINT AUTO_INCREMENT`：仅内部技术用途（排序/排障），业务逻辑不依赖。
- `ulid CHAR(26)`：业务主标识，`UNIQUE NOT NULL`，所有接口和关联优先使用。
- 外键建议优先关联 `ulid`（或维护 `*_ulid` 字段），避免未来从本地到后端迁移时 ID 语义变化。
