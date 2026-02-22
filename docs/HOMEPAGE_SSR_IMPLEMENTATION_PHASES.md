# Implementation Phases: 首页 SSR（Go Template）

**Project Type**: 现有项目新增服务端渲染首页（SEO 优先）  
**Scope**: 首页不走 `apps/web`，由 `apps/server` 使用 Go 模板直接渲染 HTML  
**Stack**: Server（Gin + GORM + html/template）  
**Primary Goal**: 搜索引擎可直接抓取首页内容；按空间发布时间倒序展示当前访问者有权限访问的空间

---

## 一、目标与约束

### 1.1 业务目标
1. 首页必须由后端模板渲染（SSR），不能在 `apps/web` 内开发。
2. 首页展示“当前访问者可见”的空间列表。
3. 列表按发布时间倒序（最新优先）展示。
4. 默认仅展示可访问且有效状态空间（`active`）。

### 1.2 可见性规则（首页）
1. 匿名访问者：仅可见 `visibility=public` 空间。
2. 登录访问者：可见：
   - `public`
   - `authenticated`
   - `member` 且该用户对空间具备读取权限（owner 或成员）。
3. `banned/deleted` 空间不在首页展示。

### 1.3 发布时间字段约定
1. 首版以 `spaces.created_at` 作为发布时间（PublishedAt）。
2. 如后续需要“定时发布/草稿发布”，再新增独立发布字段与发布状态。

---

## 二、总体方案设计

## 2.1 路由与渲染入口
1. 新增服务端路由：`GET /`（返回 HTML）。
2. 保持现有 `/api/*` 接口不变。
3. `NoRoute/NoMethod` 仍走 JSON 错误协议；`GET /` 由专用 handler 命中，不走 `NoRoute`。

## 2.2 分层设计
1. **Handler 层**：`homeHandler`
   - 解析分页参数（`page/pageSize`）。
   - 解析访问者身份（匿名或用户 ID）。
   - 调用 service 获取首页数据。
   - 渲染模板并写入 HTML。
2. **Service 层**：`HomeService`
   - 统一封装首页可见性判定与列表查询编排。
   - 输出模板所需 ViewModel（避免模板直接依赖 DB 模型）。
3. **Repository 层**：`SpaceRepository` 扩展首页查询方法
   - 按访问者身份过滤可见空间。
   - 按 `created_at DESC` 排序。
   - 返回分页结果。

## 2.3 模板结构建议
1. 新增目录（建议）：
   - `apps/server/internal/server/view/templates/layout.tmpl`
   - `apps/server/internal/server/view/templates/home.tmpl`
   - `apps/server/internal/server/view/templates/partials/space_card.tmpl`
2. 视图模型建议字段：
   - `spaceId`、`name`、`description`
   - `coverUrl`、`visibility`
   - `publishedAt`（由 `created_at` 映射）
   - `ownerName`（可选）
3. 模板默认启用转义（`html/template`），避免 XSS 注入风险。

---

## 三、数据查询设计

## 3.1 仓储接口扩展（建议）
1. 在 `SpaceRepository` 增加：
   - `ListVisibleForHomepage(ctx context.Context, params ListVisibleHomepageSpacesParams) ([]VisibleHomepageSpaceRecord, int64, error)`
2. 新增参数结构：
   - `ViewerUserID string`
   - `Limit int`
   - `Offset int`
3. 新增返回结构：
   - `Space models.Space`
   - `OwnerName string`（可选）

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
5. 分页：
   - 首版使用 `LIMIT/OFFSET`，后续可升级 keyset。

## 3.3 索引建议
1. `spaces(status, visibility, created_at DESC)` 复合索引。
2. `space_members(user_id, space_id)` 复合索引（若未覆盖则补充）。

---

## 四、登录态识别策略（首页 SSR）

## 4.1 首版策略
1. 首页优先支持匿名抓取（SEO 主场景）。
2. 若请求头带 `Authorization: Bearer <token>`，可识别登录用户并返回扩展可见空间。

## 4.2 浏览器登录态增强（建议后续）
1. 增加 `HttpOnly` Cookie 方案（登录/刷新同步设置，退出清理）。
2. `homeHandler` 优先从 Cookie 读取 access token，再回退 Authorization。
3. 这样浏览器直接打开 `/` 时可稳定识别 member 空间权限。

---

## 五、SEO 与页面规范

## 5.1 页面 SEO 基线
1. `title`、`meta description`、`canonical`。
2. 语义化结构：`header/main/section/article/footer`。
3. 列表内容为真实 HTML，不依赖客户端 JS 才能看到。

## 5.2 结构化数据（建议）
1. 输出 `JSON-LD ItemList`。
2. 项目字段可含名称、URL、发布时间、摘要。

## 5.3 扩展项（可选）
1. `GET /sitemap.xml`（仅 public 空间）。
2. `GET /robots.txt`。

---

## 六、里程碑计划

## Milestone 1：首页数据查询能力
**Type**: Repository + Service  
**Estimated**: 1~2 天

**Tasks**:
1. 扩展 `SpaceRepository` 首页可见空间查询接口。
2. 实现可见性过滤（匿名/登录/member）。
3. 按 `created_at DESC` 排序并返回分页。
4. 补充仓储与服务层测试。

**Exit Criteria**:
1. 服务层可返回“当前访问者可见空间列表”。
2. 排序与权限矩阵测试通过。

---

## Milestone 2：首页 Handler + Go 模板渲染
**Type**: Handler + View  
**Estimated**: 1~2 天

**Tasks**:
1. 新增 `GET /` 路由与 `homeHandler`。
2. 接入模板加载与渲染（layout + home + card）。
3. 增加分页参数解析与模板分页导航。
4. 增加错误回退页（渲染失败时最小可用 HTML）。

**Exit Criteria**:
1. `GET /` 可返回完整 HTML 且可直接浏览。
2. 首页列表按发布时间倒序展示。

---

## Milestone 3：SEO 基础与可抓取优化
**Type**: SEO + Delivery  
**Estimated**: 1 天

**Tasks**:
1. 完成基础 meta 标签与 canonical。
2. 增加 JSON-LD `ItemList`。
3. 补充页面标题、摘要与空状态文案。

**Exit Criteria**:
1. 搜索引擎抓取工具可直接读取空间列表内容。
2. HTML 输出不依赖前端 hydrate 才可见。

---

## Milestone 4：登录态增强（可选）
**Type**: Auth Integration  
**Estimated**: 1~2 天

**Tasks**:
1. 评估并接入 `HttpOnly` Cookie 登录态同步。
2. `homeHandler` 支持 Cookie 识别用户身份。
3. 回归登录/刷新/登出对首页可见性的影响。

**Exit Criteria**:
1. 浏览器直接访问 `/` 可识别登录身份。
2. member 空间按权限正确显示/隐藏。

---

## 七、测试与验收

### 7.1 后端测试
1. 仓储测试：匿名、登录、member 权限矩阵。
2. Handler 测试：`GET /` 返回 200 + `text/html` + 关键内容断言。
3. 排序测试：`created_at` 新空间优先。

### 7.2 验收清单
1. 匿名访问：首页仅出现 public 空间。
2. 登录访问：出现 authenticated 与可读 member 空间。
3. banned/deleted 空间不出现。
4. 页面源代码可直接看到空间卡片内容（非 JS 注入）。

---

## 八、风险与注意事项

1. 当前系统主要基于 Authorization 头，浏览器直访首页默认可能只走匿名权限；需通过 Cookie 增强解决。  
2. 模板中输出描述字段必须保持默认 HTML 转义，禁止手动 `template.HTML` 注入。  
3. 首页查询可能随数据增长变重，需通过索引与分页控制首屏成本。  

---

## 九、待确认项

1. 发布时间是否确认使用 `spaces.created_at`（首版默认是）。  
2. 首版是否同步落地 `HttpOnly` Cookie 登录态（影响 member 空间在 SSR 首页的可见性）。  
3. 首页分页默认值（建议 `pageSize=20`）。  

