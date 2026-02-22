# Implementation Phases: 首页 SSR（Go Template）

**Project Type**: 现有项目新增服务端渲染首页（SEO 优先）  
**Scope**: 首页不走 `apps/web`，由 `apps/server` 使用 Go 模板直接渲染 HTML  
**Stack**: Server（Gin + GORM + html/template）  
**Primary Goal**: 搜索引擎可直接抓取首页与分类页内容；以空间封面为核心展示可见空间，并支持分类导航与筛选

---

## 零、前置依赖与关联文档

1. 首页 SSR 依赖当前空间数据模型（含分类关联）作为基础输入。  
2. 空间分类已从配置表迁移为独立表，详见：`docs/SPACE_CATEGORY_REFACTOR_NOTES.md`。  
3. 若首页后续需要按分类筛选/聚合，请直接基于 `spaces.category_id` 与 `space_categories` 扩展查询，不再读取 `system_configs` 中的分类配置。

---

## 一、目标与约束

### 1.1 业务目标
1. 首页必须由后端模板渲染（SSR），不能在 `apps/web` 内开发。
2. 首页左侧新增分类列表区域，支持点击跳转分类页。
3. 分类页顶部罗列所有一级分类，下方展示该分类筛选后的空间列表。
4. 首页与分类页均只展示“当前访问者可见”的空间列表。
5. 空间列表按发布时间倒序（最新优先）展示。
6. 默认仅展示可访问且有效状态空间（`active`）。
7. 当分类下无可见空间时，仍渲染分类页与分类导航，仅空间列表为空。

### 1.2 可见性规则（首页/分类页）
1. 匿名访问者：仅可见 `visibility=public` 空间。
2. 登录访问者：可见：
   - `public`
   - `authenticated`
   - `member` 且该用户对空间具备读取权限（owner 或成员）。
3. `banned/deleted` 空间不在首页与分类页展示。

### 1.3 发布时间字段约定
1. 首版以 `spaces.created_at` 作为发布时间（PublishedAt）。
2. 如后续需要“定时发布/草稿发布”，再新增独立发布字段与发布状态。

### 1.4 展示交互约束
1. 空间卡片以封面图为主视觉元素（优先展示 `coverUrl`）。
2. 鼠标移入卡片时显示半透明遮罩（overlay）。
3. 遮罩层展示该空间名称；鼠标移出恢复为仅封面图。
4. 上述交互应由 SSR 输出可用 HTML + CSS 实现，不依赖前端框架 hydrate。
5. 首页网格首版按每行 5 个空间展示，默认展示 4 行（即首屏 20 个空间）。

### 1.5 渲染新鲜度与缓存约束
1. 已登录用户访问首页/分类页必须实时渲染（按请求即时查询，不使用页面缓存）。
2. 未登录用户访问首页/分类页启用缓存控制（可缓存），无需每次实时渲染。
3. 缓存策略必须与登录态严格隔离，避免缓存污染导致权限泄露。

---

## 二、总体方案设计

## 2.1 路由与渲染入口
1. 新增服务端路由：
   - `GET /`（首页，左侧分类 + 可见空间）
   - `GET /explore/:categoryId`（分类页，顶部分类导航 + 分类空间列表）
2. 保持现有 `/api/*` 接口不变。
3. `NoRoute/NoMethod` 仍走 JSON 错误协议；`GET /` 与 `GET /explore/:categoryId` 由专用 handler 命中，不走 `NoRoute`。

## 2.2 分层设计
1. **Handler 层**：`homeHandler`
   - 解析分页参数（`page/pageSize`）与分类参数（`categoryId`）。
   - 解析访问者身份（匿名或用户 ID）。
   - 调用 service 获取首页/分类页数据。
   - 根据登录态写入差异化 `Cache-Control` 响应头。
   - 渲染模板并写入 HTML。
2. **Service 层**：`HomeService`
   - 统一封装首页与分类页可见性判定、分类导航与空间列表查询编排。
   - 输出模板所需 ViewModel（避免模板直接依赖 DB 模型）。
3. **Repository 层**：`SpaceRepository + SpaceCategoryRepository` 扩展查询方法
   - 查询一级分类列表（含“未分类”）。
   - 按访问者身份过滤可见空间。
   - 按分类筛选空间（分类页）。
   - 按 `created_at DESC` 排序。
   - 返回分页结果。

## 2.3 模板结构建议
1. 新增目录（建议）：
   - `apps/server/internal/server/view/templates/layout.tmpl`
   - `apps/server/internal/server/view/templates/home.tmpl`
   - `apps/server/internal/server/view/templates/category.tmpl`
   - `apps/server/internal/server/view/templates/partials/category_sidebar.tmpl`
   - `apps/server/internal/server/view/templates/partials/category_tabs.tmpl`
   - `apps/server/internal/server/view/templates/partials/space_cover_card.tmpl`
2. 视图模型建议字段：
   - `categories[]`：`categoryId`、`name`、`isDefault`、`url`、`isActive`
   - `spaces[]`：`spaceId`、`name`、`description`、`coverUrl`、`visibility`、`publishedAt`
   - `page`、`pageSize`、`total`
   - `activeCategory`（分类页时必填，首页可为空）
3. 模板默认启用转义（`html/template`），避免 XSS 注入风险。
4. 卡片遮罩交互建议：
   - 通过 `position + :hover` 实现半透明层。
   - 遮罩文案至少包含空间名称（可选附带简要描述）。

## 2.4 样式构建方案（Tailwind Standalone CLI）
1. 后端模板样式允许引入 Tailwind，并通过 Standalone CLI 在构建时生成静态 CSS。  
2. 推荐采用“输入样式文件 + 输出编译产物”模式，例如：
   - 输入：`apps/server/internal/server/view/assets/home.tailwind.css`
   - 输出：`apps/server/internal/server/view/static/home.css`
3. 通过 `//go:generate` 触发样式编译，建议在样式目录增加生成入口文件（如 `generate.go`）：
   - `//go:generate tailwindcss -i ./home.tailwind.css -o ../static/home.css --minify`
4. 服务端模板仅引用编译后的 CSS 产物，不在运行时动态编译 Tailwind。  
5. 若本机不存在 `tailwindcss` CLI，可从 Tailwind 官方发布页下载：`https://github.com/tailwindlabs/tailwindcss/releases`。

---

## 三、数据查询设计

## 3.1 仓储接口扩展（建议）
1. 在 `SpaceRepository` 增加：
   - `ListVisibleForHomepage(ctx context.Context, params ListVisibleHomepageSpacesParams) ([]VisibleHomepageSpaceRecord, int64, error)`
2. `ListVisibleHomepageSpacesParams` 新增参数：
   - `ViewerUserID string`
   - `CategoryID string`（可选；为空表示不按分类过滤）
   - `Limit int`
   - `Offset int`
3. 新增返回结构（或扩展原结构）：
   - `Space models.Space`
   - `OwnerName string`（可选）
4. 在 `SpaceCategoryRepository` 增加：
   - `List(ctx context.Context) ([]models.SpaceCategory, error)`

## 3.2 SQL 过滤语义
1. 公共过滤条件：
   - `s.status = 'active'`
   - `s.deleted_at IS NULL`
2. 匿名访问：
   - `s.visibility = 'public'`
3. 登录访问：
   - `s.visibility IN ('public','authenticated')`
   - 或 `s.visibility='member'` 且用户为 owner/member
4. 排序：
   - `ORDER BY s.created_at DESC, s.id DESC`
5. 分类页增加过滤条件（当 `CategoryID` 非空）：
   - `s.category_id = :categoryId`
6. 分类列表查询：
   - `SELECT category_id, name, is_default FROM space_categories ORDER BY is_default DESC, name ASC`
   - 其中 `is_default=1`（未分类）固定在第一位，其余按名称升序。
7. 分页：
   - 首版使用 `LIMIT/OFFSET`，后续可升级 keyset。
   - 首页默认 `pageSize=20`（对应 5x4 网格）。

## 3.3 索引建议
1. `spaces(category_id, status, visibility, created_at DESC)` 复合索引。
2. `space_members(user_id, space_id)` 复合索引（若未覆盖则补充）。
3. `space_categories(is_default, name)` 复合索引（用于导航排序与检索）。

---

## 四、登录态识别策略（首页/分类页 SSR）

## 4.1 首版策略
1. 首页与分类页优先支持匿名抓取（SEO 主场景）。
2. 若请求头带 `Authorization: Bearer <token>`，可识别登录用户并返回扩展可见空间。

## 4.2 浏览器登录态增强（建议后续）
1. 增加 `HttpOnly` Cookie 方案（登录/刷新同步设置，退出清理）。
2. `homeHandler` 与 `categoryHandler` 优先从 Cookie 读取 access token，再回退 Authorization。
3. 这样浏览器直接打开 `/` 与 `/explore/:categoryId` 时可稳定识别 member 空间权限。

## 4.3 缓存控制策略（首页/分类页）
1. **已登录请求（实时渲染）**：
   - `Cache-Control: private, no-store, max-age=0`
   - 不允许 CDN/浏览器缓存页面主体。
2. **未登录请求（可缓存）**：
   - 匿名缓存策略从系统配置读取（推荐配置项：`homepage.ssr.anonymous_cache`）。
   - 默认值：`Cache-Control: public, max-age=60, s-maxage=300, stale-while-revalidate=60`。
   - 允许浏览器与 CDN 缓存，降低 SSR 压力。
3. **缓存隔离要求**：
   - 响应增加 `Vary: Authorization, Cookie`，确保匿名与登录态缓存分离。
   - 若后续增加区域化或语言化差异，再补充对应 `Vary` 维度。

---

## 五、SEO 与页面规范

## 5.1 页面 SEO 基线（首页 + 分类页）
1. `title`、`meta description`、`canonical`（分类页 canonical 包含 `categoryId`）。
2. 语义化结构：`header/main/section/article/footer`。
3. 列表内容为真实 HTML，不依赖客户端 JS 才能看到。

## 5.2 结构化数据（建议）
1. 首页与分类页均输出 `JSON-LD ItemList`。
2. 项目字段可含名称、URL、发布时间、摘要。

## 5.3 扩展项（可选）
1. `GET /sitemap.xml`（仅 public 空间）。
2. `GET /robots.txt`。

---

## 六、里程碑计划

## Milestone 1：首页/分类页数据查询能力
**Type**: Repository + Service  
**Estimated**: 1~2 天

**Tasks**:
1. 扩展 `SpaceRepository` 可见空间查询接口（支持可选分类过滤）。
2. 实现可见性过滤（匿名/登录/member）。
3. 扩展 `SpaceCategoryRepository` 分类列表查询。
4. 按 `created_at DESC` 排序并返回分页。
5. 补充仓储与服务层测试。

**Exit Criteria**:
1. 服务层可返回首页数据（分类导航 + 可见空间列表）。
2. 服务层可返回分类页数据（顶部分类 + 当前分类空间列表）。
3. 排序与权限矩阵测试通过。

---

## Milestone 2：首页/分类页 Handler + Go 模板渲染
**Type**: Handler + View  
**Estimated**: 1~2 天

**Tasks**:
1. 新增 `GET /` 与 `GET /explore/:categoryId` 路由及 handler。
2. 接入模板加载与渲染（layout + home + category + partials）。
3. 首页实现左侧分类列表区域。
4. 分类页实现顶部分类导航 + 下方空间列表。
5. 增加分页参数解析与模板分页导航。
6. 落地 Tailwind 样式输入文件与模板静态 CSS 引用。
7. 按登录态写入差异化缓存响应头（登录 no-store，匿名可缓存）。

**Exit Criteria**:
1. `GET /` 可返回完整 HTML，左侧分类可点击跳转分类页。
2. `GET /explore/:categoryId` 可返回完整 HTML，且按分类筛选空间。
3. 首页与分类页均仅展示访问者可见空间。
4. 登录/匿名请求的缓存头策略符合约定。

---

## Milestone 3：封面卡片交互与样式落地
**Type**: UI + CSS（SSR Template）  
**Estimated**: 1 天

**Tasks**:
1. 空间列表采用封面卡片网格布局。
2. 卡片 hover 时显示半透明遮罩。
3. 遮罩层展示空间名称（可选摘要）。
4. 适配无封面场景（占位图与降级样式）。
5. 增加 `//go:generate` 样式编译命令并在开发流程中验证产物可用。
6. 首页卡片网格按 5 列 * 4 行（20 项）实现首屏布局。

**Exit Criteria**:
1. 空间封面成为主视觉展示元素。
2. hover 交互稳定，无遮挡错位与文字不可读问题。
3. 无需前端 hydrate 即可完成交互展示。
4. 首屏默认展示 20 个空间（5x4）。

---

## Milestone 4：SEO 基础与可抓取优化
**Type**: SEO + Delivery  
**Estimated**: 1 天

**Tasks**:
1. 完成首页与分类页 meta 标签与 canonical。
2. 增加首页与分类页 JSON-LD `ItemList`。
3. 补充页面标题、摘要与空状态文案。

**Exit Criteria**:
1. 搜索引擎抓取工具可直接读取首页/分类页空间列表内容。
2. HTML 输出不依赖前端 hydrate 才可见。

---

## Milestone 5：登录态增强（可选）
**Type**: Auth Integration  
**Estimated**: 1~2 天

**Tasks**:
1. 评估并接入 `HttpOnly` Cookie 登录态同步。
2. `homeHandler` 与 `categoryHandler` 支持 Cookie 识别用户身份。
3. 回归登录/刷新/登出对首页/分类页可见性的影响。

**Exit Criteria**:
1. 浏览器直接访问 `/` 与 `/explore/:categoryId` 可识别登录身份。
2. member 空间按权限正确显示/隐藏。

---

## 七、测试与验收

### 7.1 后端测试
1. 仓储测试：匿名、登录、member 权限矩阵；分类过滤正确性。
2. Handler 测试：
   - `GET /` 返回 200 + `text/html` + 分类侧栏与空间卡片断言。
   - `GET /explore/:categoryId` 返回 200 + `text/html` + 顶部分类导航与筛选结果断言。
3. 排序测试：`created_at` 新空间优先。
4. 分类一致性测试：默认“未分类”始终可访问，非法分类返回预期行为（空状态或跳转，按实现约定）。
5. 空列表测试：当分类下无可见空间时返回 200 并渲染空列表态。
6. 缓存头测试：
   - 匿名请求命中可缓存响应头（`public/max-age/s-maxage`）。
   - 登录请求命中 `no-store` 响应头，且不复用匿名缓存。

### 7.2 验收清单
1. 首页左侧可见分类列表，点击可跳转对应分类页。
2. 分类页路由使用 `/explore/:categoryId`。
3. 分类页顶部罗列所有一级分类，排序为“未分类优先，其余名称升序”，当前分类高亮。
4. 分类页下方仅显示该分类下“当前访问者可见”的空间。
5. 当分类下无可见空间时，页面正常渲染且列表为空。
6. 首页默认 5 列 * 4 行展示空间（20 项）。
7. 首页与分类页均满足匿名/登录/member 可见性规则。
8. 空间卡片以封面为主，hover 显示半透明遮罩与空间名称。
9. 页面源代码可直接看到分类导航与空间卡片内容（非 JS 注入）。
10. 登录访问始终实时渲染；匿名访问生效缓存控制。

---

## 八、风险与注意事项

1. 当前系统主要基于 Authorization 头，浏览器直访首页/分类页默认可能只走匿名权限；需通过 Cookie 增强解决。  
2. 模板中输出描述字段必须保持默认 HTML 转义，禁止手动 `template.HTML` 注入。  
3. 首页与分类页查询可能随数据增长变重，需通过索引与分页控制首屏成本。  
4. 分类数量较多时，分类页顶部导航需支持横向滚动或换行，避免布局溢出。  
5. 分类删除后空间会迁移到“未分类”，首页与分类页查询需确保对该默认分类兼容。
6. 若环境缺少 `tailwindcss` CLI，`go generate` 会失败；需先安装 CLI 或从官方 release 下载可执行文件。
7. 若未正确设置 `Vary`，可能导致匿名缓存污染登录页内容，属于高风险项。
8. 匿名缓存策略为系统配置项，需保证配置缺失时回退默认值（`max-age=60/s-maxage=300`）并可观测。

---

## 九、已确认决策

1. 分类页路由固定为：`GET /explore/:categoryId`。  
2. 分类排序固定为：未分类第一位，其余按名称升序。  
3. 分类下无可见空间时仍渲染页面，仅空间列表为空。  
4. 首页首版固定为每行 5 个、共 4 行（默认 `pageSize=20`）。  
5. 匿名缓存策略支持系统配置，默认采用 `max-age=60/s-maxage=300`（并保留 `stale-while-revalidate=60`）。  
