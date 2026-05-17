# Docs 导航（精简版）

**Last Updated**: 2026-03-08  
**目标**: 文档收敛到“前端一份 + 后端一份”，减少重复、冲突与维护成本。

---

## 1. 当前主文档（仅维护这两份）

1. `FRONTEND_DEVELOPER_GUIDE.md`  
   前端统一开发文档：路由、模块、SSR 协作、构建、测试、排障。
2. `BACKEND_DEVELOPER_GUIDE.md`  
   后端统一开发文档：架构、配置、接口、权限、SSR、迁移、发布。

后续功能变更请只更新这两份文档，不再并行新增“阶段性主文档”。

---

## 1.1 安全专项文档

1. `AUTH_SECURITY_HARDENING_PLAN.md`  
   注册/登录安全加固专项实施文档（验证码分级、失败封禁、实施清单与落地状态）。

---

## 1.2 功能专题文档（按需阅读）

1. `READER_DOCUMENT_IDENTIFIER_SEO_TECHNICAL_PLAN.md`  
   阅读页文档自定义标识（slug）方案：兼容旧 URL、canonical 统一、编辑器文档树入口、sitemap/搜索联动与迁移测试清单。
2. `ONLYOFFICE_INTEGRATION_TECHNICAL_PLAN.md`  
   ONLYOFFICE 一等文档方案：Word/Excel 作为文档节点接入，支持工作区编辑、前端阅读、分享阅读，并明确不参与搜索与 sitemap。
3. `ONLYOFFICE_READER_HTML_RENDERING_TECHNICAL_PLAN.md`  
   ONLYOFFICE 阅读页本地 HTML 渲染方案：编辑继续走 iframe，阅读/分享按配置切换 ONLYOFFICE 或本地 HTML，并复用 `documents`、`document_image_assets`、`file_blobs` 实现轻量落地。
4. `READER_IMAGE_VIEWER_TECHNICAL_PLAN.md`
   阅读页图片浏览器方案：支持 `img`、正文 inline SVG 与 Mermaid SVG 的黑色遮罩浮层浏览、切图、缩放、原始尺寸与旋转工具条。
5. `2026-05-15_SPACE_EXPORT_TECHNICAL_PLAN.md`
   空间导入导出方案：管理后台支持导出可回灌 `.plaindoc` 空间交换包，也支持 EPUB 阅读包；空间管理上方可导入 `.plaindoc` 后解析元数据、创建新空间并恢复目录、文档、附件与 Office 源文件；导入/导出任务统一进入右下角全局任务浮层，并支持登录态刷新恢复进行中任务。
6. `2026-05-15_SPACE_IMPORT_EXPORT_TASK_CHECKLIST.md`
   空间导入导出执行清单：按协议骨架、任务 SSE、zip 导出、附件与 Office source、EPUB、导入解析、导入落地、审计清理、测试文档同步、全局任务中心与刷新恢复拆分，可逐项回写进度。
7. `2026-05-17_DATABASE_STARTUP_PROGRESS_TECHNICAL_PLAN.md`
   数据库初始化与迁移中间页方案：服务先启动 Bootstrap HTTP，迁移期间展示启动进度页，迁移完成后原子切换到正式 Router。

---

## 2. 旧文档迁移映射（信息已并入两份主文档）

1. 后台治理/发布相关内容 -> 已并入 `BACKEND_DEVELOPER_GUIDE.md`。
2. 首页 SSR/阅读 SSR/子进程协议相关内容 -> 已并入 `BACKEND_DEVELOPER_GUIDE.md` + `FRONTEND_DEVELOPER_GUIDE.md`。
3. 空间分类改造、目录树拖拽、日报、交接踩坑 -> 已并入两份主文档的“能力现状/高频坑/待办”章节。
4. 工程规范与跨端契约 -> 已并入两份主文档的“工程边界/协议约束/测试门禁”章节。

---

## 3. 推荐阅读顺序

1. 新成员先读：`BACKEND_DEVELOPER_GUIDE.md`
2. 前端开发读：`FRONTEND_DEVELOPER_GUIDE.md`
3. 涉及跨端联调：两份文档都读（尤其契约与 SSR 章节）
