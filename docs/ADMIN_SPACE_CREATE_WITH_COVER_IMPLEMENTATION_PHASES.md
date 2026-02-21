# Implementation Phases: 后台空间新建（含封面上传与系统生成）

**Project Type**: 现有管理后台增强（Space Management）  
**Scope**: 新建空间模态窗 + 封面处理链路（用户上传/系统生成） + 后端图片处理与存储  
**Stack**: Web（React + Vite） + Server（Gin + GORM）  
**Target**: 支持在后台创建空间时完整填写封面、名称、简介、可见性，并保证封面统一规格（5:8，WebP）

---

## 一、目标与约束

### 1.1 业务目标
1. 在后台“空间管理”页面增加“新建空间”能力。  
2. 新建空间通过模态窗完成：空间名称、空间简介、可见性、封面。  
3. 封面支持两种来源：  
   - 用户上传（后端裁剪/压缩/转码 WebP）  
   - 系统生成（前端 SVG → Canvas → WebP 后上传）

### 1.2 图像约束（统一）
1. 目标比例：`1600:2560 = 5:8`。  
2. 输出格式：`image/webp`。  
3. 输出尺寸策略：优先 `1600x2560`，若输入不足可按等比例缩放并限制最大边，不强制放大。  
4. 用户上传必须支持按 `5:8` 裁剪框交互。

---

## 二、接口设计（前后端契约）

## 2.1 新增接口 A：上传空间封面素材

**Endpoint**: `POST /api/admin/spaces/cover-assets`  
**Auth**: Admin（建议首期 `platform_admin`）  
**Content-Type**: `multipart/form-data`

### Request 字段
1. `file`：图片文件（上传或前端生成后的 WebP Blob）。  
2. `source`：`"user_upload"` | `"system_generated"`。  
3. （可选）裁剪参数（当 `source=user_upload` 时建议传入）：  
   - `cropX`, `cropY`, `cropWidth`, `cropHeight`（基于原图像素）
   - `originalWidth`, `originalHeight`

### Server 行为
1. 校验 MIME/文件大小/像素上限。  
2. `user_upload`：按裁剪参数裁切为 5:8，再压缩转码为 WebP。  
3. `system_generated`：若已是 WebP 则做统一规格检查与必要重采样。  
4. 落盘或对象存储（推荐路径：`uploads/space-covers/yyyy/mm/dd/...webp`）。

### Response（示例）
```json
{
  "assetId": "cover_01H...",
  "key": "space-covers/2026/02/21/xxx.webp",
  "url": "/api/uploads/space-covers/2026/02/21/xxx.webp",
  "width": 1600,
  "height": 2560,
  "mimeType": "image/webp",
  "sizeBytes": 182304,
  "source": "user_upload"
}
```

---

## 2.2 新增接口 B：后台新建空间

**Endpoint**: `POST /api/admin/spaces`  
**Auth**: Admin（建议首期 `platform_admin`）  
**Content-Type**: `application/json`

### Request（示例）
```json
{
  "name": "设计协作空间",
  "description": "用于品牌设计与文档协作",
  "visibility": "member",
  "coverAssetId": "cover_01H..."
}
```

### Response（示例）
```json
{
  "spaceId": "sp_01H...",
  "name": "设计协作空间",
  "description": "用于品牌设计与文档协作",
  "visibility": "member",
  "cover": {
    "key": "space-covers/2026/02/21/xxx.webp",
    "url": "/api/uploads/space-covers/2026/02/21/xxx.webp",
    "width": 1600,
    "height": 2560
  },
  "ownerUserId": "u_01H...",
  "createdAt": "2026-02-21T12:00:00Z",
  "updatedAt": "2026-02-21T12:00:00Z"
}
```

---

## 2.3 现有接口扩展
1. `GET /api/admin/spaces`：返回 `description`、`coverUrl`（列表可用于缩略展示）。  
2. `PATCH /api/admin/spaces/:spaceId/metadata`：可选支持 `description` 更新（封面更新可单独接口，首期不强依赖）。

---

## 三、前端交互设计（Admin 空间管理）

## 3.1 页面入口
1. 在 `AdminSpacesPage` 工具栏新增“新建空间”按钮。  
2. 点击打开模态窗（Dialog）。

## 3.2 新建空间模态窗结构
1. 基础信息区：
   - 空间名称（必填）
   - 空间简介（选填，建议 280 字内）
   - 可见性（public/authenticated/member）
2. 封面区（横向 Tabs）：
   - `上传封面`（用户上传 + 裁剪）
   - `系统生成`（模板参数 + SVG 预览）

## 3.3 上传封面流程（用户上传）
1. 选择文件（`jpg/png/webp`）。  
2. 打开裁剪器，固定比例 `5:8`。  
3. 提交裁剪参数给 `POST /api/admin/spaces/cover-assets`。  
4. 预览返回封面并绑定 `coverAssetId`。

## 3.4 系统生成流程（前端生成）
1. 选择模板（渐变/几何/简约等）。  
2. 输入标题、副标题、主色、装饰参数。  
3. 前端生成 SVG，绘制到 Canvas（目标尺寸 `1600x2560`），导出 WebP Blob。  
4. 调用 `POST /api/admin/spaces/cover-assets` 上传生成结果，获得 `coverAssetId`。

---

## 四、后端实现要点

1. **数据模型建议**（空间表扩展）：
   - `description`（TEXT）
   - `cover_key`（VARCHAR）
   - `cover_url`（VARCHAR）
   - `cover_width`、`cover_height`（INT）
   - `cover_source`（ENUM: user_upload/system_generated）
2. **图片处理 pipeline**：
   - 解码 → 裁剪（可选）→ 缩放 → WebP 转码（质量建议 75~85）→ 存储。  
3. **安全与限制**：
   - 限制最大文件大小、最大像素、允许格式。  
   - 校验裁剪参数边界。  
   - 上传接口仅管理员可用。

---

## 五、里程碑拆解

## Milestone 1：接口与数据模型基线
**Type**: API + Migration  
**Estimated**: 1~2 天

**Tasks**:
1. 设计并落地 `spaces` 扩展字段 migration。  
2. 新增 `POST /api/admin/spaces` handler/service/repository。  
3. 扩展 `AdminSpace` DTO（description/cover）。

**Exit Criteria**:
1. 后台可通过 API 创建无封面空间。  
2. 空间列表可返回 description/cover 结构（为空亦可）。

---

## Milestone 2：封面上传与后端 WebP 处理
**Type**: API + Media Processing  
**Estimated**: 2~3 天

**Tasks**:
1. 新增 `POST /api/admin/spaces/cover-assets`。  
2. 实现上传校验、裁剪、缩放、WebP 转码、落盘/对象存储。  
3. 返回统一资产响应（assetId/key/url/width/height）。

**Exit Criteria**:
1. 用户上传图片可成功裁剪为 5:8 并转码 WebP。  
2. 失败场景（超限/非法参数）有明确错误码。

---

## Milestone 3：后台新建空间模态窗（基础版）
**Type**: UI + Integration  
**Estimated**: 2 天

**Tasks**:
1. 在空间管理页增加“新建空间”按钮与 Dialog。  
2. 完成名称、简介、可见性表单与校验。  
3. 接入 `POST /api/admin/spaces` 并刷新列表。

**Exit Criteria**:
1. 可通过模态窗创建空间并立即在列表可见。  
2. 表单错误提示与提交态完整。

---

## Milestone 4：封面上传裁剪（前端）
**Type**: UI + Media  
**Estimated**: 2 天

**Tasks**:
1. 模态窗封面区加入 `上传封面` Tab。  
2. 接入裁剪器（固定 5:8），提交裁剪参数。  
3. 绑定 `coverAssetId` 到创建空间请求。

**Exit Criteria**:
1. 上传 + 裁剪 + 创建空间链路打通。  
2. 生成空间后列表可看到封面缩略图（若开启展示）。

---

## Milestone 5：系统生成封面（SVG→Canvas→WebP）
**Type**: UI + Media  
**Estimated**: 2~3 天

**Tasks**:
1. 封面区加入 `系统生成` Tab。  
2. 实现模板化 SVG 生成与参数化。  
3. Canvas 导出 WebP Blob 后上传到 `cover-assets`。

**Exit Criteria**:
1. 无需本地图片即可生成封面并创建空间。  
2. 导出封面满足 5:8 与 WebP 要求。

---

## Milestone 6：回归测试与发布收口
**Type**: QA + Release  
**Estimated**: 1~2 天

**Tasks**:
1. 覆盖接口与 UI 关键路径测试（创建成功、失败、权限、图片异常）。  
2. 增加文档与运维项（存储清理、日志、监控指标）。  
3. 发布检查清单（回滚策略 + 验收脚本）。

**Exit Criteria**:
1. 全链路稳定，无阻塞缺陷。  
2. 发布材料完备，可灰度上线。

---

## 六、需要你确认的点（关键决策）

1. **创建权限**：仅 `platform_admin` 还是 `space_admin` 也可创建？  
2. **简介长度**：建议上限 280，是否确认？  
3. **封面是否必填**：首期建议选填。  
4. **上传规格**：最大文件大小（建议 10MB）与允许格式（建议 jpg/png/webp）是否确认？  
5. **系统生成模板**：首期模板数量（建议 4~6 套）是否确认？

> 以上确认后，可按 Milestone 1 开始落地实现。
