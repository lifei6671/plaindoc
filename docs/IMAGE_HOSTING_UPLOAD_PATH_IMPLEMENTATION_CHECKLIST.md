# 图床路径模板与后端文件名生成任务清单（本地）

> 更新时间：2026-03-01（第四次）  
> 说明：以下状态基于当前本地代码与可执行验证结果。

## 范围

本清单覆盖你最近提出的“图床/图片上传”相关任务：

- 所有存储 provider 的文件名（对象 key）由后端生成
- 图床配置支持可配置上传路径模板（含约束校验）
- 后台系统配置页提供配置入口
- 迁移脚本补齐
- 同步核对延伸需求（附件同规则、幽灵图片治理）当前是否已落地

## 已完成

- [x] 新增后端对象 key 分配接口：`POST /api/uploads/images/object-key`
- [x] 上传链路改为后端生成对象 key（不再由前端拼文件名）
- [x] `UploadImage` 已支持 `local / cloudflare-r2 / aliyun-oss` 三类 provider 直传
- [x] 图片对象 key 生成支持模板变量替换（`spaceId/docId/date/assetId/origName/ext/uploaderId`）
- [x] 上传路径模板增加后端强校验：必须包含 `{assetId}`、禁止 `..`、禁止反斜杠、限制前缀等
- [x] 图床配置结构新增 `uploadPathTemplate`（`local/cloudflareR2/aliyunOss`）
- [x] 后台系统配置 -> 图床设置 已新增 3 个 provider 的“上传路径模板”输入项
- [x] 远程图片本地化（`localizeRemoteImages`）已复用同一图片 key 生成逻辑
- [x] MySQL/PostgreSQL/SQLite 迁移已新增 `0021_image_hosting_upload_path_templates`（up/down）
- [x] 后端测试通过：`go test ./...`（`apps/server`）

## 未完成 / 待开发

- [x] 附件上传链路按同样模板规则统一改造（已接入 `uploadPathTemplate` 与变量替换）
- [x] 附件上传支持所有 provider（`local/cloudflare-r2/aliyun-oss`）
- [x] 图床模板“内置场景模板选择器”已落地（按文档归档/按年月日/按年月）
- [x] 模板变量可视化选择器已落地（点选插入变量到模板输入框）
- [x] 文档图片管理表已落地（`document_image_assets` + `0022_document_image_assets` 迁移）
- [x] 保存文档时图片引用解析与“待清理”标记已落地（`SaveDocument` 调用 `SyncDocumentImageAssets`）
- [x] 定时清理幽灵图片任务已落地（数据保留清理任务接入 `CleanupPendingDocumentImageAssets`）
- [x] 附件远端 provider 的物理删除链路已落地（workspace/admin 均支持 `local/cloudflare-r2/aliyun-oss`）

## 验证结果

- 后端：`go test ./...` 通过（`apps/server`）
- 前端：`npm exec -w @plaindoc/web -- tsc -b` 通过

## 备注

- 默认模板：`images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}`
- 当前后台已可直接配置模板，但模板合法性由后端统一校验；非法模板保存会被拦截。
- 文档图片治理策略：保存文档时同步引用；取消引用后先标记 `pending_cleanup`；超过宽限期且无活跃引用再执行物理删除。
