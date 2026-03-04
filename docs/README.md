# Docs 导航（精简版）

**Last Updated**: 2026-03-04  
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
