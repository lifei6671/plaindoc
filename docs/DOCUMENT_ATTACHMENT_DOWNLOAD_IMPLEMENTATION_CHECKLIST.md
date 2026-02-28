# 文档附件下载鉴权实现任务清单（本地）

> 更新时间：2026-02-28
> 说明：以下状态基于当前本地代码实现与可编译结果。

## 范围

本清单覆盖以下能力：
- 文档附件的数据结构与接口
- 下载时基于文档权限拦截
- 非公开场景鉴权下载链接
- 非本地存储下载链接生成扩展
- 预览扩展点预留（PDF/Office 等）

## 任务清单

- [x] 新增附件数据模型 `document_attachments`（含 `preview_kind` 扩展字段）
- [x] 新增 MySQL/PostgreSQL/SQLite 迁移脚本（`0020_document_attachments`）
- [x] 新增附件仓储接口与 GORM 实现（创建/查询/软删除）
- [x] 注入路由与依赖（workspace handler + repository + token service）
- [x] 新增附件 API：列表/上传/删除/访问链接/签名链接下载
- [x] 附件访问链接 token 服务完成（默认有效期 24 小时）
- [x] 下载前按文档可见性校验（public/authenticated/member）
- [x] 完全公开文档支持直链下载（不拦截）
- [x] 非公开文档走鉴权下载链路（后端校验 + 前端带认证请求）
- [x] `purpose=download|preview` 语义落地，非法值返回明确错误
- [x] 本地存储附件下载与预览分流（`Content-Disposition: attachment/inline`）
- [x] 前端新增附件 Popover（上传、刷新、下载、预览、删除）
- [x] 删除附件前提供确认，支持“仅逻辑删除/物理删除”
- [x] 前端对附件链接做绝对 URL 归一化（避免跨域/基路径问题）
- [x] 构建验证通过：`go test ./...`、`npm run build`

- [ ] 非本地私有存储的“真实预签名 URL”生成（R2/OSS SDK 签名）
- [ ] 非本地下载链接策略配置化（公开桶直链 vs 私有桶签名）
- [ ] 文档阅读页底部附件渲染（按文档读取并展示）
- [ ] PDF/Office 在线预览页（消费 `purpose=preview`）
- [ ] 后台附件管理页面（检索/删除/审计）
- [ ] 删除文档时附件物理清理策略（批量与失败补偿）
- [ ] 附件链路自动化测试补齐（handler/service/e2e）
- [ ] 对外 API 文档补齐（请求参数、错误码、时效约束）

## 备注（当前实现边界）

- 当前“非本地下载链接能力”已支持：
  - 使用 `object_url` 直接跳转；
  - 或按 provider 的 `publicBaseURL + objectKey` 组装公开链接。
- 但“私有桶临时签名 URL”尚未落地，因此该项仍标记为未完成。

