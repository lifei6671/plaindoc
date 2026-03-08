# ONLYOFFICE 阅读页本地 HTML 渲染技术方案

**文档状态**: Draft  
**创建日期**: 2026-03-08  
**适用范围**: `apps/server`、`apps/web`、`docs`  
**目标**: 保持 ONLYOFFICE 工作区编辑继续使用 iframe 挂载，同时将前端阅读页与分享阅读页切换为本地 HTML 渲染；方案采用精简实现，复用 `documents`、`document_image_assets`、`file_blobs`，不引入独立快照表。

---

## 1. 方案结论

采用精简版方案：

1. 编辑页继续走 ONLYOFFICE iframe
2. 阅读页按配置切换：
   - ONLYOFFICE 只读模式
   - 本地 HTML 渲染
3. Office 文档转换后的 HTML 直接写入 `documents.content_md`
4. Office 文档正文里的图片继续复用 `document_image_assets`
5. 图片物理对象与 hash 去重复用 `file_blobs`
6. Office HTML 去标签后参与全文搜索

本方案不做：

1. 历史快照
2. 快照表
3. 独立图片对象表
4. 历史版本回看

---

## 2. 为什么这样做

当前阅读页依赖 ONLYOFFICE iframe，存在几个问题：

1. 首屏依赖第三方脚本和 iframe，稳定性与加载体验一般
2. 前端无法直接控制正文 HTML，做统一阅读体验受限
3. 后续想接入目录、主题、打印、站内增强能力，链路不够直接

而当前系统已经具备可以复用的基础：

1. `documents.format` 可区分 Markdown 与 Office 文档
2. `documents.content_md` 可承载当前正文内容
3. `document_image_assets` 已能跟踪正文里的图片引用
4. `file_blobs` 已有 hash 字段和去重能力

因此没必要先上复杂快照模型，先把“当前阅读态”跑通即可。

---

## 3. 核心设计

### 3.1 `content_md` 的语义

按 `documents.format` 解释 `content_md`：

1. `format=markdown`
   `content_md` 存 Markdown 正文
2. `format=docx/xlsx`
   `content_md` 存当前转换后的 HTML 正文

前端渲染规则：

1. Markdown 文档继续按 Markdown 渲染
2. Office 文档在启用独立渲染时按 HTML 渲染
3. 若 HTML 为空，则显示“生成中 / 失败 / 回退 ONLYOFFICE”

### 3.2 最小字段扩展

为避免前端无法判断当前渲染状态，建议只补最少字段：

1. `render_status`
2. `render_error`
3. `rendered_at`

`render_status` 建议支持：

1. `idle`
2. `pending`
3. `success`
4. `failed`

不在本方案中引入复杂的历史版本渲染状态。

---

## 4. 渲染流程

1. 用户在工作区通过 ONLYOFFICE 编辑 Office 文档
2. ONLYOFFICE callback 成功后，后端先保存最新源文件
3. 同时把文档渲染状态置为 `pending`
4. 异步 worker 执行 `Office -> HTML`
5. 转换成功后：
   - 把 HTML 写入 `documents.content_md`
   - `render_status = success`
   - 同步正文图片引用
6. 转换失败后：
   - `render_status = failed`
   - 写入 `render_error`

阅读页只读取文档当前内容，不读取历史渲染结果。

---

## 5. 图片处理

### 5.1 复用 `document_image_assets`

本方案不新增图片引用表。

原因很简单：

1. Office 文档转换后的 HTML 已经落在 `documents.content_md`
2. `document_image_assets` 本身就是“当前正文图片引用表”
3. 现有服务已经支持从 HTML `<img src>` 中提取图片引用

因此：

1. Markdown 文档继续复用现有图片引用同步逻辑
2. Office HTML 也继续走同一套图片引用同步逻辑

### 5.2 复用 `file_blobs` 做图片去重

本方案不新增 `image_objects` 表。

图片物理对象直接复用 `file_blobs`：

1. `file_blobs` 已有：
   - `content_hash_algo`
   - `content_hash`
   - `size_bytes`
2. `file_blobs` 已有 hash 唯一约束，可用于按内容去重

建议补一层关联：

1. 为 `document_image_assets` 增加可空 `blob_id`
2. `blob_id` 关联 `file_blobs(blob_id)`

这样职责就清楚了：

1. `file_blobs` 管物理图片对象和 hash 去重
2. `document_image_assets` 管文档当前正文对图片的引用

### 5.3 图片上传与复用流程

当 Word 中存在图片时，转换流程如下：

1. 从 `docx` 解包出图片二进制
2. 计算 hash
3. 先查 `file_blobs` 是否已有相同文件
4. 若已有：
   - 直接复用已有 `blob_id/object_key/object_url`
5. 若没有：
   - 上传图床
   - 新增 `file_blobs`
6. 最后根据 HTML 中实际引用，更新 `document_image_assets`

这样同一张图片不会在多次转换中重复上传。

### 5.4 图片清理

继续复用现有图片清理能力：

1. 每次新的 HTML 渲染成功后，同步当前正文图片引用
2. 已不再引用的图片进入 `pending_cleanup`
3. 现有后台图片管理和清理任务继续工作

物理删除时，按 `blob_id` 判断该图片是否仍被其它文档引用。

---

## 6. 搜索处理

Office 文档启用本地 HTML 渲染后，可以参与全文搜索。

规则如下：

1. Markdown 文档  
   继续按 Markdown 转纯文本建立索引
2. Office 文档  
   对 `content_md` 中的 HTML：
   - 去掉 HTML 标签
   - 解码实体
   - 压缩空白
   - 再参与索引

不需要新增单独的 Office 搜索正文表。

---

## 7. 后台系统配置

仍然拆成两个配置键：

1. `onlyoffice-integration`
2. `office-rendering`

### 7.1 `onlyoffice-integration`

字段：

1. `enabled`
2. `documentServerUrl`
3. `callbackPublicBaseUrl`
4. `jwtSecret`

### 7.2 `office-rendering`

字段：

1. `independentRenderEnabled`
2. `renderTimeoutSeconds`
3. `maxRetryCount`
4. `fallbackToOnlyOfficeOnRenderFailure`

### 7.3 后台页面要求

系统配置页继续沿用：

1. 左侧菜单
2. 右侧配置项

新增两个菜单项：

1. `ONLYOFFICE 接入`
2. `Office 阅读渲染`

同时把后台导航里的“系统配置”提升到系统治理分组首位。

### 7.4 旧配置迁移

当前 `onlyoffice` 配置迁移到 `onlyoffice-integration`：

1. 优先读取新键
2. 新键不存在时兼容读取旧键
3. 后台页面只写新键

---

## 8. 技术选型

### 8.1 Word

建议使用 `Mammoth`：

1. 不自研 Word 解析器
2. 目标是输出语义化 HTML
3. 更适合阅读页场景

### 8.2 Excel

建议使用 `Excelize` + 薄渲染层：

1. 用 Excelize 读取 workbook / sheet / cell
2. 后端输出 table HTML
3. 前端按 sheet tabs + table 结构展示

### 8.3 渲染运行时

建议：

1. Go 负责任务调度和文档回写
2. `docx` 转换走 Node adapter
3. `xlsx` 转换走 Go adapter

---

## 9. 前端改造

### 9.1 工作区

不改，继续 ONLYOFFICE 编辑。

### 9.2 阅读页

Office 文档阅读页按配置切换：

1. 关闭独立渲染：
   继续 ONLYOFFICE 只读模式
2. 开启独立渲染：
   直接渲染 `content_md` 中的 HTML

### 9.3 Excel 阅读结构

`xlsx` 阅读页建议单独布局：

1. 标题区
2. sheet tabs
3. table 容器
4. 横向滚动区域

---

## 10. 实施顺序

1. 新增 `onlyoffice-integration`、`office-rendering` 配置键，并迁移旧 `onlyoffice`
2. 为 `documents` 增加最小渲染状态字段
3. 为 `document_image_assets` 增加 `blob_id`，关联 `file_blobs`
4. 实现 callback 后异步转换并回写 `documents.content_md`
5. Office HTML 接入当前正文图片同步逻辑
6. 阅读页按配置切换 ONLYOFFICE / 本地 HTML
7. Office HTML 去标签后接入搜索索引

---

## 11. 测试清单

1. 旧 `onlyoffice` 配置迁移后，新旧键兼容读取正常
2. 独立渲染关闭时，阅读页仍走 ONLYOFFICE
3. 转换成功后，Office HTML 正确写入 `documents.content_md`
4. 转换失败后，前端能正确显示失败态
5. HTML 中的图片引用能正确同步到 `document_image_assets`
6. 相同图片多次转换不会重复上传，能复用 `file_blobs`
7. Office HTML 去标签后可参与全文搜索

---

## 12. 当前推荐结论

最终采用：

1. `documents.content_md` 直接承载 Office 当前 HTML
2. `documents.format` 区分 Markdown 和 Office HTML
3. `document_image_assets` 继续承载当前正文图片引用
4. `file_blobs` 继续承载图片物理对象与 hash 去重
5. 不引入独立快照表和历史渲染能力

这就是当前最适合落地和 review 的精简版方案。
