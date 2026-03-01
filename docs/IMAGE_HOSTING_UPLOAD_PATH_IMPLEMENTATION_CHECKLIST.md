# 图床路径模板与后端文件名生成任务清单（本地）

> 更新时间：2026-03-01  
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

- [ ] 附件上传链路按同样模板规则统一改造（当前附件仍走 `buildDocumentAttachmentObjectKey`，未接入 `uploadPathTemplate`）
- [ ] 附件上传支持所有 provider（当前附件上传仍以本地链路为主）
- [ ] 图床模板“内置场景模板选择器”（按文档归档/按时间归档/按内容寻址）UI 仍未做，仅提供文本输入
- [ ] 模板变量可视化选择器（点选拼装路径）未做
- [ ] 文档图片管理表（用于追踪文档引用图片）未落地
- [ ] 保存文档时图片引用解析与“待清理”标记未落地
- [ ] 定时清理幽灵图片任务未落地

## 验证结果

- 后端：`go test ./...` 通过（`apps/server`）
- 前端：本地 `Node/V8` 环境存在编译器崩溃（`unreachable code`）历史问题；本次改动不涉及该问题根因

## 备注

- 默认模板：`images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}`
- 当前后台已可直接配置模板，但模板合法性由后端统一校验；非法模板保存会被拦截。
