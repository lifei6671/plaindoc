# 文档附件 API 对外说明

> 更新时间：2026-03-01  
> 返回格式：所有接口使用统一 JSON Envelope：`{ code, message, requestId, data }`，`code=0` 表示成功。

## 1. 鉴权与下载策略

- 文档可见性为 `public`：可生成直链（无需登录拦截）。
- 文档可见性为 `authenticated/member`：必须先调用访问链接接口，后端校验文档权限后发放链接。
- 非本地存储（R2/OSS）：
  - `downloadStrategy=public`：返回公开对象 URL。
  - `downloadStrategy=signed`：返回预签名 URL，默认有效期 `86400s`（24 小时）。
- 附件访问 Token（`/api/attachment-downloads/:token`）默认有效期 24 小时。

## 2. 工作区附件接口

### 2.1 获取文档附件列表

- `GET /api/docs/:docId/attachments`
- 权限：文档可读即可。
- 返回：附件列表（不含已逻辑删除记录）。

### 2.2 上传附件

- `POST /api/docs/:docId/attachments`
- `Content-Type: multipart/form-data`
- 表单字段：
  - `file`：附件文件（必填）。
- 权限：空间可写（owner/collaborator）。
- 行为：
  - 计算 SHA-256，命中相同 `storage_provider + hash + size` 时仅新增引用，不重复上传文件实体。

### 2.3 删除附件

- `DELETE /api/docs/:docId/attachments/:attachmentId?physicalDelete={true|false}`
- 权限：空间可写。
- 参数：
  - `physicalDelete=false`（默认）：逻辑删除（`status=deleted`）。
  - `physicalDelete=true`：硬删除当前引用；若文件实体无引用则尝试删除物理文件。
- 补偿策略：
  - 物理删除失败时保留 `file_blobs` 记录，后续批次自动重试清理。

### 2.4 生成附件访问链接

- `POST /api/docs/:docId/attachments/:attachmentId/access-link?purpose={download|preview}`
- 权限：文档可读（按 visibility 校验）。
- 参数：
  - `purpose=download`：下载用途。
  - `purpose=preview`：在线预览用途（仅支持 `image/pdf/office/text`）。
- 非法 `purpose`：返回 `INVALID_REQUEST`。
- 返回：
  - `url`：可访问链接（直链/签名链接/token 链接）。
  - `expiresAt`：过期时间（若有）。
  - `previewKind`：预览类型。
  - `requiresAuth`：是否需要鉴权。

### 2.5 通过 Token 下载或预览

- `GET /api/attachment-downloads/:token`
- Token 内含：`attachmentId/documentId/purpose/exp`。
- 过期或非法 Token：返回无效访问错误。
- 本地存储自动按 `purpose` 输出：
  - `download` => `Content-Disposition: attachment`
  - `preview` => `Content-Disposition: inline`

### 2.6 附件预览页

- `GET /preview/docs/:docId/attachments/:attachmentId`
- 页面会调用 `access-link?purpose=preview` 获取链接并渲染。
- Office 文件在需鉴权场景下提示下载查看（防止第三方预览服务无法携带鉴权）。

## 3. 后台附件管理接口

### 3.1 后台列表

- `GET /api/admin/document-attachments`
- 权限：
  - `platform_admin`：全量可见。
  - `space_admin`：仅自己授权空间。
- 支持关键字、状态、空间、文档、存储类型等过滤。

### 3.2 后台删除

- `DELETE /api/admin/document-attachments/:attachmentId?physicalDelete={true|false}&forcePhysicalDeleteOnShare={true|false}`
- `physicalDelete=false`：逻辑删除。
- `physicalDelete=true`：
  - 若文件被多篇文档引用，默认返回 `confirmationRequired=true`；
  - `forcePhysicalDeleteOnShare=true` 后仅删除当前引用，不会误删共享文件实体。

## 4. 自动清理（删除文档后的补偿）

- 数据清理模块新增 `document_attachments` 清理项。
- 每批执行：
  1. 清理已删除文档残留的附件引用；
  2. 扫描孤儿 `file_blobs` 并尝试物理删除；
  3. 删除失败的 blob 保留记录，等待下次补偿重试。

## 5. 常见错误码（附件链路）

- `INVALID_REQUEST (2001)`：请求参数非法（例如 `purpose` 非法）。
- `INVALID_DOCUMENT_ID (2012)`：文档 ID 缺失/非法。
- `INVALID_OPERATION (2021)`：操作语义非法。
- `DOCUMENT_NOT_FOUND (3005)`：文档不存在。
- `NODE_NOT_FOUND (3013)`：节点不存在。
- `FORBIDDEN (1003)`：无空间写权限或无文档访问权限。
- `INTERNAL_ERROR (9002)`：服务内部错误（IO/存储 SDK/数据库异常等）。
