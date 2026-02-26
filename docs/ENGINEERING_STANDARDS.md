# 工程规范（前端 + 后端 + 协作）

**Last Updated**: 2026-02-26  
**适用对象**: 所有开发成员与 AI Agent  
**目标**: 统一代码结构、开发方式和交付质量，避免跨模块返工与隐性回归

---

## 1. 规范适用范围

本规范适用于：

1. 前端 `apps/web`
2. 后端 `apps/server`
3. 跨端 API 契约
4. 文档与发布协作流程

优先级：本规范 > 历史实现文档 > 临时口头约定。

---

## 2. 代码结构规范

### 2.1 前端结构（`apps/web/src`）

1. `App.tsx`：应用总入口、路由分流、全局状态编排。
2. `data-access/`：唯一后端通信层，禁止页面直连 `fetch`。
3. `editor/`：编辑、预览、渲染链路与同步滚动核心。
4. `admin/`：后台页面、菜单、模块实现。
5. `components/`：可复用组件。
6. `ssr/`：阅读页 SSR Worker 相关代码。

### 2.2 后端结构（`apps/server/internal`）

1. `server/handler`：请求入参校验与响应映射。
2. `service/`：业务规则、权限判断、审计落点。
3. `storage/repository`：数据访问实现。
4. `server/router.go`：唯一依赖装配与路由注册入口。
5. `ssr/`：Worker 协议、进程、池化与调度。
6. `config/`：配置定义、解析、校验。

---

## 3. 前端代码规范

1. TypeScript 维持 `strict: true`，禁止新增 `any` 绕过类型系统。
2. API 调用必须经过 `data-access` 网关，不在页面层散落请求代码。
3. 禁止使用 `window.confirm` / `window.prompt`，确认交互统一用项目 Dialog 组件。
4. `DropdownMenu` 必须保持 `modal=false`，提交前执行：
   - `npm run check:dropdown-menu -w @plaindoc/web`
5. 涉及预览区改动时，必须保护以下稳定契约：
   - `#plaindoc-preview-pane`
   - `.plaindoc-preview-pane`
   - `.plaindoc-preview-body`
6. 涉及阅读页交互改动时，脚本应依赖 `data-reader-*` hook，不依赖视觉 class。
7. 修改 `rehype-sanitize` 配置后，必须验证：
   - 锚点属性保留
   - XSS 拦截
   - 数学公式渲染
8. 涉及预览样式改动时，必须同步检查微信导出样式映射（`editor/wechat-export.ts`）。
9. 复杂逻辑或分支必须加简洁中文注释，说明目的与边界，而不是解释语法。

---

## 4. 后端代码规范

1. 遵循 `handler -> service -> repository`，禁止在 handler 写复杂 SQL。
2. 路由与依赖注入集中在 `router.go`，避免多入口分散注入。
3. 统一响应使用 `JsonResult`（`code/message/requestId/data`）。
4. 业务语义优先通过 `JsonResult.code` 表达；前端按 `code` 判定状态。
5. 高风险后台写操作必须接入 operation token 校验中间件。
6. 管理员请求上下文必须保留审计字段注入（actor、request_id）。
7. 配置读取通过 `config` 模块，避免业务代码直接读环境变量。
8. 涉及时间字段查询时避免无差别 `SELECT *` 扫描，尤其 SQLite 场景。
9. Go 代码必须可通过 `gofmt`（默认 Go 工具链格式）并保持可读注释。

---

## 5. 跨端契约规范

1. 前后端契约来源：
   - 前端：`apps/web/src/data-access/types.ts`
   - 后端：`apps/server/internal/server/response/*` 与 handler DTO
2. 修改字段前必须同时更新：
   - 后端响应结构
   - 前端类型定义
   - 前端 adapter 归一化逻辑
3. 兼容性策略：
   - 新增字段优先向后兼容
   - 破坏性变更需同步文档并给出迁移说明

---

## 6. 安全与权限规范

1. 角色边界必须在前后端双重校验：
   - 前端做入口可见性控制
   - 后端做强制权限校验
2. `space_admin` 与 `platform_admin` 权限边界不可通过前端绕过。
3. 用户封禁/删除需联动会话吊销与审计记录。
4. 生产环境必须替换默认密钥并审计关键配置（`JWT_SECRET`、`WEB_ORIGIN` 等）。

---

## 7. 测试与验收规范

每次改动至少满足：

1. 前端：
   - `npm run check:dropdown-menu -w @plaindoc/web`
   - `npm run web:build`
2. 后端：
   - `cd apps/server && go test ./... -count=1`
3. 涉及后台权限或高风险操作：
   - 回归 `TestRouter_Admin` 相关测试
4. 涉及迁移：
   - 回归 `internal/storage` 迁移测试
5. 涉及阅读页：
   - 手工验证 `/r/:spaceId/:docId` 渲染与降级链路

---

## 8. 文档与发布规范

1. 需求落地后，同步更新对应指南文档，不允许“代码已变、文档未变”。
2. 发布前按后台发布清单执行：
   - `docs/ADMIN_CONSOLE_RELEASE_CHECKLIST.md`
3. 关键变更必须记录：
   - 变更范围
   - 迁移影响
   - 回滚路径
4. 历史方案文档保留，但以本规范和三份指南作为当前主口径。

---

## 9. AI Agent 上手流程（建议）

1. 先读：
   - `docs/FRONTEND_DEVELOPER_GUIDE.md`
   - `docs/BACKEND_DEVELOPER_GUIDE.md`
   - `docs/ENGINEERING_STANDARDS.md`
2. 再读对应专项历史文档（SSR、后台、分类、日报）。
3. 修改代码前先定位契约文件与测试入口。
4. 提交前执行规范中的最低命令集并补充变更说明。
