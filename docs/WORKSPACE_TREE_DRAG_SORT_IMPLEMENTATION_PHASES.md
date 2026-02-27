# Implementation Phases: 编辑区目录树拖拽排序（Desktop）

**Last Updated**: 2026-02-27（执行完成）  
**Scope**: `apps/web` 编辑区右侧目录树拖拽排序（不含移动端手势） + `apps/server` 节点移动与重排  
**Primary Goal**: 最终支持跨父级拖拽，并保证排序稳定、无循环父子、失败可回滚

---

## 一、已确认范围（来自产品决策）

1. 首期先支持同级拖拽排序（同父节点内重排）。  
2. 最终目标支持跨父级拖拽。  
3. 不允许把节点拖拽到自己的子节点（含任意层级后代）下。  
4. 允许拖拽到其他节点的子节点（包含文档节点与目录节点）。  
5. 暂不支持移动端拖拽，仅桌面端启用。

---

## 二、里程碑与优先级

## Milestone 0：协议与边界冻结（P0）
**Status**: Completed  
**Type**: Design  
**Estimated**: 0.5 天

**Tasks**:
- [x] 明确移动接口参数：`targetParentId` + `targetIndex`（建议新增专用 move API）。
- [x] 明确跨父级排序语义：移出旧父级后，旧/新父级都执行连续重排（`sort` 从 1 递增）。
- [x] 明确前端落点语义：插入前/后与“成为子节点”映射为统一 `targetParentId + targetIndex`。

**Exit Criteria**:
- [x] 文档与代码注释对齐，避免前后端对 `sort` 语义理解不一致。

---

## Milestone 1：后端移动能力（同级 + 跨父级）基础（P1）
**Status**: Completed  
**Type**: API + Repository  
**Estimated**: 1~2 天

**Tasks**:
- [x] 新增接口：`POST /api/nodes/:nodeId/move`（或等价路由）。
- [x] 服务端参数校验：
  - `nodeId` 非空
  - `targetParentId` 允许 `null`
  - `targetIndex >= 0`
- [x] 权限校验：复用现有空间写权限逻辑（owner/collaborator/admin 可写，reader 不可写）。
- [x] 新增仓储事务方法：原子执行“节点移动 + 旧父级重排 + 新父级重排 + 更新时间触发”。

**Exit Criteria**:
- [x] 后端可独立完成同级与跨父级移动，树查询结果顺序稳定。

---

## Milestone 2：后端循环父子防护与错误语义（P2）
**Status**: Completed  
**Type**: API Hardening  
**Estimated**: 0.5~1 天

**Tasks**:
- [x] 增加循环检测：禁止将节点挂到自己或任意后代节点下。
- [x] 约束同空间：目标父节点必须与当前节点同 `space_id`。
- [x] 增加明确错误响应模板（如 `node move cycle detected`）。

**Exit Criteria**:
- [x] 任意非法移动请求返回可读错误，不污染数据。

---

## Milestone 3：后端测试收口（P3）
**Status**: Completed  
**Type**: Testing  
**Estimated**: 1 天

**Tasks**:
- [x] Handler 测试：同级移动成功。
- [x] Handler 测试：跨父级移动成功。
- [x] Handler 测试：拖到自己后代节点返回 400。
- [x] Handler 测试：无权限用户返回 403。
- [x] 排序连续性测试：移动后 `sort` 连续且无重复（通过 handler + DB 断言覆盖）。

**Exit Criteria**:
- [x] 关键移动路径具备自动化保障。

---

## Milestone 4：前端数据层接入 moveNode（P4）
**Status**: Completed  
**Type**: Data Access  
**Estimated**: 0.5 天

**Tasks**:
- [x] `apps/web/src/data-access/types.ts` 将 `moveNode` 从可选能力收敛为正式能力。
- [x] `apps/web/src/data-access/http/adapter.ts` 实现 `workspace.moveNode`，调用新 move API。
- [x] `apps/web/src/workspace/use-workspace.ts` 复用 `moveNode`，失败透传错误并刷新树。

**Exit Criteria**:
- [x] 目录树组件可通过统一网关触发移动，无分叉调用路径。

---

## Milestone 5：前端首期上线（同级拖拽）(P5)
**Status**: Completed  
**Type**: UI  
**Estimated**: 1 天

**Tasks**:
- [x] `WorkspaceTree` 桌面端开启拖拽（首期同级可用）。
- [x] 拖拽失败提示（服务端落库后刷新树，避免前端树状态漂移）。
- [x] 保持现有交互不回归：重命名、删除、创建、展开折叠、打开文档。

**Exit Criteria**:
- [x] 同级拖拽可稳定使用，且不影响原有目录操作体验。

---

## Milestone 6：前端最终目标（跨父级拖拽）(P6)
**Status**: Completed  
**Type**: UI + Integration  
**Estimated**: 1 天

**Tasks**:
- [x] 支持拖拽到其他节点作为子节点。
- [x] 支持跨父级拖拽到同级插入位。
- [x] 前端先做轻量防御：拖到自己后代时直接阻止；后端仍作为最终兜底。

**Exit Criteria**:
- [x] 跨父级拖拽链路可用，并与后端防循环规则一致。

---

## Milestone 7：桌面限定与回归（P7）
**Status**: Completed  
**Type**: QA  
**Estimated**: 0.5 天

**Tasks**:
- [x] 仅在桌面端启用拖拽（触屏与移动端禁用）。
- [x] 回归验证：
  - 拖拽后刷新页面顺序保持一致
  - 删除/新建后继续可拖拽
  - 自动保存与文档切换不受拖拽影响

**Exit Criteria**:
- [x] 桌面稳定可用，移动端无异常触发点。

---

## 三、执行顺序（必须遵守）

1. Milestone 0（协议冻结）
2. Milestone 1（后端移动基础）
3. Milestone 2（后端防循环）
4. Milestone 3（后端测试）
5. Milestone 4（前端数据层）
6. Milestone 5（前端同级拖拽首期）
7. Milestone 6（前端跨父级最终目标）
8. Milestone 7（桌面限定与回归）

---

## 四、Definition of Done

1. 桌面端目录树支持同级与跨父级拖拽。  
2. 服务端严格拦截循环父子移动，数据不破坏。  
3. 拖拽成功后排序稳定且可持久化（刷新后不丢失）。  
4. 失败场景有明确提示，前端能回滚到一致状态。  
5. 关键路径具备自动化测试覆盖。  
