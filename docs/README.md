# Docs 导航说明

**Last Updated**: 2026-02-26  
**目标**: 帮助后续开发人员和 AI Agent 在最短时间内找到正确文档、理解项目现状并开始开发。

---

## 1. 先读这三份主文档（推荐）

这三份是当前统一口径，建议优先阅读：

1. `FRONTEND_DEVELOPER_GUIDE.md`  
前端技术栈、已实现能力、打包方式、常见踩坑、上手路径。

2. `BACKEND_DEVELOPER_GUIDE.md`  
后端技术栈、接口能力、编译发布、运维要点、常见踩坑。

3. `ENGINEERING_STANDARDS.md`  
前后端代码规范、代码结构、跨端契约、测试门禁、文档协作规范。

---

## 2. 按角色的阅读路径

### 前端开发者

1. `FRONTEND_DEVELOPER_GUIDE.md`
2. `ENGINEERING_STANDARDS.md`
3. `ai-handoff-pitfalls.md`
4. `HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`（涉及首页模板/SSR时）
5. `SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`（涉及阅读页 SSR 时）

### 后端开发者

1. `BACKEND_DEVELOPER_GUIDE.md`
2. `ENGINEERING_STANDARDS.md`
3. `BACKEND_IMPLEMENTATION_PHASES.md`
4. `ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`
5. `SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`

### AI Agent / 新同学快速上手

1. `FRONTEND_DEVELOPER_GUIDE.md`
2. `BACKEND_DEVELOPER_GUIDE.md`
3. `ENGINEERING_STANDARDS.md`
4. `DAILY_PROGRESS_2026-02-23.md`（最近阶段快照）

---

## 3. 按主题索引

### A. 当前主文档（持续维护）

1. `FRONTEND_DEVELOPER_GUIDE.md`
2. `BACKEND_DEVELOPER_GUIDE.md`
3. `ENGINEERING_STANDARDS.md`

### B. 后台治理与发布

1. `ADMIN_CONSOLE_IMPLEMENTATION_PHASES.md`
2. `ADMIN_CONSOLE_RELEASE_CHECKLIST.md`
3. `ADMIN_SPACE_CREATE_WITH_COVER_IMPLEMENTATION_PHASES.md`

### C. SSR 与阅读链路

1. `HOMEPAGE_SSR_IMPLEMENTATION_PHASES.md`
2. `SPACE_READER_SSR_SUBPROCESS_TECHNICAL_PROPOSAL.md`
3. `SPACE_READER_SSR_SUBPROCESS_IMPLEMENTATION_PHASES.md`

### D. 数据模型与改造说明

1. `SPACE_CATEGORY_REFACTOR_NOTES.md`
2. `BACKEND_IMPLEMENTATION_PHASES.md`

### E. 交接与踩坑记录

1. `ai-handoff-pitfalls.md`
2. `backend-ai-handoff.md`
3. `DAILY_PROGRESS_2026-02-22.md`
4. `DAILY_PROGRESS_2026-02-23.md`

### F. 文档配图素材

1. `screenshot-*.png`

---

## 4. 如何使用这些文档

1. 需要“开始开发”：先读主文档（第 1 节）。
2. 需要“理解某条链路设计”：读对应专题实施文档（第 3 节 B/C/D）。
3. 需要“排查历史问题”：读踩坑与日报（第 3 节 E）。
4. 需要“上线/发布”：优先走 `ADMIN_CONSOLE_RELEASE_CHECKLIST.md`。

---

## 5. 文档维护约定

1. 三份主文档是当前事实口径，功能变化后必须优先更新。
2. 历史阶段文档保留用于追溯，不作为唯一事实来源。
3. 新增文档时，需同步更新本导航文件对应分类。
4. 文档建议包含：
   - `Last Updated`
   - 适用范围
   - 与其他文档的关系（依赖/补充/替代）

---

## 6. 推荐最小验证命令

前端改动后：

```bash
npm run check:dropdown-menu -w @plaindoc/web
npm run web:build
```

后端改动后：

```bash
cd apps/server && go test ./... -count=1
```
