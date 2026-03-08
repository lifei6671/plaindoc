# 文档编辑器导入能力技术方案

**文档状态**: Draft  
**创建日期**: 2026-03-08  
**适用范围**: `apps/server`、`apps/web`、`docs`  
**目标**: 在文档编辑器右侧子菜单增加“导入”能力，支持 `docx`、`xlsx`、`md`、`txt`、`html`、`zip` 导入；在启用 ONLYOFFICE 时优先保留 Office 原生格式，在未启用时统一转换为 Markdown 文档；支持 ZIP 目录结构恢复、部分失败、失败清单回显、文本编码统一转 UTF-8，以及 Office 内嵌图片落地为 Markdown 图片引用。

---

## 1. 方案结论

采用“前端一次性提交导入请求 + 后端在当前请求内完成处理 + 前端在请求结束后展示结果”的实现方式。

核心规则如下：

1. 在文档树右侧操作菜单增加“导入”入口，点击后打开导入模态框
2. 导入目标为当前选中节点所在目录
3. 当后台启用 ONLYOFFICE 时：
   - `docx` 创建为 `docx` 文档
   - `xlsx` 创建为 `xlsx` 文档
4. 当后台未启用 ONLYOFFICE 时：
   - `docx` 通过 `Mammoth -> HTML -> Turndown -> Markdown`
   - `xlsx` 通过 `excelize -> HTML -> Turndown -> Markdown`
5. `md`、`txt` 始终按普通 Markdown 文档导入
6. `html` 在导入时需要解析正文并转换为 Markdown 文档
7. `zip` 先安全解压，再按上述规则递归处理，并保留目录结构
8. ZIP 导入允许部分成功、部分失败，导入结果必须返回失败清单
9. Office 文档中的内嵌图片需要落地为 Markdown 图片引用，不走“附件”语义
10. 文本类文件导入前需要统一转成 UTF-8 编码

本方案默认不做：

1. 导入任务取消
2. 导入任务断点续传
3. 导入任务历史长期存档页面
4. ZIP 内密码文件支持
5. `pptx`、`csv` 等额外格式导入

---

## 2. 目标与范围

### 2.1 目标

本次导入能力需要同时解决四类问题：

1. 提供统一的编辑器导入入口和导入进度反馈
2. 在 ONLYOFFICE 开关不同的情况下，自动选择“保留 Office 原生格式”或“转换为 Markdown”
3. 支持批量导入和 ZIP 目录结构恢复
4. 对导入失败文件提供明确、可回显、可定位的失败清单
5. 对文本类文件做编码识别并统一转成 UTF-8

### 2.2 非目标

本期不追求完全还原 Office 的所有版式能力，尤其是：

1. Word 中极复杂分页、页眉页脚、批注、修订记录
2. Excel 中复杂图表、宏、透视表、公式编辑态
3. 任意 Office 浮动元素的像素级还原

本期目标是：

1. 原生 Office 模式下尽量保留文件原貌
2. Markdown 模式下保证内容可读、结构尽量保真、图片可见

---

## 3. 基于当前项目的实现基础

当前项目已经具备这次导入方案所需的主要基础设施。

### 3.1 Office 原生文档基础

后端已存在 Office 文档创建和源文件落地能力，可作为“ONLYOFFICE 启用时的原生导入”基础：

1. [workspace_office_support.go](./apps/server/internal/server/handler/workspace_office_support.go)
2. [workspace_onlyoffice.go](./apps/server/internal/server/handler/workspace_onlyoffice.go)

这部分能力说明：

1. 当前系统已经区分 `markdown`、`docx`、`xlsx` 三种文档格式
2. 当前系统已经支持为 Office 文档准备 source blob
3. 当前系统已经有 ONLYOFFICE 配置读取与格式判断逻辑

### 3.2 Office 本地解析基础

后端已经存在 Office 阅读页本地渲染的能力，可直接复用于“导入时转 Markdown”的转换链路：

1. [office_html_render_service.go](./apps/server/internal/service/office_html_render_service.go)
2. [render_docx_with_mammoth.mjs](./apps/server/internal/service/scripts/render_docx_with_mammoth.mjs)

现状包括：

1. `docx` 已通过 `Mammoth` 转 HTML
2. `xlsx` 已通过 `excelize` 渲染为 HTML 表格
3. Word 内嵌图片已存在提取与物化基础
4. Excel 图片、图表信息已经有读取基础，可继续复用到导入转换

### 3.3 前端入口基础

文档树节点右侧操作菜单已经存在，适合直接增加“导入”入口：

1. [WorkspaceTree.tsx](./apps/web/src/components/WorkspaceTree.tsx)

导入模态框可以复用当前项目已有的弹层与上传交互模式，不需要从零设计一套 UI 规范。

### 3.4 Node 转换依赖基础

当前仓库根 `package.json` 已包含 `mammoth`，但 `turndown` 仅存在于前端工作区：

1. [package.json](./package.json)
2. [apps/web/package.json](./apps/web/package.json)

因此本方案建议：

1. 将 `turndown` 与 `turndown-plugin-gfm` 提升到仓库根依赖，供服务端 Node 脚本调用
2. 服务端继续沿用“Go 调 Node 脚本”的方式完成 `docx -> html -> markdown`
3. `xlsx -> html -> markdown` 仍由 Go 负责 HTML 生成，再调用 Node 脚本做 Turndown 转换

---

## 4. 核心导入规则

### 4.1 文件类型处理矩阵

| 文件类型 | ONLYOFFICE 开启 | ONLYOFFICE 关闭 |
| --- | --- | --- |
| `docx` | 创建 `docx` 文档，保存原始 source blob | 转 Markdown 文档 |
| `xlsx` | 创建 `xlsx` 文档，保存原始 source blob | 转 Markdown 文档 |
| `md` | 直接创建 Markdown 文档 | 直接创建 Markdown 文档 |
| `txt` | 按纯文本创建 Markdown 文档 | 按纯文本创建 Markdown 文档 |
| `html` | 解析 HTML 后创建 Markdown 文档 | 解析 HTML 后创建 Markdown 文档 |
| `zip` | 解压后递归分流 | 解压后递归分流 |

### 4.2 导入目标规则

1. 当用户在目录节点上点击“导入”时，导入到该目录下
2. 当用户在文档节点上点击“导入”时，导入到该文档的父目录下
3. 当存在重名目录或文档时，采用递增后缀策略，例如 `文档 (1)`

### 4.3 Markdown 文档命名规则

1. `md`、`txt` 导入后文档标题默认取文件名
2. `docx`、`xlsx` 转 Markdown 后文档标题仍取原文件名去扩展名
3. ZIP 中的目录标题按目录名生成

---

## 5. 转换与落地设计

### 5.1 ONLYOFFICE 开启时的原生导入

当系统已启用 ONLYOFFICE 时，`docx/xlsx` 导入不做 Markdown 转换，直接按 Office 文档处理。

处理步骤：

1. 接收上传文件
2. 识别格式为 `docx` 或 `xlsx`
3. 创建对应格式的文档记录
4. 将原始文件上传为 source blob
5. 建立文档与源文件关系
6. 返回创建结果

这条链路优先复用现有 Office 文档创建与 source blob 物化逻辑，不新造一套存储模型。

### 5.2 ONLYOFFICE 关闭时的 Markdown 化导入

当系统未启用 ONLYOFFICE 时，所有非 Markdown 格式都需要转成 Markdown 内容。

#### 5.2.1 Word 导入

处理链路：

1. 读取 `docx`
2. 调用 `Mammoth` 转 HTML
3. 提取并物化内嵌图片
4. 将 HTML 中的 `<img>` 替换为最终受控图片 URL
5. 调用 `Turndown` 将 HTML 转 Markdown
6. 创建普通 Markdown 文档

#### 5.2.2 Excel 导入

处理链路：

1. 使用 `excelize` 打开工作簿
2. 将工作簿渲染为结构化 HTML
3. 提取工作表图片并映射到相应单元格或对应区域
4. 对复杂图表写入明确提示
5. 调用 `Turndown` 将 HTML 转 Markdown
6. 创建普通 Markdown 文档

Excel 转 Markdown 需要承认一个事实：Markdown 原生对复杂表格和浮动元素表达能力较弱。因此本方案要求：

1. 优先输出可读 Markdown
2. 对无法安全降级为 Markdown 的复杂表格片段，允许保留 HTML 片段嵌入在 Markdown 中
3. 对复杂图表输出固定提示，例如“当前工作表存在复杂图表，请下载原文件查看”

#### 5.2.3 Markdown、纯文本与 HTML 导入

1. `md` 直接读取原文
2. `txt` 读取后按纯文本写入 Markdown
3. `html` 读取后先抽取正文，再通过 `Turndown` 转 Markdown
4. 文本类文件在解析前统一转成 UTF-8
5. 文本编码需要优先识别 UTF-8，并兼容 `GB2312/GBK/GB18030` 等中文编码族
6. 必要时处理 BOM

#### 5.2.4 文本编码规范化

文本类导入源包括：

1. 独立上传的 `md`
2. 独立上传的 `txt`
3. ZIP 内的 `md`
4. ZIP 内的 `txt`
5. ZIP 内的 `html`

统一处理规则：

1. 后端读取原始字节后先做编码探测
2. 优先按 UTF-8 解码
3. 若 UTF-8 不成立，则按 `GB18030` 兜底解码，以兼容常见 `GB2312/GBK` 文本
4. 成功解码后统一转成 UTF-8 字符串进入后续 Markdown/HTML 解析链路
5. 若无法判定编码，则将该文件记入失败清单，并标记为“文本编码无法识别”

### 5.3 内嵌图片落地规则

你已经明确要求：图片需要落地，但不是“附件”，而是嵌入到 Markdown 里。

本方案据此约定：

1. Office 内嵌图片统一物化为受控图片资源
2. 最终正文中使用 Markdown 图片语法引用，例如 `![alt](url)`
3. 不创建“文档附件”语义，不出现在附件列表中
4. 图片资源仍可复用现有图片托管、`file_blobs` 去重、`document_image_assets` 引用追踪能力

也就是说：

1. 物理资源层仍然可以走图片托管服务与 blob 去重
2. 业务语义层不走附件，而走正文图片引用

#### 5.3.1 Word 图片处理

1. 通过 `Mammoth` 的 `convertImage` 获取图片二进制
2. 上传图片资源并拿到最终 URL
3. 将 HTML 中的 `<img>` 替换为受控 URL
4. Turndown 自定义规则将其转为 Markdown 图片语法

#### 5.3.2 Excel 图片处理

1. 使用 `excelize` 获取图片锚点与二进制
2. 物化图片资源并生成 URL
3. 在 HTML 渲染阶段将图片落到对应单元格或 sheet 区块
4. Turndown 自定义规则将图片转为 Markdown 图片语法

#### 5.3.3 为什么不使用 data URI

不建议把图片直接转成 base64 data URI 写进 Markdown：

1. 文档体积会急剧膨胀
2. 后续编辑、搜索、导出都不友好
3. 浏览器和编辑器性能会明显变差

因此“嵌入到 Markdown 里”的实现方式应是：

1. 正文中嵌入 Markdown 图片引用
2. 图片文件本身独立存储并受控访问

### 5.4 ZIP 导入设计

ZIP 导入采用“解压 -> 遍历 -> 构建目录树 -> 按文件分流”的方式。

处理步骤：

1. 上传 ZIP 文件
2. 做安全解压扫描
3. 过滤非法条目
4. 先创建目录结构
5. 再逐个处理文件
6. 汇总成功、失败、跳过结果

### 5.5 ZIP 部分失败策略

ZIP 导入允许部分失败，不做整包回滚。

必须满足：

1. 成功文件照常创建
2. 失败文件保留失败记录
3. 单次导入响应里输出失败清单
4. 前端模态框可按文件路径查看失败原因

失败清单建议最少包含：

1. 原始相对路径
2. 文件类型
3. 失败阶段
4. 失败原因
5. 是否可重试

对 ZIP 中的 `html` 文件，失败清单还应能区分：

1. 编码失败
2. HTML 解析失败
3. Markdown 转换失败

---

## 6. 一次性导入请求模型

你已经明确导入是一次性的，因此本方案不引入导入批次表或批次条目表。

导入链路改为：

1. 前端一次性提交导入请求
2. 后端在当前请求内完成解压、转换、创建目录与创建文档
3. 请求结束时直接返回完整导入结果
4. 前端根据响应渲染成功列表与失败清单

这意味着：

1. 不做持久化的导入任务状态表
2. 不做轮询查询接口
3. 不做导入历史页
4. 导入中的中间状态只存在于当前前端页面内存中

### 6.1 单次导入响应结构

单次导入响应建议最少包含：

1. `totalCount`
2. `successCount`
3. `failedCount`
4. `createdNodes`
5. `items`

其中 `items` 每项建议包含：

1. `sourceName`
2. `sourcePath`
3. `detectedType`
4. `status`
5. `stage`
6. `errorMessage`
7. `createdNodeId`
8. `createdDocumentId`

`status` 建议支持：

1. `success`
2. `failed`
3. `skipped`

`stage` 建议支持：

1. `unzipping`
2. `parsing`
3. `converting`
4. `creating_folder`
5. `creating_document`
6. `done`

---

## 7. 后端接口设计

建议首期只提供一个核心接口。

### 7.1 单次导入接口

`POST /api/workspaces/:spaceID/imports`

请求内容：

1. `targetNodeId`
2. 上传文件列表
3. 可选导入选项

返回内容：

1. `totalCount`
2. `successCount`
3. `failedCount`
4. 文件级结果列表
5. 失败清单
6. 已创建节点信息

### 7.2 鉴权要求

导入能力必须复用当前工作区编辑态权限，至少满足：

1. 只有有编辑权限的成员才能导入
2. 导入目标目录必须属于当前工作区
3. 不能通过伪造 `targetNodeId` 跨空间写入
4. ZIP 内文件创建出来的目录和文档也必须继承当前空间约束

---

## 8. 后端服务拆分建议

建议按以下服务边界拆分，避免 Handler 过重。

### 8.1 Import Service

负责：

1. 调度文件分流
2. 汇总结果
3. 维护单次导入上下文

### 8.2 ZIP Expand Service

负责：

1. 安全解压
2. 目录结构解析
3. 路径合法性校验
4. 文件列表产出

### 8.3 Office Import Converter

负责：

1. `docx -> html -> markdown`
2. `xlsx -> html -> markdown`
3. `html -> markdown`
4. 图片物化与 URL 替换
5. Turndown 自定义规则

### 8.4 Text Decode Service

负责：

1. `md/txt/html` 原始字节编码探测
2. UTF-8 规范化
3. `GB2312/GBK/GB18030` 兼容解码
4. BOM 清理

### 8.5 Tree Materialize Service

负责：

1. 创建目录节点
2. 创建文档节点
3. 处理重名
4. 回填单次导入结果

---

## 9. Turndown 转换策略

本方案不是简单调用一次默认 Turndown，而是需要针对 Office 内容补规则。

### 9.1 Word 转换规则

需要重点保留：

1. 标题层级
2. 列表
3. 引用
4. 代码块
5. 表格
6. 图片

图片转换规则：

1. `<img alt="xxx" src="...">` 转 `![xxx](...)`
2. 缺失 alt 时使用空 alt 或文件名兜底

### 9.2 Excel 转换规则

需要重点处理：

1. `table`
2. `thead`
3. `tbody`
4. `th`
5. `td`
6. 图片节点
7. 图表提示块

建议策略：

1. 简单表格转标准 Markdown 表格
2. 合并单元格、复杂列宽等无法稳定降级时保留 HTML 表格片段
3. 图片保持 Markdown 图片语法
4. 图表提示块转普通提示段落

### 9.3 HTML 转换规则

需要重点保留：

1. 标题层级
2. 列表
3. 表格
4. 图片
5. 链接
6. 代码块

建议策略：

1. 导入时优先抽取 `body` 有效内容
2. 清理脚本、样式、表单和无关装饰节点
3. 使用 Turndown 转成 Markdown
4. 对复杂表格和嵌入块允许保留局部 HTML
5. HTML中内嵌的img标签，如果是外部链接，需要下载到本地，目前已经有已经实现的下载器，可以复用。

---

## 10. 前端交互方案

### 10.1 入口位置

在文档树节点右侧操作菜单中新增：

1. `导入`

该入口建议出现在：

1. 目录节点
2. 文档节点

点击后打开导入模态框。

### 10.2 导入模态框

模态框建议分三态：

1. 待选择
2. 导入中
3. 导入完成

待选择态展示：

1. 支持格式说明
2. 拖拽上传区域
3. 多文件选择按钮
4. 当前导入目标位置

导入中展示：

1. 文件列表
2. 每项本地处理中状态
3. 整体处理中提示
4. 失败项即时提示

导入完成展示：

1. 成功数
2. 失败数
3. 失败清单
4. 刷新文档树入口

### 10.3 失败清单展示

失败清单至少展示：

1. 文件名
2. ZIP 内相对路径
3. 失败原因
4. 失败阶段

这样用户能明确知道：

1. 哪个文件失败了
2. 失败发生在哪个步骤
3. 失败后其它文件是否已经成功导入

---

## 11. 安全与限制

### 11.1 ZIP 安全要求

必须拦截以下风险：

1. `../` 路径穿越
2. 绝对路径
3. Windows 盘符路径
4. 符号链接
5. 特殊设备文件

### 11.2 资源限制

建议增加以下限制：

1. 单次导入文件数量上限
2. 单文件大小上限
3. ZIP 解压后总大小上限
4. ZIP 目录层级上限
5. 支持扩展名白名单

### 11.3 内容容错

需要允许但明确提示：

1. Word 局部样式丢失
2. Excel 复杂图表降级为提示文案
3. 个别损坏文件只影响自身，不影响同次导入中的其他文件

---

## 12. 分阶段实施建议

### 12.1 第一阶段

先交付完整主链路：

1. 节点菜单导入入口
2. 导入模态框
3. 单次导入 API
4. `md/txt/html` 导入
5. `docx` Markdown 化导入
6. `xlsx` Markdown 化导入
7. ONLYOFFICE 开启时 `docx/xlsx` 原生导入
8. ZIP 目录恢复
9. 失败清单

### 12.2 第二阶段

再补体验增强：

1. 导入失败重试
2. 更细粒度进度文案
3. 更丰富的导入统计

---

## 13. 任务清单

### 13.1 后端

1. 新增单次导入接口
2. 实现工作区编辑权限与目标节点权限校验
3. 实现文件类型识别和白名单校验
4. 实现文本编码探测与 UTF-8 规范化
5. 实现 ZIP 安全解压与路径清洗
6. 实现目录结构恢复和重名处理
7. 复用现有 Office source blob 链路实现原生 `docx/xlsx` 导入
8. 为服务端引入 `turndown` 依赖与转换脚本
9. 实现 `docx -> Mammoth -> Turndown -> Markdown`
10. 实现 `xlsx -> excelize HTML -> Turndown -> Markdown`
11. 实现 `html -> Turndown -> Markdown`
12. 实现 Word/Excel 图片物化并替换为 Markdown 图片引用
13. 实现复杂图表提示降级
14. 实现单次导入结果汇总与失败清单
15. 补齐后端单测与路由测试

### 13.2 前端

1. 在文档树右侧菜单增加“导入”入口
2. 实现导入模态框与多文件选择
3. 实现单次导入请求
4. 实现请求中的本地处理中状态
5. 实现文件级结果列表
6. 实现失败清单展示
7. 导入完成后刷新文档树并按需定位新文档
8. 补齐 `WorkspaceTree` 相关组件测试

### 13.3 联调与验收

1. 验证 ONLYOFFICE 开关分流是否符合预期
2. 验证 `docx/xlsx/md/txt/html/zip` 各类型导入
3. 验证 UTF-8 与 `GB2312/GBK/GB18030` 文本解码
4. 验证 ZIP 混合成功失败场景
5. 验证目录结构恢复
6. 验证图片是否正确落成 Markdown 图片引用
7. 验证复杂图表提示是否可见
8. 验证权限拦截与跨空间安全性

---

## 14. 风险与注意事项

1. `xlsx -> markdown` 是本方案里不确定性最高的一段，因为 Markdown 对复杂表格天然表达能力有限，需要通过“Markdown + 局部 HTML 保留”兜底。
2. 服务端当前没有现成的 `turndown` 依赖，需要将其提升到可供 Node 脚本使用的位置。
3. 文本导入不能假设都是 UTF-8，需要显式处理 `GB2312/GBK/GB18030` 兼容解码。
4. Office 图片虽然不走附件语义，但仍应复用现有图片托管与引用追踪，否则后续清理和鉴权会失控。
5. ZIP 导入不回滚意味着需要把失败清单做完整，否则用户会误判导入结果。
6. 一次性导入请求如果处理时间过长，可能受到反向代理或网关超时限制，因此需要通过文件数量和解压总量做硬限制。
7. ONLYOFFICE 配置虽然开启，但若服务不可用，需要明确错误提示，不应静默降级成 Markdown 导入。

---

## 15. 推荐实施顺序

1. 先实现单次导入接口和前端模态框骨架
2. 先接 `md/txt/html` 与文本编码规范化、ZIP 目录恢复
3. 再接 `docx` 转 Markdown
4. 再接 `xlsx` 转 Markdown
5. 最后接 ONLYOFFICE 原生 Office 导入与整体联调

这样做的好处是：

1. 最早能跑通一条端到端导入链路
2. 最复杂的 `xlsx` 转换放在后面收敛
3. 前端 UI 可以尽早联调，不必等所有格式完成
