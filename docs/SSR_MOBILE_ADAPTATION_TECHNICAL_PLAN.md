# SSR 页面移动端适配技术方案（首页/分类/搜索/阅读）

**文档状态**: Implemented（已完成）  
**创建日期**: 2026-03-05  
**完成日期**: 2026-03-05  
**适用范围**: `apps/server`（首页/分类/搜索 Go 模板 SSR）、`apps/web`（阅读页 Worker SSR）

---

## 1. 目标与范围

本次只做移动端适配，不改后端业务接口与权限模型，覆盖：

1. 首页：`GET /`
2. 分类页：`GET /explore/:categoryId`
3. 搜索结果页：`GET /search`
4. 文档阅读页：`GET /r/:spaceId/:docId`

---

## 2. 实施步骤（按序执行）

### Step A：首页/分类/搜索页面移动端导航改造

- [x] A1. 在顶部导航新增移动端触发器（目录按钮、搜索按钮）。
- [x] A2. 将左侧分类导航改为移动端抽屉（desktop 保持原布局）。
- [x] A3. 增加移动端遮罩层、关闭按钮与 body 状态类（`yt-mobile-nav-open`、`yt-mobile-search-open`）。
- [x] A4. 调整网格与卡片在小屏下的间距、字号与列数。

目标文件：

1. `apps/server/internal/server/view/templates/partials/youtube_top_nav.tmpl`
2. `apps/server/internal/server/view/templates/partials/youtube_sidebar.tmpl`
3. `apps/server/internal/server/view/templates/home.tmpl`
4. `apps/server/internal/server/view/templates/explore.tmpl`
5. `apps/server/internal/server/view/templates/search.tmpl`
6. `apps/server/internal/server/view/static/home-youtube.css`
7. `apps/server/internal/server/view/static/home.js`

### Step B：阅读页移动端目录抽屉改造

- [x] B1. 在阅读页 SSR 标记中新增移动端工具条与目录开关。
- [x] B2. 侧栏目录在小屏改为抽屉，正文区优先展示。
- [x] B3. 异步脚本新增抽屉开关、遮罩关闭、路由切换后自动收起。
- [x] B4. 保持既有 `data-reader-*` 契约不破坏，仅新增 hook。

目标文件：

1. `apps/web/src/ssr/render-space-reader.tsx`
2. `apps/web/src/ssr/render-space-reader.base.css`
3. `apps/web/src/ssr/render-space-reader.async-script.ts`

### Step C：验证与收尾

- [x] C1. 后端模板页回归测试通过（首页/分类/搜索）。
- [x] C2. 前端构建通过（包含 SSR Worker）。
- [x] C3. 文档状态改为 Implemented，步骤全部勾选完成。

建议验证命令：

1. `cd apps/server && go test ./internal/server -run TestRouter_HomePages -count=1`
2. `npm run web:build`

---

## 3. 验收标准

1. 手机视口下（宽度 <= 900px）首页/分类/搜索无横向滚动，导航可展开与关闭。
2. 阅读页在宽度 <= 1024px 时正文优先显示，目录以抽屉形式打开。
3. 阅读页异步切换文档、附件操作、导出功能不回归。
4. 不破坏 canonical、slug 路由、权限分流和 SSR 降级链路。

---

## 4. 回滚策略

1. 首页链路仅涉及模板与静态资源，可回滚上述 7 个文件。
2. 阅读页链路仅涉及 `src/ssr` 三个文件，可独立回滚。
3. 不涉及数据库迁移与 API 契约变更，无数据层回滚成本。
