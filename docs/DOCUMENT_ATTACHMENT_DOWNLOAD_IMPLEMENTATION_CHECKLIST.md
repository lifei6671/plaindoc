# 文档附件下载鉴权实现任务清单（本地）

> 更新时间：2026-03-01
> 说明：以下状态基于当前本地代码实现与可编译结果。

## 范围

本清单覆盖以下能力：
- 文档附件的数据结构与接口
- 下载时基于文档权限拦截
- 非公开场景鉴权下载链接
- 非本地存储下载链接生成扩展
- 预览扩展点预留（PDF/Office 等）

## 任务清单

- [x] 附件物理文件实体表 `file_blobs`（按 `storage_provider + hash + size` 去重）
- [x] 附件表增加 `blob_id / content_hash_algo / content_hash` 字段
- [x] 上传链路增加 SHA-256 计算与秒传（命中哈希仅新增引用，不重复上传）
- [x] 物理删除改为“先删附件记录，再按引用判断是否删 `file_blobs` 与物理文件”
- [x] 后台物理删除增加共享引用提醒（可二次确认后继续仅删当前引用）

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

- [x] 非本地私有存储的“真实预签名 URL”生成（R2/OSS SDK 签名）
- [x] 非本地下载链接策略配置化（公开桶直链 vs 私有桶签名）
- [x] 文档阅读页底部附件渲染（按文档读取并展示）
- [x] PDF/Office 在线预览页（消费 `purpose=preview`）
- [x] 后台附件管理页面（检索/删除/审计）
- [ ] 删除文档时附件物理清理策略（批量与失败补偿）
- [ ] 附件链路自动化测试补齐（handler/service/e2e）
- [ ] 对外 API 文档补齐（请求参数、错误码、时效约束）

## 备注（当前实现边界）

- 当前“非本地下载链接能力”已支持：
  - `downloadStrategy=public`：优先使用 `objectUrl`，否则按 `publicBaseURL + objectKey` 组装；
  - `downloadStrategy=signed`：按 provider 使用 SDK 生成临时签名 URL（R2/OSS）。
- 新增图床配置字段：
  - `cloudflareR2.downloadStrategy` / `aliyunOss.downloadStrategy`：`public | signed`
  - `cloudflareR2.signedUrlTtlSeconds` / `aliyunOss.signedUrlTtlSeconds`：默认 `86400`（24h）
- 在线预览页当前行为：
  - `image/pdf/text`：直接内嵌预览；
  - `office`：公开附件走 Office Web Viewer，需鉴权附件提示下载查看；
  - 预留 `window.__PLAINDOC_ATTACHMENT_PREVIEW_RENDERERS__.office` 扩展点，可接入自建 Office/PDF 预览服务。
