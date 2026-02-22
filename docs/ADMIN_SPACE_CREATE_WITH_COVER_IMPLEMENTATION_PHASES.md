# Implementation Phases: 后台空间新建（含封面上传与系统生成）

**Project Type**: 现有管理后台增强（Space Management）  
**Scope**: 新建空间模态窗 + 封面处理链路（用户上传/系统生成） + 前端预处理 + 后端二次校验与统一绘制  
**Stack**: Web（React + Vite） + Server（Gin + GORM）  
**Target**: 支持在后台创建空间时完整填写封面、名称、简介、可见性，并保证封面统一规格（5:8，WebP）

---

## 一、目标与约束

### 1.1 业务目标
1. 在后台“空间管理”页面增加“新建空间”能力。  
2. 新建空间通过模态窗完成：空间名称、空间简介、可见性、封面。  
3. 封面支持两种来源：  
   - 用户上传（前端实现裁剪 + Canvas 转码 WebP）  
   - 系统生成（后端基于空间名称自动绘制）
4. 后端不信任前端上传结果：必须二次校验格式、比例、尺寸，不符合规范时自动处理为标准封面。

### 1.2 图像约束（统一）
1. 目标比例：`1600:2560 = 5:8`。  
2. 输出格式：`image/webp`。  
3. 输出尺寸策略：优先 `1600x2560`；输入不足时按等比例缩放，不强制放大。  
4. 前端用户上传必须支持按 `5:8` 裁剪框交互并导出 WebP。  
5. 后端接收后必须再次校验并自动规范化，确保最终落库始终满足 `5:8 + WebP`。

---

## 二、接口设计（前后端契约）

## 2.1 新增接口 A：上传空间封面素材

**Endpoint**: `POST /api/admin/spaces/cover-assets`  
**Auth**: Admin（`platform_admin` 与 `space_admin`）  
**Content-Type**: `multipart/form-data`

### Request 字段
1. `file`：图片文件（前端裁剪/转码后的 WebP Blob；`source=user_upload` 时必填）。  
2. `source`：`"user_upload"` | `"system_generated"`。  
3. （可选）前端处理元信息（用于审计与排障）：  
   - `clientWidth`, `clientHeight`  
   - `clientMimeType`  
   - `clientProcessed`（是否经过前端裁剪/转码）
4. `spaceName`：空间名称（`source=system_generated` 时必填；用于后端生成大标题封面）。

### Server 行为
1. `source=user_upload`：校验 MIME/文件大小/像素上限。  
2. `source=user_upload`：解码并校验是否满足 `5:8 + WebP + 尺寸约束`，不满足则自动裁剪/缩放/转码。  
3. `source=system_generated`：根据 `spaceName` 在后端统一生成简约风封面（仅大标题）。  
4. 两种来源均输出统一规格 `1600x2560`、`image/webp`，并落盘/对象存储（推荐路径：`uploads/space-covers/yyyy/mm/dd/...webp`）。

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
  "normalized": true,
  "source": "user_upload"
}
```

---

## 2.2 新增接口 B：后台新建空间

**Endpoint**: `POST /api/admin/spaces`  
**Auth**: Admin（`platform_admin` 与 `space_admin`）  
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
   - `系统生成`（固定简约风 + 使用空间名称作为大标题 + 后端生成预览）

## 3.3 上传封面流程（用户上传）
1. 选择文件（`jpg/png/webp`）。  
2. 打开裁剪器，固定比例 `5:8`，完成交互裁剪。  
3. 前端将裁剪结果绘制到 Canvas（目标 `1600x2560` 或等比例缩放），导出 WebP Blob。  
4. 上传 WebP Blob 到 `POST /api/admin/spaces/cover-assets`。  
5. 预览返回封面并绑定 `coverAssetId`。

## 3.4 系统生成流程（前端触发，后端绘制）
1. 固定使用简约风样式生成封面（不提供模板切换）。  
2. 自动读取“空间名称”作为封面唯一文案，仅渲染大标题（无副标题）。  
3. 调用 `POST /api/admin/spaces/cover-assets`，携带 `source=system_generated` 与 `spaceName`。  
4. 后端生成封面并返回 `coverAssetId`，前端完成预览与绑定。

---

## 四、后端实现要点

1. **数据模型建议**（空间表扩展）：
   - `description`（TEXT）
   - `cover_key`（VARCHAR）
   - `cover_url`（VARCHAR）
   - `cover_width`、`cover_height`（INT）
   - `cover_source`（ENUM: user_upload/system_generated）
2. **图片处理 pipeline**：
   - 读取上传结果 → 规范校验（比例/格式/尺寸）→ 不合规自动裁剪/缩放/转码 → 存储。  
   - 系统生成场景：后端直接渲染 → WebP 编码 → 存储。  
   - 说明：前端已处理只是优化体验与带宽，后端仍以最终规范为准。  
3. **后端绘制栈（系统生成）**：
   - 使用 `github.com/fogleman/gg` 作为主绘图上下文。  
   - 使用 `golang.org/x/image/font` 处理文本绘制。  
   - 使用 `golang.org/x/image/font/opentype` 加载 TTF/OTF 中文字体。  
   - 使用 `golang.org/x/image/math/fixed` 做字形精确定位与度量。  
   - 使用 `golang.org/x/image/draw` 做高质量缩放插值。  
4. **文本与布局规则（系统生成）**：
   - 仅渲染大标题，文本来源为空间名称。  
   - 采用安全边距（建议四周 ≥ 128px），标题块不得贴边。  
   - 标题支持中文换行：按可用宽度逐字测量换行，避免英文单词被截断。  
   - 限制最大行数（建议 2~3 行）与最小字号；超出时尾行省略号处理。  
   - 防止溢出：任何字形包围盒不得越出画布。  
   - 保持统一美观：固定对齐方式、行高、字重、配色与留白比例。  
5. **安全与限制**：
   - 限制最大文件大小、最大像素、允许格式。  
   - 防止伪造 MIME、异常图像炸弹、超大分辨率输入。  
   - 上传接口仅管理员可用（`platform_admin` / `space_admin`）。

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

## Milestone 2：封面上传与后端二次校验兜底
**Type**: API + Media Processing  
**Estimated**: 2~3 天

**Tasks**:
1. 新增 `POST /api/admin/spaces/cover-assets`。  
2. 实现上传校验与规范检查（比例、尺寸、格式）。  
3. 对不合规输入自动执行裁剪、缩放、WebP 转码并落盘/对象存储。  
4. 返回统一资产响应（assetId/key/url/width/height/normalized）。

**Exit Criteria**:
1. 绕过前端直接上传非 WebP/非 5:8 图片时，后端可自动规范化。  
2. 超限/非法输入场景有明确错误码，且不会污染存储。

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

## Milestone 4：封面上传裁剪与前端 WebP 转码
**Type**: UI + Media  
**Estimated**: 2 天

**Tasks**:
1. 模态窗封面区加入 `上传封面` Tab。  
2. 接入裁剪器（固定 5:8）并支持拖拽缩放裁剪框。  
3. 前端 Canvas 导出 WebP（质量参数可配置）。  
4. 上传处理后的 Blob，绑定 `coverAssetId` 到创建空间请求。

**Exit Criteria**:
1. 上传 + 裁剪 + 创建空间链路打通。  
2. 前端上传文件体积较原图显著下降（目标压缩率可观测）。  
3. 生成空间后列表可看到封面缩略图（若开启展示）。

---

## Milestone 5：系统生成封面（后端 `gg` 绘制）
**Type**: UI + Media  
**Estimated**: 2~3 天

**Tasks**:
1. 封面区加入 `系统生成` Tab。  
2. 后端接入 `gg + x/image` 渲染链路，按空间名称生成大标题封面。  
3. 实现中文换行、字形边界检查、统一安全边距与美观规则。  
4. 输出 WebP 并上传到 `cover-assets` 资产存储。

**Exit Criteria**:
1. 无需本地图片即可生成封面并创建空间。  
2. 长中文标题、多语言混排标题均无裁切/贴边问题。  
3. 生成封面满足统一风格、5:8 与 WebP 要求。

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

## 六、需要你确认的点（剩余决策）

1. **简介长度**：建议上限 280，是否确认？  
2. **封面是否必填**：首期建议选填。  
3. **上传规格**：最大文件大小（建议 10MB）与允许格式（建议 jpg/png/webp）是否确认？  
4. **中文字体文件**：生产环境使用哪套字体（如 `Noto Sans SC`）及其授权是否已确认？

> 以上确认后，可按 Milestone 1 开始落地实现。
